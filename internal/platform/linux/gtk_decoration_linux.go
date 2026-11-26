//go:build linux && vivagtk

package linux

import (
	"fmt"
	"sync"

	"github.com/vivaui/viva/internal/platform"
)

type gtkDecorationState struct {
	mu      sync.Mutex
	closed  bool
	pending *gtkDecorationRequest
	// changing is only read and written on the GTK thread.
	changing bool
}

type gtkDecorationRequest struct {
	decorated  bool
	done       func(error)
	task       uintptr
	source     uint32
	api        *xcbAPI
	conn       uintptr
	xid        uint32
	native     uintptr
	extents    uint32
	gdkExtents uintptr
	before     [4]uint32
	geometry   x11DecorationGeometry
	focused    bool
	rollback   bool
}

func (w *gtkWindow) SetDecorated(decorated bool, done func(error)) error {
	if w == nil || w.app == nil {
		return fmt.Errorf("linux: window is closed")
	}
	if done == nil {
		return fmt.Errorf("linux: window decoration completion callback is required")
	}
	if w.app.wayland || w.config.WindowMode != platform.WindowTitled {
		return platform.ErrWindowDecorationUnsupported
	}
	state := &w.decoration
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.closed {
		return fmt.Errorf("linux: window is closed")
	}
	if state.pending != nil {
		return fmt.Errorf("linux: a decoration change is already pending")
	}
	r := &gtkDecorationRequest{decorated: decorated, done: done}
	state.pending = r
	w.queueDecoration(r, 0, func() { w.beginDecoration(r) })
	return nil
}

// queueDecoration runs with the submission lock held. A tracked GLib source
// lets close discard queued work even when the last window exits the loop.
func (w *gtkWindow) queueDecoration(r *gtkDecorationRequest, delay uint32, run func()) {
	r.task = w.app.tasks.add(func() {
		state := &w.decoration
		state.mu.Lock()
		r.task, r.source = 0, 0
		pending := state.pending == r
		state.mu.Unlock()
		if pending {
			run()
		}
	})
	if delay == 0 {
		r.source = w.g.idleAdd(callbacks().task, r.task)
	} else {
		r.source = w.g.timeoutAdd(delay, callbacks().task, r.task)
	}
}

func (w *gtkWindow) beginDecoration(r *gtkDecorationRequest) {
	if w.g.widgetGetMapped(w.toplevel) == 0 || w.minimized || w.maximized || w.Fullscreen() {
		w.finishDecoration(r, fmt.Errorf("linux: decorations can change only on a mapped normal window"))
		return
	}
	r.native = w.g.widgetGetWindow(w.toplevel)
	var xid func(uintptr) uintptr
	if r.native == 0 || !w.g.lookup(&xid, "gdk_x11_window_get_xid") {
		w.finishDecoration(r, platform.ErrWindowDecorationUnsupported)
		return
	}
	r.xid = uint32(xid(r.native))
	api, err := loadXCB()
	if err != nil {
		w.finishDecoration(r, err)
		return
	}
	// A separate connection reads server state without sharing GDK's event queue.
	r.api = api
	r.conn = api.connect(w.g.gdkDisplayName(w.g.gdkDisplayDefault()), nil)
	if r.conn == 0 || api.connectionHasError(r.conn) != 0 || r.xid == 0 {
		w.finishDecoration(r, fmt.Errorf("linux: could not inspect the GTK X11 window"))
		return
	}
	r.extents, err = api.atom(r.conn, "_NET_FRAME_EXTENTS")
	if err != nil {
		w.finishDecoration(r, err)
		return
	}
	atomName := cString("_NET_FRAME_EXTENTS")
	r.gdkExtents = w.g.atomIntern(&atomName[0], 0)
	before, ok := r.frameExtents()
	current := w.g.windowGetDecorated(w.toplevel) != 0
	if !ok || current != (before != [4]uint32{}) {
		w.finishDecoration(r, fmt.Errorf("%w: the X11 window manager does not report the GTK frame", platform.ErrWindowDecorationUnsupported))
		return
	}
	if current == r.decorated {
		w.finishDecoration(r, nil)
		return
	}
	r.before = before
	r.geometry = api.decorationGeometry(r.conn, r.xid)
	r.focused = w.focused
	if r.geometry.width == 0 || r.geometry.height == 0 {
		w.finishDecoration(r, fmt.Errorf("linux: could not read the GTK client rectangle"))
		return
	}
	w.decoration.changing = true
	w.armDecorationDeadline(r)
	w.setDecorationHint(r.decorated)
	w.observeDecoration(r)
}

func (w *gtkWindow) setDecorationHint(decorated bool) {
	var hint int32
	if decorated {
		hint = 1
	}
	w.g.windowSetDecorated(w.toplevel, hint)
}

func (r *gtkDecorationRequest) frameExtents() ([4]uint32, bool) {
	var frame [4]uint32
	var present bool
	ok := r.api.readDecorationProperty(r.conn, r.xid, r.extents, xcbAtomCardinal, frame[:], &present)
	return frame, ok && present
}

func (w *gtkWindow) pendingDecorationObservation() *gtkDecorationRequest {
	if w.closed || !w.decoration.changing {
		return nil
	}
	w.decoration.mu.Lock()
	defer w.decoration.mu.Unlock()
	return w.decoration.pending
}

func (w *gtkWindow) observeDecorationEvent(property *gdkEventProperty) {
	if r := w.pendingDecorationObservation(); r != nil &&
		(property == nil || property.Window == r.native && property.Atom == r.gdkExtents) {
		w.observeDecoration(r)
	}
}

func (w *gtkWindow) observeDecoration(r *gtkDecorationRequest) bool {
	w.decoration.mu.Lock()
	pending := w.decoration.pending == r
	w.decoration.mu.Unlock()
	if !pending {
		return true
	}
	frame, ok := r.frameExtents()
	if ok && w.g.widgetGetMapped(w.toplevel) != 0 && w.g.widgetGetWindow(w.toplevel) == r.native &&
		r.api.decorationGeometry(r.conn, r.xid) == r.geometry {
		if r.rollback && frame == r.before {
			w.finishDecoration(r, fmt.Errorf("linux: decoration change was not confirmed; previous frame restored"))
			return true
		}
		if !r.rollback && r.decorated == (frame != [4]uint32{}) && w.focused == r.focused &&
			!w.minimized && !w.maximized && !w.Fullscreen() {
			w.finishDecoration(r, nil)
			return true
		}
	}
	return false
}

func (w *gtkWindow) armDecorationDeadline(r *gtkDecorationRequest) {
	w.decoration.mu.Lock()
	defer w.decoration.mu.Unlock()
	w.queueDecoration(r, uint32(decorationDeadline.Milliseconds()), func() { w.decorationTimedOut(r) })
}

func (w *gtkWindow) decorationTimedOut(r *gtkDecorationRequest) {
	if w.observeDecoration(r) {
		return
	}
	if r.rollback {
		w.finishDecoration(r, fmt.Errorf("linux: decoration change and restoration were not confirmed"))
		return
	}
	r.rollback = true
	w.armDecorationDeadline(r)
	w.setDecorationHint(!r.decorated)
	w.observeDecoration(r)
}

func (w *gtkWindow) closeDecoration() {
	state := &w.decoration
	state.mu.Lock()
	state.closed = true
	r := state.pending
	state.mu.Unlock()
	if r != nil {
		w.finishDecoration(r, fmt.Errorf("linux: window closed before its decoration change completed"))
	}
}

func (w *gtkWindow) finishDecoration(r *gtkDecorationRequest, err error) {
	state := &w.decoration
	state.mu.Lock()
	if state.pending != r {
		state.mu.Unlock()
		return
	}
	state.pending = nil
	if r.task != 0 {
		w.app.tasks.take(r.task)
		w.g.sourceRemove(r.source)
		r.task, r.source = 0, 0
	}
	state.mu.Unlock()
	state.changing = false
	if r.conn != 0 {
		r.api.disconnect(r.conn)
	}
	r.done(err)
}
