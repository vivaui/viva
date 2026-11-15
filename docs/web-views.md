# Web views

A window can host pages of the system's own web engine among Viva's drawing: WebKitGTK on Linux,
WKWebView on macOS, WebView2 on Windows. It is for the part of an application that already is
a web page (a plugin center, an account page, documentation), not for Viva's own interface, which
stays native.

Nothing is bundled and nothing is loaded until the first page opens: an application that opens no page
does not link, load or start the engine, and pays nothing in size, memory or idle work.

## Using it

```go
web := &app.WebHost{}
var page *app.WebView
web.OnReady = func() {
	var err error
	page, err = web.Open(app.WebViewConfig{URL: "https://example.com/"})
	if err != nil { /* app.ErrUnsupported: no engine installed, or a platform without pages */ }
}
app.RunCanvas(app.Config{Web: web, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
	if page != nil {
		canvas.NativeView(page.ID(), gfx.Rect{Y: 48, Width: size.Width, Height: size.Height - 48})
	}
}}, &canvas)
```

`examples/guides/webview` is a complete program: a toolbar, a page served from memory under its own
scheme, a message in each direction and a menu that stands over the page.

- `WebViewConfig` takes a `URL`, or `HTML`; a `Scheme` serves `name://host/path` from the application's own
  resources without a server (`Serve` gets the path after the host). `Respond` answers with a `WebResponse`
  instead: a status (0 is 200), a MIME type, headers and a body. A status the answer sets wins, even when
  `ok` is false; otherwise `ok` false is a 404. A 404 carries whatever body and MIME the answer names, and an
  empty MIME is `text/plain` for a 404. `ServeAsync` replaces both: it is called on the window's thread, does
  its work wherever it likes, and answers once through `respond`, from any goroutine; a second call, or one
  after the page closed, is ignored. `Headers` are merged over `Content-Type`, `Content-Length` and
  `Access-Control-Allow-Origin: *`: a name is case-insensitive, and a header with an empty value removes the
  default. A 204 or 304 carries no body. On Windows the scheme is served as
  `https://name.viva-app.invalid/host/path` (WebView2 only takes a custom scheme registered before any page
  exists), so write relative URLs in the pages; `OnLoad` reports addresses with the scheme as you wrote it.
  `LoadHTML`'s base URL is ignored on Windows.
- On Linux, the status and headers of a scheme's answer need WebKitGTK's `WebKitURISchemeResponse`, which
  WebKitGTK added in 2.36: 2.36 and newer honour them. An older engine that lacks it finishes with
  `webkit_uri_scheme_request_finish`, which drops them: the body is served as 200, and a 404 becomes the
  engine's failed load. macOS (`NSHTTPURLResponse`) and Windows (`CreateWebResourceResponse`) answer with
  the full response. The feature was tested on macOS in a real WKWebView page; Linux (WebKitGTK) and Windows
  (WebView2) were type-checked only, because no engine of either was available.
- The page and the application talk through `window.viva.postMessage(text)` / `window.viva.onmessage` and
  `OnMessage` / `WebView.PostMessage`. A page sees nothing else of the application.
- `Navigate` decides a navigation before it starts, the application's own `Load` included: the
  `WebNavigation` names the address, whether the target is the main frame (as far as the engine can say),
  and whether the engine says a click or a key press started it rather than a script or a redirect (again
  as far as the engine can say; each engine's limits are below). `nil` allows every navigation; `false`
  cancels it before the request leaves the engine, so no byte of the foreign document is fetched and none
  of its script runs.
- `NewWindow` decides a `window.open` or `target=_blank` before it opens. `nil` refuses it, because a page
  must not open windows unless the application allows it, and `false` opens nothing. That refusal is a
  change on Windows and Linux, where without this option the engine opened the window itself; macOS refused
  it already. `true` hands the address to the system's browser (the platform's `OpenURL`, the same one
  `app.OpenURL` uses) and never opens a page in the window.
- `Download` decides a download before it starts. `nil` refuses it and `false` cancels it, so no file is
  written; that is a change too, because without this option Windows and Linux let the engine save the file.
  `true` saves it in the user's downloads folder under a free name (`report.pdf`, then `report (1).pdf`), so
  a download never replaces a file that is already there. The engine may have received the response by the
  time it asks (that is where its suggested name comes from), but none of the body is written before the
  application answers.
- A navigation the engine turns into a download is decided in two steps on every engine: `Navigate` first,
  and only if it is allowed does `Download` see it. A refused `Navigate` cancels the navigation without
  asking `Download`. Popups are not navigations of the page: `NewWindow` alone decides them on every engine.
- `BridgeMainFrameOnly` injects `window.viva` into the main frame only, so an embedded frame does not get the
  bridge; `false` keeps it in every frame, as before. It restricts the script, not the engine's message
  channel: a frame that calls that channel itself is still heard, so an application that hosts content it does
  not trust should also check the origin of what `OnMessage` delivers.
- The decisions run synchronously on the window's thread and must return quickly; they see the navigation
  before anything is fetched, so a policy over an allowlist of origins cannot be raced by a redirect or a
  popup.
- Everything runs on the window's thread: `OnReady`, the callbacks, the methods of `WebHost` and `WebView`.
  Reach it from another goroutine with an `Invalidator` and `OnWake`; the work of `ServeAsync` and its
  `respond` is the one exception, and `respond` may be called from any goroutine. A callback is followed by a
  paint.
- `Proxy` (`"127.0.0.1:1080"`) holds a page to a SOCKS5 proxy on this machine: every request, loopback
  included, goes through it, names are resolved by the proxy, and WebRTC sends no UDP around it, so the page
  reaches only what the proxy reaches (a tunnel to a remote network, Tor, a test harness). On Windows each
  proxy has its own WebView2 browser, in a folder of its own under the user data folder; on Linux an ephemeral
  WebKit context (WebKitGTK 2.16 or newer). Either ends with its last page. macOS cannot hold a page to a
  SOCKS5 proxy, and `Open` fails with `ErrUnsupported` rather than open one that connects directly:
  WKWebView's `proxyConfigurations` (macOS 14) sends names to the proxy unresolved, but the system exempts
  loopback addresses from it, so `http://127.0.0.1:3000/` would reach the user's own machine.
- `Proxy: app.WebProxyDirect` bypasses the system proxy for this page alone, which an embedded document or
  workspace page needs when the machine's proxy rejects it. macOS needs 14 or newer and returns
  `ErrUnsupported` on older versions; Windows passes `--no-proxy-server` to the page's own WebView2
  environment; Linux sets WebKitGTK's no-proxy mode on the page's own context. This selects a route, not a
  privacy guarantee. Direct pages also use a nonpersistent store on macOS and Linux even when `Ephemeral` is
  false, so cookies and storage do not outlive them; Windows keeps its dedicated persistent WebView2 profile.
- `TopCornerRadius` rounds the page's two top corners in logical pixels, for a page that sits inside a rounded
  window or toolbar; zero keeps it rectangular. macOS cuts the rounded outline out of the page and refuses the
  pointer in the clipped corners, and Windows cuts the same outline into the child window's region. GTK has no
  native rounded page: `Open` fails with `ErrUnsupported` for a nonzero radius rather than open a page with
  square corners.
- `Ephemeral` keeps the page's cookies, `localStorage`, IndexedDB and caches in memory only, separate from
  every other page, and drops them when the page closes: a run of the application leaves nothing the page kept
  on disk. macOS gives the configuration a non-persistent `WKWebsiteDataStore`; Windows needs WebView2 Runtime
  101 (SDK 1.0.1210.39, the first runtime with `ICoreWebView2Environment10` and
  `ICoreWebView2ControllerOptions`) or newer, and runs the page in an InPrivate controller with a profile name of
  its own. That profile's metadata lives in the application's user data folder, while its cookies, storage and
  caches stay in memory; the distinct name is what keeps two ephemeral pages apart. WebView2 keeps that profile's
  metadata folder (`viva-ephemeral-N`) even for an InPrivate page, so it stays under the user data folder and
  accumulates across pages and runs, which an application that opens many ephemeral pages should know. Linux
  makes the page's own ephemeral `WebKitWebContext` with `webkit_web_context_new_ephemeral` and creates the page
  in it, and the context ends with the page (WebKitGTK 2.16 or newer); a page that also asks for a `Proxy` gets
  its own ephemeral context with the proxy's settings, not the context shared behind that proxy. Where the engine
  cannot do it, the page is refused or closed rather than opened with a store that persists.
- `Canvas.NativeView` declares where the page is, on the current layer and inside the current clip. What Viva
  paints on that layer or below is hidden by the page; what it paints on a higher layer (a menu, a tooltip,
  `ui`'s `f.PopupLayer`) stands over it, and a hit region on a higher layer takes the pointer where it lies
  over the page (the backdrop that closes a menu on an outside press).
- Keys go to the page after a press in it, and to Viva after a press in Viva's content.

## How each platform does it

| | Page | Viva over the page | Pointer over the page |
| --- | --- | --- | --- |
| Linux, Wayland | WebKitGTK widget in a `GtkOverlay` | the content subsurface lies above everything GTK draws; the page's place is a transparent hole in it | the subsurface's input region excludes the hole, except what Viva covers |
| Linux, X11 | the same | each part Viva draws over the page is a native child window over it ("piece") painting the same canvas; opaque, so a shadow's translucent edge is not blended with the page | pieces and invisible input windows (`GtkEventBox`) |
| macOS | `WKWebView`, a subview of the Metal view | masked off (`CAShapeLayer`) where Viva draws over it; the translucent edge of a shadow is not blended with the page | `hitTest:` refuses those parts and the covers |
| Windows | WebView2 in a child window | the child window's region leaves those parts out (`SetWindowRgn`) | the same, and for covers an almost invisible owned popup that hands the mouse back to the window |

The Linux build needs `libwebkit2gtk-4.1` (or 4.0); the GTK shell is built with `-tags vivagtk`. macOS needs
nothing; Windows needs the WebView2 runtime (part of Windows 11, installed with Edge on Windows 10).

## Not covered yet

A real input method inside a page, drag and drop into a page, permission prompts, a page's own popups
(select menus) on X11, several pages in one window on every platform, and accessibility of the page from
Viva's tree.

## What each engine cannot do

The decisions are the engine's own, and each one has a limit worth knowing:

- **macOS**: `WKNavigationAction` has no public user-gesture flag. `WebNavigation.UserGesture` is derived
  from the navigation type: a link click and a form submission are gestures, a script's navigation is not.
  A form submitted by a script therefore reads as a gesture, and a redirect keeps the gesture that started
  it. Downloads need macOS 11.3 (`WKDownload`); on an older system a download response is refused rather
  than saved, and `Download` is not asked.
- **Linux**: WebKitGTK's navigation action names no target frame (there is no way from the action to
  the frame), so `WebNavigation.MainFrame` is always true here and a subframe cannot be told apart from
  the main frame. The decision still cancels the right request. The redirect flag needs WebKitGTK 2.20;
  on an older engine a redirect keeps the gesture of the request that led to it. WebKitGTK starts a
  download for a response it treats as a download (an attachment, or a type it cannot show); the
  application decides it in `decide-destination`, before the body is written.
- **Windows**: `DownloadStarting` needs `ICoreWebView2_4`. A runtime older than that (2021) cannot report a
  download before it starts, so `Download` is not asked and the engine saves the file its own way in the
  user's downloads folder; a warning through `diag` says so once, when the page opens. `NewWindow` and
  `Navigate` are on the base interface and are always available.
