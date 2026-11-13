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

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/diag"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// Web pages inside the window (ADR 0064): WebView2, one controller per page, each in a child
// window of its own over the window's swap chain. A page gives way where Viva draws over it: the
// child window's region (SetWindowRgn) leaves those parts out, so Viva's picture shows and Viva
// gets the pointer there (platform.NativeCuts).

const (
	wmWebReady     = 32790
	wmWebAnswer    = 32791
	wsExLayered    = 0x00080000
	wsExNoActivate = 0x08000000
	wsChild        = 0x40000000
	wsClipSibs     = 0x04000000
	wsClipKids     = 0x02000000
	swpNoActive    = 0x0010
	swpShow        = 0x0040
	rgnDiff        = 4
	rgnOr          = 2
	webSchemeTLD   = ".viva-app.invalid"
)

var (
	webGDI             = systemDLL("gdi32.dll")
	createRectRgn      = webGDI.NewProc("CreateRectRgn")
	createRoundRectRgn = webGDI.NewProc("CreateRoundRectRgn")
	combineRgn         = webGDI.NewProc("CombineRgn")
	setWindowRgn    = user.NewProc("SetWindowRgn")
	getParent       = user.NewProc("GetParent")
	webHostClass    = "VivaWebHost"
	webHostRegister sync.Once

	shGetKnownFolderPath = shell.NewProc("SHGetKnownFolderPath")
)

// webAnswer is an asynchronous scheme request the runtime is holding open with a deferral, and the
// answer the application gave, which the UI thread turns into the runtime's response.
type webAnswer struct {
	page     *windowsWeb
	args     uintptr
	deferral uintptr
	response platform.WebResponse
	answered bool
}

var (
	webAnswersMu  sync.Mutex
	webAnswers    = map[uintptr]*webAnswer{}
	webAnswerNext uintptr
)

// folderIDDownloads is FOLDERID_Downloads, the known folder the user keeps downloads in.
var folderIDDownloads = guid{0x374de290, 0x123f, 0x4565, [8]byte{0x91, 0x64, 0x39, 0xc4, 0x92, 0x5e, 0x46, 0x7b}}

// interfaceWebView4 is IID_ICoreWebView2_4: the interface that adds DownloadStarting to the view.
var interfaceWebView4 = guid{0x20d02d59, 0x6df2, 0x42dc, [8]byte{0xbd, 0x06, 0xf9, 0x8a, 0x69, 0x4b, 0x13, 0x02}}

// webHost is the pages of one window.
type webHost struct {
	state *windowState
	pages map[uint32]*windowsWeb
	rects []platform.NativeView
	cuts  []gfx.Rect
}

// windowsWeb is one page.
type windowsWeb struct {
	host   *webHost
	id     uint32
	config platform.WebViewConfig
	window uintptr // the child window the controller lives in
	// environment is the runtime's browser the page is in, held until the page closes.
	environment *webEnvironment
	// controller and view exist once the runtime has made them; until then actions queue.
	controller, view uintptr
	queued           []func()
	closed           bool
	// device is where the child window is, and cutsSet the region it was last given.
	device  rect
	cutsSet []gfx.Rect
	shown   bool
	// covers are windows over the page that take the pointer for Viva where something Viva handles
	// lies over it without being drawn; they pass what they get to the window.
	covers   []uintptr
	coverSet []gfx.Rect
}

func newWebHost(state *windowState, host *platform.WebHost, initial []platform.NativeView) *webHost {
	w := &webHost{state: state, pages: map[uint32]*windowsWeb{}}
	for _, view := range initial {
		view.Covers = append([]gfx.Rect(nil), view.Covers...)
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
		class := windowClass{Size: uint32(unsafe.Sizeof(windowClass{})), Procedure: syscall.NewCallback(webHostProc), Instance: state.application.instance, Cursor: cursor, Name: name}
		registerClass.Call(uintptr(unsafe.Pointer(&class)))
	})
	window, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(wide(webHostClass))), 0, wsChild|wsClipSibs|wsClipKids,
		0, 0, 1, 1, state.window, 0, state.application.instance, 0)
	if window == 0 {
		return nil, fmt.Errorf("windows: create the page window: %w", err)
	}
	page := &windowsWeb{host: w, id: config.ID, config: config, window: window}
	w.pages[config.ID] = page
	page.environment = withEnvironment(userDataFolder(state.config.Class), config.Proxy, func(environment uintptr, err error) {
		if page.closed {
			return
		}
		if err != nil {
			if diag.Enabled(diag.Warn) {
				diag.Report(diag.Warn, "web", "environment", "the WebView2 environment could not be created",
					diag.String("error", err.Error()))
			}
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
		page.navigationStarting(args, true)
		return 0
	}), uintptr(unsafe.Pointer(&token)))
	comCall(view, viewAddFrameNavStarting, newHandler(func(_, args uintptr) uintptr {
		page.navigationStarting(args, false)
		return 0
	}), uintptr(unsafe.Pointer(&token)))
	comCall(view, viewAddNewWindowRequested, newHandler(func(_, args uintptr) uintptr {
		page.newWindowRequested(args)
		return 0
	}), uintptr(unsafe.Pointer(&token)))
	page.addDownloadHandler()
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
	script := platform.WebBridgeScript
	if page.config.BridgeMainFrameOnly {
		script = platform.WebBridgeMainFrameOnlyScript
	}
	comCall(view, viewAddScriptOnCreate, uintptr(unsafe.Pointer(wide(script))), 0)
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
	page.config.OnLoad(platform.WebLoad{Phase: phase, URL: page.appAddress(address)})
}

// appAddress is an address as the application knows it: the page's own scheme is served as
// https://<name>.viva-app.invalid/, and a decision or a load report names the address the
// application wrote.
func (page *windowsWeb) appAddress(address string) string {
	if scheme := page.config.Scheme; scheme != nil {
		if rest, ok := strings.CutPrefix(address, "https://"+scheme.Name+webSchemeTLD+"/"); ok {
			return scheme.Name + "://" + rest
		}
	}
	return address
}

// navigationStarting asks the application about a navigation before the request leaves. A
// refused navigation is cancelled, so the foreign document is never fetched.
func (page *windowsWeb) navigationStarting(args uintptr, mainFrame bool) {
	var address uintptr
	comCall(args, startingGetURI, uintptr(unsafe.Pointer(&address)))
	url := wideString(address)
	allowed := page.config.Navigate == nil
	if !allowed {
		var gesture, redirect int32
		comCall(args, startingGetUserGesture, uintptr(unsafe.Pointer(&gesture)))
		comCall(args, startingGetRedirected, uintptr(unsafe.Pointer(&redirect)))
		allowed = page.config.Navigate(platform.WebNavigation{
			URL:         page.appAddress(url),
			MainFrame:   mainFrame,
			UserGesture: gesture != 0 && redirect == 0,
		})
	}
	if !allowed {
		comCall(args, startingPutCancel, 1)
		return
	}
	// Started is reported only for a navigation the application allows: macOS and Linux start a
	// load after their policy decision, so a refused one must not leave a Started without a
	// Committed or Finished. loaded names the address as the application knows it there too.
	if mainFrame {
		page.loaded(platform.WebLoadStarted, url)
	}
}

// newWindowRequested answers a window.open or target=_blank. The runtime opens its own window
// unless the request says the application handled it, so an allowed address goes to the system's
// browser and a refused or nil NewWindow opens nothing.
func (page *windowsWeb) newWindowRequested(args uintptr) {
	var address uintptr
	comCall(args, newWindowGetURI, uintptr(unsafe.Pointer(&address)))
	url := page.appAddress(wideString(address))
	comCall(args, newWindowPutHandled, 1)
	if page.config.NewWindow == nil || !page.config.NewWindow(url) || !platform.OpenableURL(url) {
		return
	}
	_ = OpenURL(url)
}

// addDownloadHandler watches for downloads through ICoreWebView2_4, which is where DownloadStarting
// lives. A runtime older than that cannot report one before it starts, so a download there cannot
// be refused; it is said once rather than pretended away.
func (page *windowsWeb) addDownloadHandler() {
	var extended uintptr
	if err := hresult(comCall(page.view, 0, uintptr(unsafe.Pointer(&interfaceWebView4)), uintptr(unsafe.Pointer(&extended)))); err != nil || extended == 0 {
		if diag.Enabled(diag.Warn) {
			diag.Report(diag.Warn, "web", "download", "the WebView2 runtime cannot report downloads (ICoreWebView2_4 is missing); Download cannot refuse them")
		}
		return
	}
	defer comCall(extended, 2)
	var token int64
	comCall(extended, viewAddDownloadStarting, newHandler(func(_, args uintptr) uintptr {
		page.downloadStarting(args)
		return 0
	}), uintptr(unsafe.Pointer(&token)))
}

// downloadStarting asks the application about a download before the runtime writes it. The result
// file path is a free name in the user's downloads folder, and the runtime's own download UI is
// hidden either way.
func (page *windowsWeb) downloadStarting(args uintptr) {
	var operation uintptr
	if comCall(args, downloadArgsGetOperation, uintptr(unsafe.Pointer(&operation))) != 0 || operation == 0 {
		comCall(args, downloadArgsPutCancel, 1)
		return
	}
	var address uintptr
	comCall(operation, downloadGetURI, uintptr(unsafe.Pointer(&address)))
	url := page.appAddress(wideString(address))
	comCall(args, downloadArgsPutHandled, 1)
	if page.config.Download == nil || !page.config.Download(url) {
		comCall(args, downloadArgsPutCancel, 1)
		return
	}
	suggested := ""
	var result uintptr
	if comCall(args, downloadArgsGetResultPath, uintptr(unsafe.Pointer(&result))) == 0 && result != 0 {
		suggested = filepath.Base(wideString(result))
	}
	folder := downloadsFolder()
	if err := os.MkdirAll(folder, 0o700); err != nil {
		comCall(args, downloadArgsPutCancel, 1)
		return
	}
	destination := platform.FreeDownloadPath(folder, platform.DownloadFileName(suggested, url))
	comCall(args, downloadArgsPutResultPath, uintptr(unsafe.Pointer(wide(destination))))
}

// downloadsFolder is the user's downloads folder: the known folder, which may have been moved
// (OneDrive does), or the fallback when the system cannot name it.
func downloadsFolder() string {
	var path uintptr
	if status, _, _ := shGetKnownFolderPath.Call(uintptr(unsafe.Pointer(&folderIDDownloads)), 0, 0, uintptr(unsafe.Pointer(&path))); status == 0 && path != 0 {
		return wideString(path) // copies and frees the task allocator's string
	}
	return platform.DownloadsFolder()
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
	if err != nil || scheme == nil {
		return
	}
	path := strings.TrimPrefix(parsed.Path, "/")
	if _, rest, ok := strings.Cut(path, "/"); ok {
		path = rest
	} else {
		path = ""
	}
	if scheme.ServeAsync != nil {
		page.hold(args, path)
		return
	}
	schemeResponse, found := platform.ServeWebScheme(scheme, path)
	page.answer(environment, args, platform.AnswerWebResponse(schemeResponse, found))
}

// answer gives the runtime one response object for a request, now.
func (page *windowsWeb) answer(environment, args uintptr, answer platform.WebAnswer) {
	var stream uintptr
	if len(answer.Body) > 0 {
		stream, _, _ = createMemoryBlob.Call(uintptr(unsafe.Pointer(&answer.Body[0])), uintptr(len(answer.Body)))
	} else {
		stream, _, _ = createMemoryBlob.Call(0, 0)
	}
	var response uintptr
	comCall(environment, envCreateResponse, stream, uintptr(answer.Status),
		uintptr(unsafe.Pointer(wide(answer.Reason))), uintptr(unsafe.Pointer(wide(answer.HeaderBlock()))),
		uintptr(unsafe.Pointer(&response)))
	if response != 0 {
		comCall(args, resourceArgsPutResponse, response)
		comCall(response, 2)
	}
	if stream != 0 {
		comCall(stream, 2)
	}
}

// hold keeps the runtime waiting for an answer the application gives later, from any goroutine. The
// deferral keeps the request's arguments alive; the answer reaches the UI thread as a message to the
// page's window, which is where the runtime's objects may be touched.
func (page *windowsWeb) hold(args uintptr, path string) {
	var deferral uintptr
	comCall(args, resourceArgsGetDeferral, uintptr(unsafe.Pointer(&deferral)))
	if deferral == 0 {
		return // the runtime will not wait, so there is nothing to answer into
	}
	webAnswersMu.Lock()
	webAnswerNext++
	id := webAnswerNext
	webAnswers[id] = &webAnswer{page: page, args: args, deferral: deferral}
	window := page.window
	webAnswersMu.Unlock()
	page.config.Scheme.ServeAsync(platform.WebRequest{Path: path}, func(response platform.WebResponse) {
		webAnswersMu.Lock()
		held := webAnswers[id]
		if held == nil || held.answered {
			webAnswersMu.Unlock()
			return
		}
		held.response, held.answered = response, true
		webAnswersMu.Unlock()
		// A closed page's window is gone, and PostMessage to it fails without doing anything.
		postMessage.Call(window, wmWebAnswer, uintptr(id), 0)
	})
}

// answerSchemeRequest completes a held request on the UI thread.
func answerSchemeRequest(id uintptr) {
	webAnswersMu.Lock()
	entry := webAnswers[id]
	delete(webAnswers, id)
	webAnswersMu.Unlock()
	if entry == nil {
		return
	}
	if entry.page.closed || entry.page.environment == nil || entry.page.environment.object == 0 {
		comCall(entry.deferral, 2)
		return
	}
	entry.page.answer(entry.page.environment.object, entry.args, platform.AnswerWebResponse(entry.response, true))
	comCall(entry.deferral, deferralComplete)
	comCall(entry.deferral, 2)
}

// releaseAnswers lets go of the requests the runtime holds open for a page that is closing. An answer
// that still arrives afterwards finds no entry and is dropped.
func (page *windowsWeb) releaseAnswers() {
	webAnswersMu.Lock()
	var held []*webAnswer
	for id, entry := range webAnswers {
		if entry.page == page {
			held = append(held, entry)
			delete(webAnswers, id)
		}
	}
	webAnswersMu.Unlock()
	for _, entry := range held {
		comCall(entry.deferral, 2)
	}
}

// webHostProc handles the answer to an asynchronous scheme request, posted to the page's own window
// from the goroutine that produced it, and passes everything else to the system.
func webHostProc(window, kind, first, second uintptr) uintptr {
	if kind == wmWebAnswer {
		answerSchemeRequest(first)
		return 0
	}
	result, _, _ := defaultWindowProc.Call(window, kind, first, second)
	return result
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
	page.releaseAnswers()
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
	if page.environment != nil {
		page.environment.leave()
		page.environment = nil
	}
}

// frame takes what a frame says about the places of the pages and puts them there.
func (w *webHost) frame(views []platform.NativeView) {
	if views != nil {
		w.rects = w.rects[:0]
		for _, view := range views {
			view.Covers = append([]gfx.Rect(nil), view.Covers...)
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
	logicalRadius := min(max(page.config.TopCornerRadius, 0), view.Bounds.Width/2, view.Bounds.Height)
	if radius := int32(round(logicalRadius * scale)); radius > 0 {
		// 完整圆角区域与底部矩形取并集，只有顶部两角被裁剪。
		rounded, _, _ := createRoundRectRgn.Call(0, 0, uintptr(width+1), uintptr(height+1), uintptr(2*radius), uintptr(2*radius))
		bottom, _, _ := createRectRgn.Call(0, uintptr(radius), uintptr(width), uintptr(height))
		if rounded != 0 && bottom != 0 {
			combineRgn.Call(region, rounded, bottom, rgnOr)
		}
		if rounded != 0 {
			deleteObject.Call(rounded)
		}
		if bottom != 0 {
			deleteObject.Call(bottom)
		}
	}
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

func sameRects(a, b []gfx.Rect) bool {
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
