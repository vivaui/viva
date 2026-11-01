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
	"slices"
	"strings"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/ui"
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
	cuts       []ui.Rect
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
	cutsSet []ui.Rect
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
	script := C.CString(platform.WebBridgeScript)
	defer C.free(unsafe.Pointer(script))
	var failure *C.char
	native := C.viva_web_open(view, C.uint32_t(config.ID), C.uintptr_t(w.run.callbackHandle), url, html, base, script, scheme, &failure)
	if native == nil {
		message := "the page could not be created"
		if failure != nil {
			message = C.GoString(failure)
			C.free(unsafe.Pointer(failure))
		}
		return nil, fmt.Errorf("darwin: %s", message)
	}
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

//export vivaWebServe
func vivaWebServe(handle C.uintptr_t, id C.uint32_t, scheme, path *C.char, mime **C.char, data **C.uchar, length *C.size_t) C.int {
	page := pageOf(handle, id)
	if page == nil || page.config.Scheme == nil || page.config.Scheme.Serve == nil ||
		!strings.EqualFold(page.config.Scheme.Name, C.GoString(scheme)) {
		return 0
	}
	contentType, body, ok := page.config.Scheme.Serve(C.GoString(path))
	if !ok {
		return 0
	}
	*mime = C.CString(contentType)
	*data = (*C.uchar)(C.CBytes(body))
	*length = C.size_t(len(body))
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
