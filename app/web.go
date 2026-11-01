package app

import (
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// A window can host pages of the system's web engine among Viva's own drawing: WebKitGTK on Linux,
// WKWebView on macOS, WebView2 on Windows. The engine is the system's, loaded when the first page
// opens; an application that opens none never loads it.
//
// Hosting is two steps. The application opens a page, and paints a hole where it goes:
//
//	web := &app.WebHost{}
//	var page *app.WebView
//	web.OnReady = func() {
//		page, _ = web.Open(app.WebViewConfig{URL: "https://example.com/"})
//	}
//	app.Run(app.Config{Web: web, Paint: func(canvas *ui.Canvas, size ui.Size) {
//		if page != nil {
//			canvas.NativeView(page.ID(), ui.Rect{Y: 48, Width: size.Width, Height: size.Height - 48})
//		}
//		// ...toolbar, and popovers on a higher layer, which stand over the page...
//	}})
//
// Everything here runs on the window's thread: OnReady, the callbacks of a page, and the methods of
// WebHost and WebView. Work from another goroutine reaches it through an Invalidator and OnWake.
//
// A page is a separate, untrusted program: it sees only what the application hands it through the
// window.viva bridge (postMessage and onmessage) and the resources of its custom scheme. Keys go to
// the page while it has the keyboard, and to Viva after a press in Viva's own content.

// WebHost opens pages in one window. The zero value is ready to put in Config.Web.
type WebHost struct {
	// OnReady runs once on the window's thread when the window exists and Open can be used.
	OnReady func()

	native *platform.WebHost
	next   atomic.Uint32
	// repaint asks for a paint: a page that opened or closed is declared (or not) by the next one.
	repaint func()
}

// bind makes the platform's side of the host for a window, or nil when there is no host.
func (host *WebHost) bind() *platform.WebHost {
	if host == nil {
		return nil
	}
	host.native = &platform.WebHost{Ready: func() {
		if host.OnReady != nil {
			host.OnReady()
		}
	}}
	return host.native
}

func (host *WebHost) requestPaint() {
	if host.repaint != nil {
		host.repaint()
	}
}

// repaintWith lets the host ask the painter for a frame.
func (host *WebHost) repaintWith(painter *painter) {
	if host != nil {
		host.repaint = func() {
			if painter.scheduleFrame != nil {
				painter.scheduleFrame(0)
			}
		}
	}
}

// WebViewConfig describes a page to open.
type WebViewConfig struct {
	// URL is loaded when the page opens, unless HTML is set.
	URL string
	// HTML is loaded instead of a URL, with BaseURL as its address.
	HTML    string
	BaseURL string
	// Scheme, when set, serves the custom scheme Scheme.Name (a page loads "name://host/path")
	// from the application's own resources, without a server.
	Scheme *WebScheme
	// OnMessage receives what the page posts with window.viva.postMessage(text).
	OnMessage func(text string)
	// OnLoad reports the page's navigation.
	OnLoad func(WebLoad)
}

// WebScheme serves a custom URL scheme.
type WebScheme struct {
	Name string
	// Serve answers one request: the path after the host, without a query. ok false is a 404.
	Serve func(path string) (mime string, data []byte, ok bool)
}

// WebLoadPhase is where a navigation is.
type WebLoadPhase uint8

const (
	WebLoadStarted WebLoadPhase = iota
	WebLoadCommitted
	WebLoadFinished
)

// WebLoad is one step of a page's navigation.
type WebLoad struct {
	Phase WebLoadPhase
	URL   string
}

// WebView is one open page.
type WebView struct {
	id     ui.NativeViewID
	native platform.WebView
	host   *WebHost
}

// Open opens a page. It fails with ErrUnsupported when the platform cannot host pages or the system
// has no web engine installed.
func (host *WebHost) Open(config WebViewConfig) (*WebView, error) {
	if host == nil || host.native == nil || host.native.Open == nil {
		return nil, fmt.Errorf("%w: web views", ErrUnsupported)
	}
	id := ui.NativeViewID(host.next.Add(1))
	platformConfig := platform.WebViewConfig{
		ID: uint32(id), URL: config.URL, HTML: config.HTML, BaseURL: config.BaseURL,
	}
	// What a page tells the application usually changes what it shows, so a callback is followed
	// by a paint.
	if config.OnMessage != nil {
		platformConfig.OnMessage = func(text string) {
			config.OnMessage(text)
			host.requestPaint()
		}
	}
	if config.Scheme != nil {
		platformConfig.Scheme = &platform.WebScheme{Name: config.Scheme.Name, Serve: config.Scheme.Serve}
	}
	if config.OnLoad != nil {
		platformConfig.OnLoad = func(load platform.WebLoad) {
			config.OnLoad(WebLoad{Phase: WebLoadPhase(load.Phase), URL: load.URL})
			host.requestPaint()
		}
	}
	native, err := host.native.Open(platformConfig)
	if err != nil {
		if errors.Is(err, platform.ErrWebViewUnsupported) {
			return nil, fmt.Errorf("%w: %w", ErrUnsupported, err)
		}
		return nil, err
	}
	host.requestPaint()
	return &WebView{id: id, native: native, host: host}, nil
}

// ID names the page for Canvas.NativeView.
func (view *WebView) ID() ui.NativeViewID { return view.id }

// Load navigates to a URL.
func (view *WebView) Load(url string) { view.native.Load(url) }

// LoadHTML shows a document, with baseURL as its address.
func (view *WebView) LoadHTML(html, baseURL string) { view.native.LoadHTML(html, baseURL) }

// Reload loads the page again.
func (view *WebView) Reload() { view.native.Reload() }

// Back and Forward move through the page's history.
func (view *WebView) Back()    { view.native.Back() }
func (view *WebView) Forward() { view.native.Forward() }

// Eval runs a script in the page. The result is not read back: have the script post a message.
func (view *WebView) Eval(script string) { view.native.Eval(script) }

// PostMessage delivers text to the page's window.viva.onmessage handler.
func (view *WebView) PostMessage(text string) { view.native.PostMessage(text) }

// Focus gives the page the keyboard.
func (view *WebView) Focus() { view.native.Focus() }

// Close closes the page. Its NativeView stops being placed.
func (view *WebView) Close() {
	view.native.Close()
	view.host.requestPaint()
}
