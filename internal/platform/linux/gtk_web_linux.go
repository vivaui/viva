//go:build linux && vivagtk

package linux

import (
	"encoding/json"
	"fmt"
	"math"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Web pages inside the window (ADR 0064): the system's WebKitGTK, one widget per page, stacked over
// the window's drawing area in a GtkOverlay and placed where Viva leaves a hole. What Viva draws on
// a layer above the hole shows over the page (the Wayland subsurface lies above everything GTK
// draws). The engine is loaded when a window opens its first page; an application that opens none
// never loads it.

// webKit is the engine's entry points.
type webKit struct {
	managerNew        func() uintptr
	managerRegister   func(manager uintptr, name *byte) int32
	managerAddScript  func(manager, script uintptr)
	scriptNew         func(source *byte, frames, time int32, allow, block uintptr) uintptr
	viewNew           func(manager uintptr) uintptr
	loadURI           func(view uintptr, uri *byte)
	loadHTML          func(view uintptr, content, base *byte)
	reload            func(view uintptr)
	goBack, goForward func(view uintptr)
	getURI            func(view uintptr) uintptr
	getContext        func(view uintptr) uintptr
	setZoom           func(view uintptr, level float64)
	registerScheme    func(context uintptr, scheme *byte, callback, data, destroy uintptr)
	securityManager   func(context uintptr) uintptr
	schemeSecure      func(manager uintptr, scheme *byte)
	schemeCORS        func(manager uintptr, scheme *byte)
	schemeURI         func(request uintptr) uintptr
	schemeFinish      func(request, stream uintptr, length int64, mime *byte)
	schemeFinishError func(request, err uintptr)
	jscString         func(value uintptr) uintptr
	resultValue       func(result uintptr) uintptr
	memoryStream      func(data uintptr, length int64, destroy uintptr) uintptr
	gMalloc           func(size uint64) uintptr
	quarkFrom         func(name *byte) uint32
	errorLiteral      func(domain uint32, code int32, message *byte) uintptr
	// Either way of running a script is there: the first from 2.40, the second before.
	evaluate func(view uintptr, script *byte, length int64, world, source, cancellable, callback, data uintptr)
	run      func(view uintptr, script *byte, cancellable, callback, data uintptr)
	free     uintptr
}

var (
	webOnce sync.Once
	webAPI  *webKit
	webErr  error
)

func loadWebKit(g *gtkLibrary) (*webKit, error) {
	webOnce.Do(func() {
		var err error
		found := false
		for _, name := range []string{"libwebkit2gtk-4.1.so.0", "libwebkit2gtk-4.0.so.37"} {
			var library uintptr
			if library, err = purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				g.libraries = append(g.libraries, library)
				found = true
				break
			}
		}
		if !found {
			webErr = fmt.Errorf("%w: WebKitGTK is not installed: install libwebkit2gtk-4.1-0 (Debian, Ubuntu) or webkit2gtk4.1 (Fedora)", platform.ErrWebViewUnsupported)
			return
		}
		for _, name := range []string{"libjavascriptcoregtk-4.1.so.0", "libjavascriptcoregtk-4.0.so.18"} {
			if library, err := purego.Dlopen(name, purego.RTLD_NOW|purego.RTLD_GLOBAL); err == nil {
				g.libraries = append(g.libraries, library)
				break
			}
		}
		k := &webKit{}
		for _, r := range []struct {
			function any
			symbol   string
		}{
			{&k.resultValue, "webkit_javascript_result_get_js_value"},
			{&k.managerNew, "webkit_user_content_manager_new"}, {&k.managerRegister, "webkit_user_content_manager_register_script_message_handler"},
			{&k.managerAddScript, "webkit_user_content_manager_add_script"}, {&k.scriptNew, "webkit_user_script_new"},
			{&k.viewNew, "webkit_web_view_new_with_user_content_manager"}, {&k.loadURI, "webkit_web_view_load_uri"},
			{&k.loadHTML, "webkit_web_view_load_html"}, {&k.reload, "webkit_web_view_reload"}, {&k.goBack, "webkit_web_view_go_back"},
			{&k.goForward, "webkit_web_view_go_forward"}, {&k.getURI, "webkit_web_view_get_uri"}, {&k.getContext, "webkit_web_view_get_context"}, {&k.setZoom, "webkit_web_view_set_zoom_level"},
			{&k.registerScheme, "webkit_web_context_register_uri_scheme"}, {&k.schemeURI, "webkit_uri_scheme_request_get_uri"},
			{&k.schemeFinish, "webkit_uri_scheme_request_finish"}, {&k.schemeFinishError, "webkit_uri_scheme_request_finish_error"},
			{&k.jscString, "jsc_value_to_string"},
			{&k.memoryStream, "g_memory_input_stream_new_from_data"}, {&k.gMalloc, "g_malloc"},
			{&k.quarkFrom, "g_quark_from_string"}, {&k.errorLiteral, "g_error_new_literal"},
		} {
			if !g.lookup(r.function, r.symbol) {
				webErr = fmt.Errorf("%w: WebKitGTK lacks %s", platform.ErrWebViewUnsupported, r.symbol)
				return
			}
		}
		if !g.lookup(&k.evaluate, "webkit_web_view_evaluate_javascript") {
			k.evaluate = nil
		}
		if !g.lookup(&k.run, "webkit_web_view_run_javascript") {
			k.run = nil
		}
		if k.evaluate == nil && k.run == nil {
			webErr = fmt.Errorf("%w: WebKitGTK cannot run scripts", platform.ErrWebViewUnsupported)
			return
		}
		g.lookup(&k.securityManager, "webkit_web_context_get_security_manager")
		g.lookup(&k.schemeSecure, "webkit_security_manager_register_uri_scheme_as_secure")
		g.lookup(&k.schemeCORS, "webkit_security_manager_register_uri_scheme_as_cors_enabled")
		k.free = g.symbol("g_free")
		webAPI = k
	})
	return webAPI, webErr
}

// webQuirks keeps a virtual machine or a container from breaking the engine: without a GPU render
// node its buffer sharing between processes fails and pages stay blank.
func webQuirks() {
	if os.Getenv("WEBKIT_DISABLE_DMABUF_RENDERER") == "" {
		if _, err := os.Stat("/dev/dri/renderD128"); err != nil {
			os.Setenv("WEBKIT_DISABLE_DMABUF_RENDERER", "1")
		}
	}
}

// WebKitLoadEvent.
const (
	webLoadStarted   = 0
	webLoadCommitted = 2
	webLoadFinished  = 3
)

// gtkWebView is one page.
type gtkWebView struct {
	w       *gtkWindow
	k       *webKit
	id      uint32
	handle  uintptr // the key of its callbacks
	widget  uintptr
	manager uintptr
	config  platform.WebViewConfig
	closed  bool
	shown   bool
	// at is where the widget is, in widget units; zoom is the page's zoom.
	at   viewPlace
	zoom float64
}

// viewPlace is a rectangle in the overlay's units.
type viewPlace struct{ x, y, width, height int32 }

var (
	webViews      = map[uintptr]*gtkWebView{}
	webNext       uintptr
	webSchemes    = map[string]*platform.WebScheme{}
	webSchemeDone = map[string]bool{}
)

// openWebView is the window's WebHost.Open.
func (w *gtkWindow) openWebView(c platform.WebViewConfig) (platform.WebView, error) {
	if w.closed || w.overlay == 0 {
		return nil, platform.ErrWebViewUnsupported
	}
	for _, v := range w.views {
		if v.id == c.ID {
			return nil, fmt.Errorf("linux: a web view with id %d is already open", c.ID)
		}
	}
	g := w.g
	webQuirks()
	k, err := loadWebKit(g)
	if err != nil {
		return nil, err
	}
	cb := callbacks()
	webNext++
	v := &gtkWebView{w: w, k: k, id: c.ID, handle: webNext, config: c}
	v.manager = k.managerNew()
	name := cString("viva")
	k.managerRegister(v.manager, &name[0])
	source := cString(platform.WebBridgeScript)
	k.managerAddScript(v.manager, k.scriptNew(&source[0], 0, 0, 0, 0))
	g.connect(v.manager, "script-message-received::viva", cb.webMessage, v.handle)
	v.widget = k.viewNew(v.manager)
	// Placed by its margins from the overlay's top left; hidden until a frame says where.
	g.widgetSetHAlign(v.widget, 1)
	g.widgetSetVAlign(v.widget, 1)
	g.overlayAdd(w.overlay, v.widget)
	g.connect(v.widget, "load-changed", cb.webLoad, v.handle)
	g.connect(v.widget, "focus-in-event", cb.event, w.id)
	g.connect(v.widget, "focus-out-event", cb.event, w.id)
	if c.Scheme != nil && c.Scheme.Name != "" {
		v.registerScheme(c.Scheme)
	}
	webViews[v.handle] = v
	w.views = append(w.views, v)
	switch {
	case c.HTML != "":
		v.LoadHTML(c.HTML, c.BaseURL)
	case c.URL != "":
		v.Load(c.URL)
	}
	if len(w.viewRects) > 0 {
		w.layoutViews() // the application may have declared its place before it opened
	}
	return v, nil
}

// registerScheme makes the engine ask the application for a custom scheme's resources. The engine's
// default context is shared by every page, so the scheme is registered once and answered by the
// latest page that declared it.
func (v *gtkWebView) registerScheme(s *platform.WebScheme) {
	webSchemes[s.Name] = s
	if webSchemeDone[s.Name] {
		return
	}
	webSchemeDone[s.Name] = true
	k := v.k
	name := cString(s.Name)
	context := k.getContext(v.widget)
	k.registerScheme(context, &name[0], callbacks().webScheme, 0, 0)
	if k.securityManager != nil {
		manager := k.securityManager(context)
		if k.schemeSecure != nil {
			k.schemeSecure(manager, &name[0])
		}
		if k.schemeCORS != nil {
			k.schemeCORS(manager, &name[0])
		}
	}
}

func (v *gtkWebView) Load(address string) {
	if v.closed {
		return
	}
	text := cString(address)
	v.k.loadURI(v.widget, &text[0])
}

func (v *gtkWebView) LoadHTML(html, base string) {
	if v.closed {
		return
	}
	content := cString(html)
	var baseAddress *byte
	if base != "" {
		text := cString(base)
		baseAddress = &text[0]
	}
	v.k.loadHTML(v.widget, &content[0], baseAddress)
}

func (v *gtkWebView) Reload() {
	if !v.closed {
		v.k.reload(v.widget)
	}
}

func (v *gtkWebView) Back() {
	if !v.closed {
		v.k.goBack(v.widget)
	}
}

func (v *gtkWebView) Forward() {
	if !v.closed {
		v.k.goForward(v.widget)
	}
}

func (v *gtkWebView) Eval(script string) {
	if v.closed {
		return
	}
	text := cString(script)
	if v.k.evaluate != nil {
		v.k.evaluate(v.widget, &text[0], -1, 0, 0, 0, 0, 0)
		return
	}
	v.k.run(v.widget, &text[0], 0, 0, 0)
}

func (v *gtkWebView) PostMessage(text string) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return
	}
	v.Eval("window.viva.deliver(" + string(encoded) + ");")
}

func (v *gtkWebView) Focus() {
	if !v.closed && v.shown {
		v.w.g.widgetGrabFocus(v.widget)
	}
}

func (v *gtkWebView) Close() {
	if v.closed {
		return
	}
	w := v.w
	v.release()
	views := w.views[:0]
	for _, other := range w.views {
		if other != v {
			views = append(views, other)
		}
	}
	clear(w.views[len(views):])
	w.views = views
	w.g.widgetDestroy(v.widget)
	w.layoutViews()
}

// release forgets the page; its widget goes with the window or with Close.
func (v *gtkWebView) release() {
	v.closed = true
	delete(webViews, v.handle)
	if v.w.webFocus == v.widget {
		v.w.webFocus = 0
		v.w.setNativeKeys(false)
	}
}

// place puts the page where Viva left its hole; a page with no place is hidden.
func (v *gtkWebView) place(at viewPlace, found bool) {
	g := v.w.g
	// A widget unit is not a logical pixel when GTK draws at a scale of its own (X11): the page
	// is zoomed so that its pixels are.
	if zoom := float64(v.w.ratio); found && zoom != v.zoom {
		v.zoom = zoom
		v.k.setZoom(v.widget, zoom)
	}
	if !found {
		if v.shown {
			g.widgetHide(v.widget)
			v.shown = false
		}
		return
	}
	if !v.shown || v.at != at {
		v.at = at
		g.widgetSetMarginX(v.widget, at.x)
		g.widgetSetMarginY(v.widget, at.y)
		g.widgetSetSize(v.widget, max(at.width, 1), max(at.height, 1))
	}
	if !v.shown {
		g.widgetShow(v.widget)
		v.shown = true
	}
}

// message is the page posting text to the application; result is a WebKitJavascriptResult.
func (v *gtkWebView) message(result uintptr) {
	if v.closed || v.config.OnMessage == nil || result == 0 {
		return
	}
	k := v.k
	value := k.resultValue(result)
	text := k.jscString(value)
	if text == 0 {
		return
	}
	defer v.w.g.gFree(text)
	v.config.OnMessage(goString(text))
}

// loadChanged follows the page's navigation.
func (v *gtkWebView) loadChanged(event int32) {
	if v.closed || v.config.OnLoad == nil {
		return
	}
	address := goString(v.k.getURI(v.widget))
	switch event {
	case webLoadStarted:
		v.config.OnLoad(platform.WebLoad{Phase: platform.WebLoadStarted, URL: address})
	case webLoadCommitted:
		v.config.OnLoad(platform.WebLoad{Phase: platform.WebLoadCommitted, URL: address})
	case webLoadFinished:
		v.config.OnLoad(platform.WebLoad{Phase: platform.WebLoadFinished, URL: address})
	}
}

// serveScheme answers a request to a custom scheme from the application's resources.
func serveScheme(g *gtkLibrary, request uintptr) {
	k := webAPI
	if k == nil {
		return
	}
	address := goString(k.schemeURI(request))
	parsed, err := url.Parse(address)
	if err != nil {
		parsed = &url.URL{}
	}
	var (
		mime string
		data []byte
		ok   bool
	)
	if scheme := webSchemes[parsed.Scheme]; scheme != nil && scheme.Serve != nil {
		mime, data, ok = scheme.Serve(strings.TrimPrefix(parsed.Path, "/"))
	}
	if !ok {
		domain, message := cString("viva"), cString("not found")
		failure := k.errorLiteral(k.quarkFrom(&domain[0]), 404, &message[0])
		k.schemeFinishError(request, failure)
		g.errorFree(failure)
		return
	}
	memory := k.gMalloc(uint64(max(len(data), 1)))
	copy(unsafeSlice(memory, len(data)), data)
	stream := k.memoryStream(memory, int64(len(data)), k.free)
	mimeText := cString(mime)
	k.schemeFinish(request, stream, int64(len(data)), &mimeText[0])
	g.objectUnref(stream)
}

// setNativeViews takes the places the application keeps for native views.
func (w *gtkWindow) setNativeViews(views []platform.NativeView) {
	w.viewRects = append(w.viewRects[:0], views...)
	w.layoutViews()
}

// layoutViews puts the pages where the hole for each is. The hole is on the grid the widgets are
// placed on, so the bounds are brought to whole widget units first.
func (w *gtkWindow) layoutViews() {
	if w.scene == nil {
		return
	}
	ratio := float64(w.ratio)
	w.snapped = w.snapped[:0]
	w.places = w.places[:0]
	for _, view := range w.viewRects {
		b := view.Bounds
		left, top := math.Round(float64(b.X)*ratio), math.Round(float64(b.Y)*ratio)
		right, bottom := math.Round(float64(b.X+b.Width)*ratio), math.Round(float64(b.Y+b.Height)*ratio)
		if right <= left || bottom <= top {
			continue
		}
		w.snapped = append(w.snapped, platform.NativeView{ID: view.ID, Layer: view.Layer, Covers: view.Covers, Bounds: ui.Rect{
			X: float32(left / ratio), Y: float32(top / ratio), Width: float32((right - left) / ratio), Height: float32((bottom - top) / ratio),
		}})
		w.places = append(w.places, viewPlace{int32(left), int32(top), int32(right - left), int32(bottom - top)})
	}
	w.scene.setViews(w.snapped)
	for _, v := range w.views {
		at, found := viewPlace{}, false
		for i := range w.snapped {
			if w.snapped[i].ID == v.id {
				at, found = w.places[i], true
				break
			}
		}
		v.place(at, found)
	}
	w.updateOverlayRegions()
	w.queuePaint()
}

// updateOverlayRegions makes the pointer, and on X11 the picture, the page's where it shows and
// Viva's where Viva draws over it.
func (w *gtkWindow) updateOverlayRegions() {
	if w.wl != nil {
		w.wl.updateRegions(w, int32(w.width+0.5), int32(w.height+0.5))
		return
	}
	w.layoutPieces()
}

// webFocusChanged is a page gaining or losing the keyboard.
func (w *gtkWindow) webFocusChanged(widget uintptr, in bool) {
	switch {
	case in:
		w.webFocus = widget
		w.setNativeKeys(true)
	case w.webFocus == widget:
		w.webFocus = 0
		w.setNativeKeys(false)
	}
}

// setNativeKeys says who has the keyboard: a page (true) or Viva. While a page has it the keys go
// to GTK's focus widget and the input method is the page's.
func (w *gtkWindow) setNativeKeys(on bool) {
	if w.nativeKeys == on {
		return
	}
	w.nativeKeys = on
	if on {
		clear(w.keys.down)
		if w.config.OnFocusLost != nil {
			frame, err := w.config.OnFocusLost()
			w.present(frame, err)
		}
	}
	if w.ime != nil {
		w.ime.sync()
	}
}

// takeKeys gives the keyboard back to Viva, as a press in Viva's content does.
func (w *gtkWindow) takeKeys() {
	if w.webFocus != 0 {
		w.g.windowSetFocus(w.toplevel, 0)
	}
}
