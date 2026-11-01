package windows

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/ui"
)

// Web pages inside the window (ADR 0064): WebView2, one controller per page, each in a child
// window of its own over the window's swap chain. A page gives way where Viva draws over it: the
// child window's region (SetWindowRgn) leaves those parts out, so Viva's picture shows and Viva
// gets the pointer there (platform.NativeCuts).

const (
	wmWebReady     = 32790
	wsExLayered    = 0x00080000
	wsExNoActivate = 0x08000000
	wsChild        = 0x40000000
	wsClipSibs     = 0x04000000
	wsClipKids     = 0x02000000
	swpNoActive    = 0x0010
	swpShow        = 0x0040
	rgnDiff        = 4
	webSchemeTLD   = ".viva-app.invalid"
)

var (
	webGDI          = systemDLL("gdi32.dll")
	createRectRgn   = webGDI.NewProc("CreateRectRgn")
	combineRgn      = webGDI.NewProc("CombineRgn")
	setWindowRgn    = user.NewProc("SetWindowRgn")
	getParent       = user.NewProc("GetParent")
	webHostClass    = "VivaWebHost"
	webHostRegister sync.Once
)

// webHost is the pages of one window.
type webHost struct {
	state *windowState
	pages map[uint32]*windowsWeb
	rects []platform.NativeView
	cuts  []ui.Rect
}

// windowsWeb is one page.
type windowsWeb struct {
	host   *webHost
	id     uint32
	config platform.WebViewConfig
	window uintptr // the child window the controller lives in
	// controller and view exist once the runtime has made them; until then actions queue.
	controller, view uintptr
	queued           []func()
	closed           bool
	// device is where the child window is, and cutsSet the region it was last given.
	device  rect
	cutsSet []ui.Rect
	shown   bool
	// covers are windows over the page that take the pointer for Viva where something Viva handles
	// lies over it without being drawn; they pass what they get to the window.
	covers   []uintptr
	coverSet []ui.Rect
}

func newWebHost(state *windowState, host *platform.WebHost, initial []platform.NativeView) *webHost {
	w := &webHost{state: state, pages: map[uint32]*windowsWeb{}}
	for _, view := range initial {
		view.Covers = append([]ui.Rect(nil), view.Covers...)
		w.rects = append(w.rects, view)
	}
	host.Open = w.open
	return w
}

// userDataFolder is where the runtime keeps a page's profile: beside the other things the program
// caches for the user, under its own name.
func userDataFolder(class string) string {
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	if class == "" {
		if executable, err := os.Executable(); err == nil {
			class = strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable))
		}
	}
	folder := filepath.Join(base, class, "WebView2")
	_ = os.MkdirAll(folder, 0o700)
	return folder
}

// open is WebHost.Open.
func (w *webHost) open(config platform.WebViewConfig) (platform.WebView, error) {
	state := w.state
	if state.window == 0 {
		return nil, platform.ErrWebViewUnsupported
	}
	if w.pages[config.ID] != nil {
		return nil, fmt.Errorf("windows: a web view with id %d is already open", config.ID)
	}
	if _, err := webViewClientLibrary(); err != nil {
		return nil, fmt.Errorf("%w: %v", platform.ErrWebViewUnsupported, err)
	}
	webHostRegister.Do(func() {
		name := wide(webHostClass)
		cursor, _, _ := loadCursor.Call(0, 32512)
		class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: defaultWindowProc.Addr(), Instance: state.application.instance, Cursor: cursor, Name: name}
		registerClass.Call(uintptr(unsafe.Pointer(&class)))
	})
	window, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(wide(webHostClass))), 0, wsChild|wsClipSibs|wsClipKids,
		0, 0, 1, 1, state.window, 0, state.application.instance, 0)
	if window == 0 {
		return nil, fmt.Errorf("windows: create the page window: %w", err)
	}
	page := &windowsWeb{host: w, id: config.ID, config: config, window: window}
	w.pages[config.ID] = page
	withEnvironment(userDataFolder(state.config.Class), func(environment uintptr, err error) {
		if err != nil || page.closed {
			fmt.Fprintln(os.Stderr, "windows:", err)
			return
		}
		page.createController(environment)
	})
	return page, nil
}

// createController makes the page's controller in its window.
func (page *windowsWeb) createController(environment uintptr) {
	handler := newHandler(func(status, controller uintptr) uintptr {
		if int32(uint32(status)) < 0 || controller == 0 || page.closed {
			if page.closed && controller != 0 {
				comCall(controller, controllerClose)
			}
			return 0
		}
		comCall(controller, 1)
		page.controller = controller
		comCall(controller, controllerGetWebView, uintptr(unsafe.Pointer(&page.view)))
		page.attach(environment)
		return 0
	})
	comCall(environment, envCreateController, page.window, handler)
}

// attach wires the page's events and what it is given, and runs what was asked of it meanwhile.
func (page *windowsWeb) attach(environment uintptr) {
	view := page.view
	var token int64
	comCall(view, viewAddMessageReceived, newHandler(func(_, args uintptr) uintptr {
		var text uintptr
		if comCall(args, messageTryGetString, uintptr(unsafe.Pointer(&text))) == 0 && page.config.OnMessage != nil {
			page.config.OnMessage(wideString(text))
		}
		return 0
	}), uintptr(unsafe.Pointer(&token)))
	comCall(view, viewAddNavStarting, newHandler(func(_, args uintptr) uintptr {
		var address uintptr
		comCall(args, startingGetURI, uintptr(unsafe.Pointer(&address)))
		page.loaded(platform.WebLoadStarted, wideString(address))
		return 0
	}), uintptr(unsafe.Pointer(&token)))
	for _, event := range []struct {
		slot  int
		phase platform.WebLoadPhase
	}{{viewAddContentLoading, platform.WebLoadCommitted}, {viewAddNavCompleted, platform.WebLoadFinished}} {
		phase := event.phase
		comCall(view, event.slot, newHandler(func(_, _ uintptr) uintptr {
			var address uintptr
			comCall(view, viewGetSource, uintptr(unsafe.Pointer(&address)))
			page.loaded(phase, wideString(address))
			return 0
		}), uintptr(unsafe.Pointer(&token)))
	}
	comCall(view, viewAddScriptOnCreate, uintptr(unsafe.Pointer(wide(platform.WebBridgeScript))), 0)
	if scheme := page.config.Scheme; scheme != nil && scheme.Name != "" {
		comCall(view, viewAddWebResourceReq, newHandler(func(_, args uintptr) uintptr {
			page.serve(environment, args)
			return 0
		}), uintptr(unsafe.Pointer(&token)))
		comCall(view, viewAddResourceFilter, uintptr(unsafe.Pointer(wide("https://"+scheme.Name+webSchemeTLD+"/*"))), resourceContextAll)
	}
	switch {
	case page.config.HTML != "":
		page.LoadHTML(page.config.HTML, page.config.BaseURL)
	case page.config.URL != "":
		page.Load(page.config.URL)
	}
	queued := page.queued
	page.queued = nil
	for _, action := range queued {
		action()
	}
	page.host.layout(page, true)
}

// do runs an action now, or when the page exists.
func (page *windowsWeb) do(action func()) {
	switch {
	case page.closed:
	case page.view == 0:
		page.queued = append(page.queued, action)
	default:
		action()
	}
}

// loaded reports a step of navigation, with the page's own scheme in the address as the
// application knows it.
func (page *windowsWeb) loaded(phase platform.WebLoadPhase, address string) {
	if page.closed || page.config.OnLoad == nil {
		return
	}
	if scheme := page.config.Scheme; scheme != nil {
		if rest, ok := strings.CutPrefix(address, "https://"+scheme.Name+webSchemeTLD+"/"); ok {
			address = scheme.Name + "://" + rest
		}
	}
	page.config.OnLoad(platform.WebLoad{Phase: phase, URL: address})
}

// serve answers a request for the page's scheme, which is served as https://<name>.viva-app.invalid/
// (WebView2 takes a custom scheme only when it is registered with the environment, before any
// page exists). The path after the first segment, the host the application wrote, is what Serve gets.
func (page *windowsWeb) serve(environment, args uintptr) {
	scheme := page.config.Scheme
	var request, address uintptr
	comCall(args, resourceArgsGetRequest, uintptr(unsafe.Pointer(&request)))
	comCall(request, resourceRequestGetURI, uintptr(unsafe.Pointer(&address)))
	comCall(request, 2)
	parsed, err := url.Parse(wideString(address))
	if err != nil || scheme == nil || scheme.Serve == nil {
		return
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	if _, rest, ok := strings.Cut(path, "/"); ok {
		path = rest
	} else {
		path = ""
	}
	mime, data, found := scheme.Serve(path)
	status, reason := 200, "OK"
	if !found {
		status, reason, mime, data = 404, "Not Found", "text/plain", nil
	}
	var stream uintptr
	if len(data) > 0 {
		stream, _, _ = createMemoryBlob.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)))
	} else {
		stream, _, _ = createMemoryBlob.Call(0, 0)
	}
	headers := "Content-Type: " + mime + "\r\nAccess-Control-Allow-Origin: *"
	var response uintptr
	comCall(environment, envCreateResponse, stream, uintptr(status), uintptr(unsafe.Pointer(wide(reason))), uintptr(unsafe.Pointer(wide(headers))), uintptr(unsafe.Pointer(&response)))
	if response != 0 {
		comCall(args, resourceArgsPutResponse, response)
		comCall(response, 2)
	}
	if stream != 0 {
		comCall(stream, 2)
	}
}

// mapAddress turns an address in the page's scheme into the one it is served at.
func (page *windowsWeb) mapAddress(address string) string {
	if scheme := page.config.Scheme; scheme != nil {
		if rest, ok := strings.CutPrefix(address, scheme.Name+"://"); ok {
			return "https://" + scheme.Name + webSchemeTLD + "/" + rest
		}
	}
	return address
}

func (page *windowsWeb) Load(address string) {
	address = page.mapAddress(address)
	page.do(func() { comCall(page.view, viewNavigate, uintptr(unsafe.Pointer(wide(address)))) })
}

func (page *windowsWeb) LoadHTML(html, base string) {
	page.do(func() { comCall(page.view, viewNavigateToString, uintptr(unsafe.Pointer(wide(html)))) })
}

func (page *windowsWeb) Reload()  { page.do(func() { comCall(page.view, viewReload) }) }
func (page *windowsWeb) Back()    { page.do(func() { comCall(page.view, viewGoBack) }) }
func (page *windowsWeb) Forward() { page.do(func() { comCall(page.view, viewGoForward) }) }

func (page *windowsWeb) Eval(script string) {
	page.do(func() { comCall(page.view, viewExecuteScript, uintptr(unsafe.Pointer(wide(script))), 0) })
}

func (page *windowsWeb) PostMessage(text string) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return
	}
	page.Eval("window.viva.deliver(" + string(encoded) + ");")
}

func (page *windowsWeb) Focus() {
	page.do(func() { comCall(page.controller, controllerMoveFocus, 0) })
}

func (page *windowsWeb) Close() {
	if page.closed {
		return
	}
	page.closed = true
	delete(page.host.pages, page.id)
	page.release()
}

// release lets go of the runtime's objects and the page's window.
func (page *windowsWeb) release() {
	if page.controller != 0 {
		comCall(page.controller, controllerClose)
		comCall(page.view, 2)
		comCall(page.controller, 2)
		page.controller, page.view = 0, 0
	}
	for _, cover := range page.covers {
		destroyWindow.Call(cover)
	}
	page.covers = nil
	if page.window != 0 {
		destroyWindow.Call(page.window)
		page.window = 0
	}
}

// frame takes what a frame says about the places of the pages and puts them there.
func (w *webHost) frame(views []platform.NativeView) {
	if views != nil {
		w.rects = w.rects[:0]
		for _, view := range views {
			view.Covers = append([]ui.Rect(nil), view.Covers...)
			w.rects = append(w.rects, view)
		}
	}
	w.layoutAll(false)
}

func (w *webHost) layoutAll(force bool) {
	for _, page := range w.pages {
		w.layout(page, force)
	}
}

// layout puts a page where the application declared it, with a hole in it for what Viva draws over it.
func (w *webHost) layout(page *windowsWeb, force bool) {
	if page.closed || page.window == 0 || page.controller == 0 {
		return
	}
	scale := w.state.scale
	view, found := platform.NativeView{}, false
	for _, candidate := range w.rects {
		if candidate.ID == page.id {
			view, found = candidate, true
			break
		}
	}
	if !found {
		if page.shown {
			for _, cover := range page.covers {
				showWindow.Call(cover, swHide)
			}
			showWindow.Call(page.window, swHide)
			comCall(page.controller, controllerPutVisible, 0)
			page.shown = false
		}
		return
	}
	device := rect{
		Left: int32(round(view.Bounds.X * scale)), Top: int32(round(view.Bounds.Y * scale)),
		Right: int32(round((view.Bounds.X + view.Bounds.Width) * scale)), Bottom: int32(round((view.Bounds.Y + view.Bounds.Height) * scale)),
	}
	w.layoutCovers(page, view)
	w.cuts = platform.NativeCuts(view, w.state.textRectangles, w.state.textLabels, w.cuts[:0])
	if !force && page.shown && page.device == device && sameRects(page.cutsSet, w.cuts) {
		return
	}
	page.device, page.cutsSet, page.shown = device, append(page.cutsSet[:0], w.cuts...), true
	width, height := device.Right-device.Left, device.Bottom-device.Top
	setWindowPosition.Call(page.window, 0, uintptr(device.Left), uintptr(device.Top), uintptr(width), uintptr(height), swpNoActive|swpShow)
	comCall(page.controller, controllerPutVisible, 1)
	putBounds(page.controller, rect{Right: width, Bottom: height})
	region, _, _ := createRectRgn.Call(0, 0, uintptr(width), uintptr(height))
	for _, cut := range w.cuts {
		left, top := int32(round((cut.X-view.Bounds.X)*scale)), int32(round((cut.Y-view.Bounds.Y)*scale))
		right, bottom := int32(round((cut.X+cut.Width-view.Bounds.X)*scale)), int32(round((cut.Y+cut.Height-view.Bounds.Y)*scale))
		part, _, _ := createRectRgn.Call(uintptr(left), uintptr(top), uintptr(right), uintptr(bottom))
		combineRgn.Call(region, region, part, rgnDiff)
		deleteObject.Call(part)
	}
	setWindowRgn.Call(page.window, region, 1) // the window owns the region now
}

// pointerOverCover reports whether the pointer is over one of the covers.
func (w *webHost) pointerOverCover() bool {
	var at point
	getCursorPos.Call(uintptr(unsafe.Pointer(&at)))
	under := windowAtPoint(at)
	for _, page := range w.pages {
		for _, cover := range page.covers {
			if cover == under {
				return true
			}
		}
	}
	return false
}

// moved puts the covers over the page again after the window moved: they are placed on the screen.
func (w *webHost) moved() {
	for _, page := range w.pages {
		page.coverSet = nil
		w.layout(page, false)
	}
}

// layoutCovers puts the windows that hand the pointer to Viva where something Viva handles, and
// draws nothing, lies over the page.
func (w *webHost) layoutCovers(page *windowsWeb, view platform.NativeView) {
	scale := w.state.scale
	w.cuts = platform.NativeCovers(view, w.cuts[:0])
	if sameRects(page.coverSet, w.cuts) {
		return
	}
	page.coverSet = append(page.coverSet[:0], w.cuts...)
	origin := point{}
	clientToScreen.Call(w.state.window, uintptr(unsafe.Pointer(&origin)))
	for i, cover := range w.cuts {
		if i == len(page.covers) {
			page.covers = append(page.covers, w.newCover())
		}
		at := rect{
			Left: int32(round(cover.X * scale)), Top: int32(round(cover.Y * scale)),
			Right: int32(round((cover.X + cover.Width) * scale)), Bottom: int32(round((cover.Y + cover.Height) * scale)),
		}
		setWindowPosition.Call(page.covers[i], 0, uintptr(at.Left+origin.X), uintptr(at.Top+origin.Y), uintptr(at.Right-at.Left), uintptr(at.Bottom-at.Top), swpNoActive|swpShow)
	}
	for _, window := range page.covers[min(len(w.cuts), len(page.covers)):] {
		showWindow.Call(window, swHide)
	}
}

var (
	coverRegister sync.Once
	coverClass    = "VivaWebCover"
	setLayered    = user.NewProc("SetLayeredWindowAttributes")
)

// newCover makes a window that is all but invisible (a layered window with an alpha of 1 in 255,
// which still takes the pointer) and hands the mouse to the window it covers.
func (w *webHost) newCover() uintptr {
	state := w.state
	coverRegister.Do(func() {
		cursor, _, _ := loadCursor.Call(0, 32512)
		class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: syscall.NewCallback(coverProc), Instance: state.application.instance, Cursor: cursor, Name: wide(coverClass)}
		registerClass.Call(uintptr(unsafe.Pointer(&class)))
	})
	// A popup the window owns, not a child: a layered child needs the program to say it runs on
	// Windows 8 or later in a manifest, which a Go program does not have unless its builder adds one.
	window, _, _ := createWindow.Call(wsExLayered|wsExNoActivate|wsExToolWindow, uintptr(unsafe.Pointer(wide(coverClass))), 0, wsPopup,
		0, 0, 1, 1, state.window, 0, state.application.instance, 0)
	if window != 0 {
		setLayered.Call(window, 0, 1, 2)
	}
	return window
}

// coverProc forwards the mouse to the window the cover is a child of, in its coordinates.
func coverProc(window, kind, first, second uintptr) uintptr {
	switch kind {
	case wmMouseMove, wmLeftDown, wmLeftUp, wmRightDown, wmRightUp, wmMiddleDown, wmMiddleUp, wmXButtonDown, wmXButtonUp:
		parent, _, _ := getParent.Call(window)
		at := point{X: int32(int16(second)), Y: int32(int16(second >> 16))}
		clientToScreen.Call(window, uintptr(unsafe.Pointer(&at)))
		screenToClient.Call(parent, uintptr(unsafe.Pointer(&at)))
		postMessage.Call(parent, kind, first, uintptr(uint16(at.X))|uintptr(uint16(at.Y))<<16)
		return 0
	case wmWheel, wmHorizontalWheel:
		parent, _, _ := getParent.Call(window) // the wheel's position is the screen's already
		postMessage.Call(parent, kind, first, second)
		return 0
	case wmErase:
		return 1
	}
	result, _, _ := defaultWindowProc.Call(window, kind, first, second)
	return result
}

func round(value float32) float32 {
	if value < 0 {
		return -round(-value)
	}
	return float32(int64(value + 0.5))
}

func sameRects(a, b []ui.Rect) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// closeAll lets go of every page when the window goes.
func (w *webHost) closeAll() {
	for _, page := range w.pages {
		page.closed = true
		page.release()
	}
	clear(w.pages)
}

var _ = syscall.Errno(0)
var _ render.Rectangle
