//go:build darwin

package darwin

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit -framework QuartzCore
#include <stdlib.h>
#include "web_darwin.h"
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"sync"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// Web pages inside the window (ADR 0064): WKWebView, one per page, subviews of the window's Metal
// view and above its layer. A page gives way where Viva draws over it (platform.NativeCuts): those
// parts are masked off, so Viva's picture shows, and refuse the pointer, so Viva gets it. The
// engine is the system's; nothing is loaded until the first page opens.

// webState is the pages of one window.
type webState struct {
	run   *runState
	token unsafe.Pointer
	// pages by view id; rects are the places the application declared, rectangles and labels what
	// Viva draws, as the last frames left them.
	pages      map[uint32]*darwinWeb
	rects      []platform.NativeView
	rectangles []render.Rectangle
	labels     []platform.TextLabel
	cuts       []gfx.Rect
	scratch    []C.double
}

// darwinWeb is one page.
type darwinWeb struct {
	state  *webState
	id     uint32
	native unsafe.Pointer
	config platform.WebViewConfig
	closed bool
	// placed is where the page was last put, to leave AppKit alone when nothing changed.
	placed  platform.NativeView
	cutsSet []gfx.Rect
	shown   bool
}

func newWebState(run *runState, host *platform.WebHost, initial []platform.NativeView) *webState {
	w := &webState{run: run, pages: map[uint32]*darwinWeb{}}
	for _, view := range initial {
		view.Covers = slices.Clone(view.Covers)
		w.rects = append(w.rects, view)
	}
	host.Open = w.open
	return w
}

// open is WebHost.Open.
func (w *webState) open(config platform.WebViewConfig) (platform.WebView, error) {
	if config.Proxy != "" {
		return nil, platform.ErrWebViewUnsupported
	}
	if w.run.closed || w.token == nil {
		return nil, platform.ErrWebViewUnsupported
	}
	if w.pages[config.ID] != nil {
		return nil, fmt.Errorf("darwin: a web view with id %d is already open", config.ID)
	}
	view := C.viva_application_window_view(w.token)
	if view == nil {
		return nil, platform.ErrWebViewUnsupported
	}
	var url, html, base, scheme *C.char
	for _, field := range []struct {
		text string
		out  **C.char
	}{{config.URL, &url}, {config.HTML, &html}, {config.BaseURL, &base}} {
		*field.out = C.CString(field.text)
		defer C.free(unsafe.Pointer(*field.out))
	}
	if config.Scheme != nil {
		scheme = C.CString(config.Scheme.Name)
		defer C.free(unsafe.Pointer(scheme))
	}
	scriptText := platform.WebBridgeScript
	mainFrameOnly := C.int(0)
	if config.BridgeMainFrameOnly {
		scriptText = platform.WebBridgeMainFrameOnlyScript
		mainFrameOnly = 1
	}
	script := C.CString(scriptText)
	defer C.free(unsafe.Pointer(script))
	var ephemeral C.int
	if config.Ephemeral {
		ephemeral = 1
	}
	var failure *C.char
	native := C.viva_web_open(view, C.uint32_t(config.ID), C.uintptr_t(w.run.callbackHandle), url, html, base, script, mainFrameOnly, scheme, ephemeral, &failure)
	if native == nil {
		message := "the page could not be created"
		if failure != nil {
			message = C.GoString(failure)
			C.free(unsafe.Pointer(failure))
		}
		return nil, fmt.Errorf("darwin: %s", message)
	}
	C.viva_web_set_top_corner_radius(native, C.double(config.TopCornerRadius))
	page := &darwinWeb{state: w, id: config.ID, native: native, config: config}
	w.pages[config.ID] = page
	w.layout(true)
	return page, nil
}

func (page *darwinWeb) usable() bool { return !page.closed && !page.state.run.closed }

func (page *darwinWeb) Load(address string) {
	if page.usable() {
		text := C.CString(address)
		defer C.free(unsafe.Pointer(text))
		C.viva_web_load(page.native, text)
	}
}

func (page *darwinWeb) LoadHTML(html, base string) {
	if page.usable() {
		h, b := C.CString(html), C.CString(base)
		defer C.free(unsafe.Pointer(h))
		defer C.free(unsafe.Pointer(b))
		C.viva_web_load_html(page.native, h, b)
	}
}

func (page *darwinWeb) Reload() {
	if page.usable() {
		C.viva_web_reload(page.native)
	}
}

func (page *darwinWeb) Back() {
	if page.usable() {
		C.viva_web_back(page.native)
	}
}

func (page *darwinWeb) Forward() {
	if page.usable() {
		C.viva_web_forward(page.native)
	}
}

func (page *darwinWeb) Eval(script string) {
	if page.usable() {
		text := C.CString(script)
		defer C.free(unsafe.Pointer(text))
		C.viva_web_eval(page.native, text)
	}
}

func (page *darwinWeb) PostMessage(text string) {
	encoded, err := json.Marshal(text)
	if err != nil {
		return
	}
	page.Eval("window.viva.deliver(" + string(encoded) + ");")
}

func (page *darwinWeb) Focus() {
	if page.usable() {
		C.viva_web_focus(page.native)
	}
}

func (page *darwinWeb) Close() {
	if !page.usable() {
		return
	}
	page.closed = true
	delete(page.state.pages, page.id)
	C.viva_web_close(page.native)
}

// frame takes what a frame says about the pages and what Viva draws, and lays them out again when
// any of it changed.
func (w *webState) frame(frame platform.Frame) {
	changed := false
	if frame.NativeViews != nil {
		w.rects = w.rects[:0]
		for _, view := range frame.NativeViews {
			view.Covers = slices.Clone(view.Covers)
			w.rects = append(w.rects, view)
		}
		changed = true
	}
	if !frame.Update.Empty() {
		w.rectangles = frame.Rectangles
		changed = true
	}
	if frame.TextLabels != nil {
		w.labels = frame.TextLabels
		changed = true
	}
	if changed {
		w.layout(false)
	}
}

// layout puts every page where the application declared it, cut where Viva draws over it.
func (w *webState) layout(force bool) {
	for id, page := range w.pages {
		view, found := platform.NativeView{}, false
		for _, candidate := range w.rects {
			if candidate.ID == id {
				view, found = candidate, true
				break
			}
		}
		if !found {
			if page.shown || force {
				page.shown = false
				C.viva_web_place(page.native, 0, 0, 0, 0, 0, nil, 0, 0)
			}
			continue
		}
		w.cuts = platform.NativeCuts(view, w.rectangles, w.labels, w.cuts[:0])
		visible := len(w.cuts)
		w.cuts = platform.NativeCovers(view, w.cuts)
		if !force && page.shown && page.placed.Bounds == view.Bounds && slices.Equal(page.cutsSet, w.cuts) {
			continue
		}
		page.placed, page.cutsSet, page.shown = view, append(page.cutsSet[:0], w.cuts...), true
		w.scratch = w.scratch[:0]
		for _, cut := range w.cuts {
			w.scratch = append(w.scratch, C.double(cut.X), C.double(cut.Y), C.double(cut.Width), C.double(cut.Height))
		}
		var cuts *C.double
		if len(w.scratch) > 0 {
			cuts = &w.scratch[0]
		}
		b := view.Bounds
		C.viva_web_place(page.native, 1, C.double(b.X), C.double(b.Y), C.double(b.Width), C.double(b.Height), cuts, C.size_t(visible), C.size_t(len(w.cuts)))
	}
}

// closeAll forgets the pages of a window that is going away; its views take them with them.
func (w *webState) closeAll() {
	for _, page := range w.pages {
		page.closed = true
		// An answer still being made would otherwise message a task of a view that no longer exists.
		C.viva_web_cancel_scheme_tasks(page.native)
	}
	clear(w.pages)
}

func pageOf(handle C.uintptr_t, id C.uint32_t) *darwinWeb {
	state, ok := runStateFor(handle)
	if !ok || state.web == nil {
		return nil
	}
	return state.web.pages[uint32(id)]
}

//export vivaWebMessage
func vivaWebMessage(handle C.uintptr_t, id C.uint32_t, utf8 *C.char) {
	if page := pageOf(handle, id); page != nil && page.config.OnMessage != nil {
		page.config.OnMessage(C.GoString(utf8))
	}
}

//export vivaWebLoad
func vivaWebLoad(handle C.uintptr_t, id C.uint32_t, phase C.int, url *C.char) {
	page := pageOf(handle, id)
	if page == nil || page.config.OnLoad == nil {
		return
	}
	page.config.OnLoad(platform.WebLoad{Phase: platform.WebLoadPhase(phase), URL: C.GoString(url)})
}

// vivaWebNavigate asks the application about a navigation before WebKit makes it. It answers 1 to
// allow, which is what an application that set no Navigate wants.
//
//export vivaWebNavigate
func vivaWebNavigate(handle C.uintptr_t, id C.uint32_t, url *C.char, mainFrame, userGesture C.int) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.Navigate == nil {
		return 1
	}
	navigation := platform.WebNavigation{
		URL:         C.GoString(url),
		MainFrame:   mainFrame != 0,
		UserGesture: userGesture != 0,
	}
	if !page.config.Navigate(navigation) {
		return 0
	}
	return 1
}

// vivaWebNewWindow asks the application about a window.open or target=_blank. It answers 1 when the
// address was handed to the system's browser, and 0 otherwise; the page itself is never opened in
// the window.
//
//export vivaWebNewWindow
func vivaWebNewWindow(handle C.uintptr_t, id C.uint32_t, url *C.char) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.NewWindow == nil {
		return 0
	}
	address := C.GoString(url)
	if !page.config.NewWindow(address) || !platform.OpenableURL(address) {
		return 0
	}
	_ = OpenURL(address)
	return 1
}

// vivaWebDownload asks the application about a download before its body is written. It answers 1
// to allow and 0 otherwise, which is what an application that set no Download wants.
//
//export vivaWebDownload
func vivaWebDownload(handle C.uintptr_t, id C.uint32_t, url *C.char) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.Download == nil {
		return 0
	}
	if !page.config.Download(C.GoString(url)) {
		return 0
	}
	return 1
}

// vivaWebDownloadPath is where a download the application allowed is saved: a free name under
// folder, or under the user's downloads folder when the platform names none. The caller frees the
// result.
//
//export vivaWebDownloadPath
func vivaWebDownloadPath(folder, suggested, address *C.char) *C.char {
	directory := C.GoString(folder)
	if directory == "" {
		directory = platform.DownloadsFolder()
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return nil
	}
	name := platform.DownloadFileName(C.GoString(suggested), C.GoString(address))
	return C.CString(platform.FreeDownloadPath(directory, name))
}

//export vivaWebServe
func vivaWebServe(handle C.uintptr_t, id C.uint32_t, scheme, path *C.char, status *C.int, headers **C.char, data **C.uchar, length *C.size_t) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.Scheme == nil ||
		!strings.EqualFold(page.config.Scheme.Name, C.GoString(scheme)) {
		return 0
	}
	response, found := platform.ServeWebScheme(page.config.Scheme, C.GoString(path))
	answer := platform.AnswerWebResponse(response, found)
	*status = C.int(answer.Status)
	*headers = C.CString(answer.HeaderBlock())
	*data = (*C.uchar)(C.CBytes(answer.Body))
	*length = C.size_t(len(answer.Body))
	return 1
}

//export vivaWebServeAsync
func vivaWebServeAsync(handle C.uintptr_t, id C.uint32_t, scheme, path *C.char, request C.uint64_t) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.Scheme == nil || page.config.Scheme.ServeAsync == nil ||
		!strings.EqualFold(page.config.Scheme.Name, C.GoString(scheme)) {
		return 0
	}
	serve := page.config.Scheme.ServeAsync
	var answered sync.Once
	serve(platform.WebRequest{Path: C.GoString(path)}, func(response platform.WebResponse) {
		// A request has one answer: a second call has nothing left to answer into, and the bridge
		// frees the buffers of one whose page closed meanwhile.
		answered.Do(func() {
			answer := platform.AnswerWebResponse(response, true)
			C.viva_web_scheme_answer(request, C.int(answer.Status), C.CString(answer.HeaderBlock()),
				(*C.uchar)(C.CBytes(answer.Body)), C.size_t(len(answer.Body)))
		})
	})
	return 1
}

//export vivaWebReady
func vivaWebReady(handle C.uintptr_t) {
	state, ok := runStateFor(handle)
	if !ok || state.web == nil || state.webReady == nil {
		return
	}
	state.webReady()
}
