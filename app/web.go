package app

import (
	"errors"
	"fmt"
	"math"
	"net/netip"
	"strings"
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

// WebProxyDirect selects direct networking for a page, bypassing system proxies.
// macOS requires version 14 or newer; unsupported engines return ErrUnsupported.
// On macOS and Linux, direct pages use a nonpersistent data store.
const WebProxyDirect = platform.WebProxyDirect

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
	// Empty follows the system proxy policy. WebProxyDirect bypasses system proxies for this page.
	Proxy string
	// OnMessage receives what the page posts with window.viva.postMessage(text).
	OnMessage func(text string)
	// TopCornerRadius rounds the top two corners in logical pixels; zero keeps a rectangular view.
	// macOS and Windows clip natively; GTK returns ErrUnsupported for nonzero radii.
	TopCornerRadius float32
	// OnLoad reports the page's navigation.
	OnLoad func(WebLoad)
	// OnProcessFailed reports the page's process gone: it crashed, the system or the engine stopped
	// it, or it never started. That is not a navigation that failed: the page was there, and its
	// process is gone, so what the user typed into it may be lost and it cannot draw again until it
	// is loaded again, which WebView.Reload does. The WebProcessFailure says which of those it was,
	// because a crash is worth warning about before a reload and a page the system reclaimed under
	// memory pressure is not. WebView.IsCrashed answers the same question afterwards, and a
	// navigation that starts again clears it.
	OnProcessFailed func(WebProcessFailure)
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
	// that persists. On Windows it needs WebView2 Runtime 101 (SDK 1.0.1210.39) or newer; on Linux,
	// WebKitGTK 2.16 or newer.
	Ephemeral bool
	// Partition names the store the page keeps its cookies, localStorage, IndexedDB and caches in:
	// pages that name the same partition share them, pages that name different partitions do not,
	// and neither shares with the pages that name none. Empty keeps the application's default store,
	// which every page without a partition shares, and with no partition an Ephemeral page keeps its
	// own store in memory.
	// A named partition is persistent: it is still there after the application exits. With Ephemeral
	// it is memory-only instead, shared by the pages of that name and gone when the application
	// exits; macOS has no way to name a non-persistent store and refuses that page with
	// ErrUnsupported. A name is at most 64 ASCII letters, digits or # @ $ ( ) + - _ ~ . or space, does
	// not end in a period or a space, and is matched without case (both rules are WebView2's, kept
	// everywhere so a name means the same store on every engine). On Windows it needs WebView2
	// Runtime 101 (SDK 1.0.1210.39) or newer; on macOS, 14 or newer; on Linux, WebKitGTK 2.10 or
	// newer. A named partition cannot be combined with Proxy on macOS or Linux, where the proxy
	// belongs to the store the partition names: Open fails with ErrUnsupported rather than let two
	// pages of one partition disagree about it. On Linux a name already open as the other kind of
	// store is refused as well, since one WebKit context per name can only be one of the two.
	Partition string
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

// WebLoadError is why a navigation did not load the page, in the terms every engine can be held to:
// the engines name the same failures differently, so a failed load reads the same way on every
// platform. WebLoadNoError is a navigation that loaded.
type WebLoadError = platform.WebLoadError

const (
	// WebLoadNoError is a navigation that loaded.
	WebLoadNoError = platform.WebLoadNoError
	// WebLoadUnknownError is a navigation the engine reports as failed without saying why, or with
	// a reason Viva does not classify; WebLoad.Description carries whatever the engine said.
	WebLoadUnknownError = platform.WebLoadUnknownError
	// WebLoadCancelled is a navigation that was stopped: by the engine, or by a policy that
	// interrupted the load it was part of.
	WebLoadCancelled = platform.WebLoadCancelled
	// WebLoadTimeout is a server that did not answer in time.
	WebLoadTimeout = platform.WebLoadTimeout
	// WebLoadHostNotFound is a name that did not resolve.
	WebLoadHostNotFound = platform.WebLoadHostNotFound
	// WebLoadServerUnreachable is a host that could not be reached: no route, or a connection that
	// was refused.
	WebLoadServerUnreachable = platform.WebLoadServerUnreachable
	// WebLoadConnectionLost is a connection that broke while the page was arriving, or a machine
	// with no network at all.
	WebLoadConnectionLost = platform.WebLoadConnectionLost
	// WebLoadBadServerResponse is an answer the engine could not read as HTTP, or one that broke its
	// own framing.
	WebLoadBadServerResponse = platform.WebLoadBadServerResponse
	// WebLoadBadCertificate is a TLS certificate the engine refused, or a client certificate the
	// server refused.
	WebLoadBadCertificate = platform.WebLoadBadCertificate
	// WebLoadRedirectFailed is a redirect the engine could not follow.
	WebLoadRedirectFailed = platform.WebLoadRedirectFailed
	// WebLoadAuthenticationRequired is a server or proxy that stopped the navigation for credentials
	// the engine did not have.
	WebLoadAuthenticationRequired = platform.WebLoadAuthenticationRequired
)

// WebLoad is one step of a page's navigation.
type WebLoad struct {
	Phase WebLoadPhase
	URL   string
	// Title is the title of the document the engine holds at this step: empty before a page has
	// one, and still the title of the document being left at WebLoadStarted.
	Title string
	// Code is why the navigation failed, when Phase is WebLoadFinished and the page did not load;
	// WebLoadNoError when it did, and at every earlier step.
	// Description is the engine's own message for it, when the engine gives one: WebView2 names a
	// status and no text, so it is empty on Windows. An application that shows a failed load reads
	// Description when it is not empty and Code otherwise, which names the failure as well.
	Code        WebLoadError
	Description string
}

// WebProcessFailure is why a page's process is gone, in the terms every engine can be held to: the
// engines name the same failures differently, so a page whose process was lost reads the same way
// on every platform. WebProcessNoFailure is a page whose process is there.
//
// This is not a navigation that failed: a page that did not load has its process, and reports
// through WebLoad. A page that lost its process was there, and cannot draw again until it is
// loaded again.
type WebProcessFailure = platform.WebProcessFailure

const (
	// WebProcessNoFailure is a page whose process is there.
	WebProcessNoFailure = platform.WebProcessNoFailure
	// WebProcessCrashed is a process that stopped without being asked to: a fault in the page or
	// in the engine, which may have lost what the user typed into it that was not saved.
	WebProcessCrashed = platform.WebProcessCrashed
	// WebProcessKilled is a process the system or the engine stopped: memory pressure, an
	// out-of-memory kill, or the engine ending it itself. The page is gone with it, but nothing
	// the page did made it happen.
	WebProcessKilled = platform.WebProcessKilled
	// WebProcessUnresponsive is a process that stopped answering, which the engine ended.
	WebProcessUnresponsive = platform.WebProcessUnresponsive
	// WebProcessLaunchFailed is a process that never started, so the page never ran at all.
	WebProcessLaunchFailed = platform.WebProcessLaunchFailed
	// WebProcessUnknown is a process the engine reports as gone without a reason Viva classifies,
	// including macOS, whose delegate names no reason at all.
	WebProcessUnknown = platform.WebProcessUnknown
)

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

// webPartitionNameLength is the longest name a WebView2 profile accepts, which is the longest name a
// partition may carry: a name is refused here rather than on one engine only, so that a name an
// application uses means the same store wherever it runs.
const webPartitionNameLength = 64

// webPartitionRune reports whether an engine accepts r in the name of a store. The set is WebView2's
// profile name set; WebKit takes any name, but a name is portable or it is not worth having.
func webPartitionRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("#@$()+-_~. ", r)
}

// partitionName checks a storage partition's name and returns the name the platform is given. Case
// is folded: Windows maps a WebView2 profile name to a folder that ignores case, so two partitions
// whose names differ only in case are one store there, and are one store everywhere rather than a
// store on one engine and two on another.
func partitionName(name string) (string, error) {
	if name == "" {
		return "", nil
	}
	if len(name) > webPartitionNameLength {
		return "", fmt.Errorf("app: web view partition name %q is longer than %d characters", name, webPartitionNameLength)
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", fmt.Errorf("app: web view partition name %q must not end with a period or a space", name)
	}
	for _, r := range name {
		if !webPartitionRune(r) {
			return "", fmt.Errorf("app: web view partition name %q carries %q, which is not a letter, a digit, or one of # @ $ ( ) + - _ ~ . space", name, r)
		}
	}
	return strings.ToLower(name), nil
}

// Open opens a page. It fails with ErrUnsupported when the platform cannot host pages or the system
// has no web engine installed.
func (host *WebHost) Open(config WebViewConfig) (*WebView, error) {
	if config.TopCornerRadius < 0 || math.IsNaN(float64(config.TopCornerRadius)) || math.IsInf(float64(config.TopCornerRadius), 0) {
		return nil, fmt.Errorf("app: invalid web view top corner radius")
	}
	partition, err := partitionName(config.Partition)
	if err != nil {
		return nil, err
	}

	if host == nil || host.native == nil || host.native.Open == nil {
		return nil, fmt.Errorf("%w: web views", ErrUnsupported)
	}
	if config.Proxy != "" && config.Proxy != WebProxyDirect && !loopbackAddress(config.Proxy) {
		return nil, fmt.Errorf("app: web view proxy %q is not a loopback host:port", config.Proxy)
	}
	id := gfx.NativeViewID(host.next.Add(1))
	platformConfig := platform.WebViewConfig{
		ID: uint32(id), URL: config.URL, HTML: config.HTML, BaseURL: config.BaseURL, Proxy: config.Proxy,
		TopCornerRadius: config.TopCornerRadius,
		Ephemeral:       config.Ephemeral,
		Partition:       partition,
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
			config.OnLoad(WebLoad{
				Phase: WebLoadPhase(load.Phase), URL: load.URL, Title: load.Title,
				Code: WebLoadError(load.Code), Description: load.Description,
			})
			host.requestPaint()
		}
	}
	if config.Navigate != nil {
		platformConfig.Navigate = func(navigation platform.WebNavigation) bool {
			return config.Navigate(WebNavigation(navigation))
		}
	}
	if config.OnProcessFailed != nil {
		platformConfig.OnProcessFailed = func(failure platform.WebProcessFailure) {
			config.OnProcessFailed(WebProcessFailure(failure))
			host.requestPaint()
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

// Title is the title of the page's current document: empty before the engine has one, and after the
// page closes. It is what the engine reports when it is asked, which can change after WebLoad
// reports WebLoadFinished, so a toolbar reads it on the paint it draws for.
func (view *WebView) Title() string { return view.native.Title() }

// CanGoBack and CanGoForward report whether the page's history has an entry each way, which is what
// a Back or a Forward button is disabled by.
func (view *WebView) CanGoBack() bool    { return view.native.CanGoBack() }
func (view *WebView) CanGoForward() bool { return view.native.CanGoForward() }

// IsLoading reports whether the page is loading a navigation: true from the step the engine starts
// one until it finishes or fails, which is when WebLoad reports WebLoadStarted and WebLoadFinished.
func (view *WebView) IsLoading() bool { return view.native.IsLoading() }

// ProcessFailure is why the page's process is gone, WebProcessNoFailure while it is there: the value
// OnProcessFailed was given, until the page starts a navigation again, which is what clears it. The
// engines have no way to ask, so it is kept from what they reported.
func (view *WebView) ProcessFailure() WebProcessFailure { return view.native.ProcessFailure() }

// IsCrashed reports whether the page's process is gone: it crashed, the system or the engine stopped
// it, or it never started. Such a page cannot draw and its input is lost, so an application that
// hosts one shows its own notice and Reload loads it again rather than leaving the engine's blank
// view. It says the page is gone, not that the reason was a crash: ProcessFailure names the reason,
// which is what a notice that tells the user whether their input is lost needs.
func (view *WebView) IsCrashed() bool { return view.native.ProcessFailure() != WebProcessNoFailure }

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
