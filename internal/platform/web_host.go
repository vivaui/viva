package platform

import (
	"errors"

	"github.com/vivaui/viva/ui"
)

// A window can host native views: pages of the system's web engine that live inside the window
// among Viva's own drawing. The view is a platform object; Viva keeps a hole in what it draws where
// the view is (a NativeView in Frame.NativeViews), and everything on a layer above the view's
// is drawn over it, so a menu opened by Viva stands over a page.

// ErrWebViewUnsupported is returned by WebHost.Open when the platform cannot host web pages in the
// window, or the system's web engine is not installed.
var ErrWebViewUnsupported = errors.New("platform: this window cannot host web pages")

// NativeView is a place in the window that a native view fills.
type NativeView struct {
	// ID is the WebView it belongs to (WebViewConfig.ID).
	ID uint32
	// Bounds is the place in logical window pixels.
	Bounds ui.Rect
	// Layer is the compositing layer the view sits in: what Viva draws on a higher layer shows
	// over the view, what it draws on this layer or a lower one is hidden by it.
	Layer uint16
	// Covers are the parts of Bounds, in logical window pixels, where something Viva does not draw
	// but handles (the invisible backdrop of a menu that closes on an outside press) lies over the
	// view on a higher layer: the pointer belongs to Viva there. What Viva draws over the view
	// there is already known to the adapter from the rectangles.
	Covers []ui.Rect
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
