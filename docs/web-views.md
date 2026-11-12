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
  instead: a status (0 is 200), a MIME type, headers and a body; `ok` false is a 404. `ServeAsync` replaces
  both: it is called on the window's thread, does its work wherever it likes, and answers once through
  `respond`, from any goroutine; a second call, or one after the page closed, is ignored. `Headers` are merged
  over `Content-Type`, `Content-Length` and `Access-Control-Allow-Origin: *`: a name is case-insensitive, and a
  header with an empty value removes the default. A 204 or 304 carries no body. On Windows the scheme is
  served as `https://name.viva-app.invalid/host/path` (WebView2 only takes a custom scheme registered before
  any page exists), so write relative URLs in the pages; `OnLoad` reports addresses with the scheme as you
  wrote it. `LoadHTML`'s base URL is ignored on Windows.
- On Linux, the status and headers of a scheme's answer need WebKitGTK's `WebKitURISchemeResponse` (2.36 or
  newer, and the 2.44 build used here has it). An older engine that lacks it finishes with
  `webkit_uri_scheme_request_finish`, which drops them: the body is served as 200, and a 404 becomes the
  engine's failed load. macOS (`NSHTTPURLResponse`) and Windows (`CreateWebResourceResponse`) answer with
  the full response.
- The page and the application talk through `window.viva.postMessage(text)` / `window.viva.onmessage` and
  `OnMessage` / `WebView.PostMessage`. A page sees nothing else of the application.
- Everything runs on the window's thread: `OnReady`, the callbacks, the methods of `WebHost` and `WebView`.
  Reach it from another goroutine with an `Invalidator` and `OnWake`; the work of `ServeAsync` and its
  `respond` is the one exception, and `respond` may be called from any goroutine. A callback is followed by a
  paint.
- `Proxy` (`"127.0.0.1:1080"`) holds a page to a SOCKS5 proxy on this machine: every request, loopback
  included, goes through it, names are resolved by the proxy, and WebRTC sends no UDP around it, so the page
  reaches only what the proxy reaches (a tunnel to a remote network, Tor, a test harness). On Windows each
  proxy has its own WebView2 browser, in a folder of its own under the user data folder; on Linux an ephemeral
  WebKit context (WebKitGTK 2.16 or newer). Either ends with its last page. macOS cannot hold a page to a proxy,
  and `Open` fails with `ErrUnsupported` rather than open one that connects directly: WKWebView's
  `proxyConfigurations` (macOS 14) sends names to the proxy unresolved, but the system exempts loopback
  addresses from it, so `http://127.0.0.1:3000/` would reach the user's own machine.
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

A real input method inside a page, drag and drop into a page, `window.open`/`target=_blank`, downloads,
permission prompts, a page's own popups (select menus) on X11, several pages in one window on every platform,
and accessibility of the page from Viva's tree.
