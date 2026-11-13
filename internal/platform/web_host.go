package platform

import (
	"errors"

	"github.com/vivaui/viva/gfx"
)

// A window can host native views: pages of the system's web engine that live inside the window
// among Viva's own drawing. The view is a platform object; Viva keeps a hole in what it draws where
// the view is (a NativeView in Frame.NativeViews), and everything on a layer above the view's
// is drawn over it, so a menu opened by Viva stands over a page.

// ErrWebViewUnsupported is returned by WebHost.Open when the platform cannot host web pages in the
// window, or the system's web engine is not installed.
var ErrWebViewUnsupported = errors.New("platform: this window cannot host web pages")

// WebProxyDirect explicitly bypasses system proxies for one page's network context.
const WebProxyDirect = "direct"

// NativeView is a place in the window that a native view fills.
type NativeView struct {
	// ID is the WebView it belongs to (WebViewConfig.ID).
	ID uint32
	// Bounds is the place in logical window pixels.
	Bounds gfx.Rect
	// Layer is the compositing layer the view sits in: what Viva draws on a higher layer shows
	// over the view, what it draws on this layer or a lower one is hidden by it.
	Layer uint16
	// Covers are the parts of Bounds, in logical window pixels, where something Viva does not draw
	// but handles (the invisible backdrop of a menu that closes on an outside press) lies over the
	// view on a higher layer: the pointer belongs to Viva there. What Viva draws over the view
	// there is already known to the adapter from the rectangles.
	Covers []gfx.Rect
}

// WebHost is how an application opens web pages in a window. The adapter assigns Open before the
// window is shown; it and everything it returns run on the window's event-loop thread.
type WebHost struct {
	Open func(WebViewConfig) (WebView, error)
	// Ready, when set by the application, is called by the adapter on the window's thread once
	// the window exists and Open can be used.
	Ready func()
}

// WebViewConfig describes a page to open.
type WebViewConfig struct {
	// ID names the view in Frame.NativeViews; it is chosen by the opener and unique in the window.
	ID uint32
	// URL is loaded when the view opens, if HTML is empty.
	URL string
	// HTML is loaded instead of a URL, with BaseURL as its address.
	HTML    string
	BaseURL string
	// Scheme, when set, answers requests to the custom scheme Scheme.Name (the page loads
	// "name://host/path") from the application's own resources, without a server.
	Scheme *WebScheme
	// Proxy, when set, is the loopback host:port of a SOCKS5 proxy that carries all of the page's
	// traffic, names resolved by the proxy. A platform that cannot hold the page to it returns
	// ErrWebViewUnsupported rather than opening a page that connects directly.
	// WebProxyDirect bypasses system proxies instead.
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
	// Ephemeral keeps the page's cookies, storage and caches in memory only, separate from every
	// other page, and drops them when the page closes: a run of the application leaves nothing the
	// page kept on disk. A platform without such a store must refuse the page rather than open one
	// that persists.
	Ephemeral bool
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

// WebLoad is one step of the page's navigation.
type WebLoad struct {
	Phase WebLoadPhase
	URL   string
}

// WebView is one open page.
type WebView interface {
	Load(url string)
	LoadHTML(html, baseURL string)
	Reload()
	Back()
	Forward()
	// Eval runs script in the page; the result is not read back (post a message instead).
	Eval(script string)
	// PostMessage delivers text to the page's window.viva.onmessage handler.
	PostMessage(text string)
	// Focus gives the page the keyboard.
	Focus()
	Close()
}
