package app

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"sync/atomic"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
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
//	app.Run(app.Config{Web: web, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
//		if page != nil {
//			canvas.NativeView(page.ID(), gfx.Rect{Y: 48, Width: size.Width, Height: size.Height - 48})
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
	// Proxy, when set, is the address ("127.0.0.1:1080") of a SOCKS5 proxy on this machine that
	// carries all of the page's traffic, names resolved by the proxy: the page reaches only what the
	// proxy reaches. Opening fails with ErrUnsupported where the engine cannot be held to it, rather
	// than opening a page that connects directly.
	Proxy string
	// OnMessage receives what the page posts with window.viva.postMessage(text).
	OnMessage func(text string)
	// TopCornerRadius rounds the top two corners in logical pixels; zero keeps a rectangular view.
	// macOS and Windows clip natively; GTK returns ErrUnsupported for nonzero radii.
	TopCornerRadius float32
	// OnLoad reports the page's navigation.
	OnLoad func(WebLoad)
	// Navigate decides a navigation before it starts; nil allows every navigation.
	Navigate func(WebNavigation) bool
	// NewWindow decides a window.open or target=_blank before it opens; nil refuses it. That is a
	// change: without this option the engine opened the window itself. True opens the address in
	// the system's browser, outside the page.
	NewWindow func(url string) bool
	// Download decides a download before it starts; nil refuses it. That is a change: without this
	// option Windows and Linux let the engine save the file. True saves it in the user's downloads
	// folder.
	Download func(url string) bool
	// BridgeMainFrameOnly injects window.viva into the main frame only, so an embedded frame does
	// not get the bridge. False keeps it in every frame, as today.
	BridgeMainFrameOnly bool
}

// WebNavigation is one navigation the page wants to make.
type WebNavigation struct {
	// URL is the address being navigated to.
	URL string
	// MainFrame is false for a subframe, an iframe.
	MainFrame bool
	// UserGesture is true when a click or a key press started it, not a script or a redirect.
	UserGesture bool
}

// WebScheme serves a custom URL scheme. Serve is the simple form, Respond adds a status and headers,
// and ServeAsync answers later from another goroutine. The first of ServeAsync, Respond and Serve that
// is set is the one used.
type WebScheme struct {
	Name string
	// Serve answers one request: the path after the host, without a query. ok false is a 404.
	Serve func(path string) (mime string, data []byte, ok bool)
	// Respond, when set, answers with a status, headers and a body instead of Serve. A Status it sets
	// wins, even when ok is false; otherwise ok false is a 404, which carries whatever body and MIME
	// the answer names.
	Respond func(request WebRequest) (WebResponse, bool)
	// ServeAsync, when set, replaces both: it is called on the window's thread and answers once,
	// later, from any goroutine. respond may be called from any goroutine, at most once; a second
	// call, or one after the page closed, is ignored.
	ServeAsync func(request WebRequest, respond func(WebResponse))
}

// WebRequest is one request for a resource of a custom scheme.
type WebRequest struct {
	// Path is the path after the host, without a query, as Serve receives it.
	Path string
}

// WebResponse is a custom scheme's answer to a request. A 204 or 304 carries no body, whatever Data
// says.
type WebResponse struct {
	// Status is the HTTP status; 0 means 200, or 404 when Serve or Respond did not find the resource.
	Status int
	// MIME is the Content-Type; empty means "application/octet-stream", or "text/plain" for a 404.
	MIME string
	// Headers are merged over the platform's defaults (Content-Type, Content-Length,
	// Access-Control-Allow-Origin: *); a header with an empty value removes the default. Names are
	// case-insensitive.
	Headers map[string]string
	// Data is the body.
	Data []byte
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
	id     gfx.NativeViewID
	native platform.WebView
	host   *WebHost
}

// loopbackAddress reports whether address is an IP of this machine and a port.
func loopbackAddress(address string) bool {
	parsed, err := netip.ParseAddrPort(address)
	return err == nil && parsed.Addr().IsLoopback() && parsed.Port() != 0
}

// Open opens a page. It fails with ErrUnsupported when the platform cannot host pages or the system
// has no web engine installed.
func (host *WebHost) Open(config WebViewConfig) (*WebView, error) {
	if config.TopCornerRadius < 0 || math.IsNaN(float64(config.TopCornerRadius)) || math.IsInf(float64(config.TopCornerRadius), 0) {
		return nil, fmt.Errorf("app: invalid web view top corner radius")
	}

	if host == nil || host.native == nil || host.native.Open == nil {
		return nil, fmt.Errorf("%w: web views", ErrUnsupported)
	}
	if config.Proxy != "" && !loopbackAddress(config.Proxy) {
		return nil, fmt.Errorf("app: web view proxy %q is not a loopback host:port", config.Proxy)
	}
	id := gfx.NativeViewID(host.next.Add(1))
	platformConfig := platform.WebViewConfig{
		ID: uint32(id), URL: config.URL, HTML: config.HTML, BaseURL: config.BaseURL, Proxy: config.Proxy,
		TopCornerRadius: config.TopCornerRadius,
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
		// The page keeps a copy: an application may reuse or change its WebScheme value after Open,
		// and the callbacks the page holds are still the ones it passed.
		scheme := *config.Scheme
		native := &platform.WebScheme{Name: scheme.Name, Serve: scheme.Serve}
		if scheme.Respond != nil {
			native.Respond = func(request platform.WebRequest) (platform.WebResponse, bool) {
				response, ok := scheme.Respond(WebRequest{Path: request.Path})
				return platform.WebResponse{Status: response.Status, MIME: response.MIME, Headers: response.Headers, Data: response.Data}, ok
			}
		}
		if scheme.ServeAsync != nil {
			native.ServeAsync = func(request platform.WebRequest, respond func(platform.WebResponse)) {
				scheme.ServeAsync(WebRequest{Path: request.Path}, func(response WebResponse) {
					respond(platform.WebResponse{Status: response.Status, MIME: response.MIME, Headers: response.Headers, Data: response.Data})
				})
			}
		}
		platformConfig.Scheme = native
	}
	if config.OnLoad != nil {
		platformConfig.OnLoad = func(load platform.WebLoad) {
			config.OnLoad(WebLoad{Phase: WebLoadPhase(load.Phase), URL: load.URL})
			host.requestPaint()
		}
	}
	if config.Navigate != nil {
		platformConfig.Navigate = func(navigation platform.WebNavigation) bool {
			return config.Navigate(WebNavigation(navigation))
		}
	}
	platformConfig.NewWindow = config.NewWindow
	platformConfig.Download = config.Download
	platformConfig.BridgeMainFrameOnly = config.BridgeMainFrameOnly
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
func (view *WebView) ID() gfx.NativeViewID { return view.id }

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
