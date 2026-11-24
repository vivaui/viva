//go:build !android

package linux

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const decorationDeadline = 750 * time.Millisecond

type x11DecorationHints struct {
	present bool
	words   [5]uint32
}

type x11DecorationGeometry struct {
	x, y          int32
	width, height uint32
}

type x11DecorationChange struct {
	decorated      bool
	before, after  x11DecorationHints
	oldFrame       [4]uint32
	geometry       x11DecorationGeometry
	motif, extents uint32
	done           func(error)
	timer          *time.Timer
	rollingBack    bool
}

type x11DecorationRequest struct {
	done func(error)
}

func (window *nativeWindow) SetDecorated(decorated bool, done func(error)) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("linux: window is closed")
	}
	if done == nil {
		return fmt.Errorf("linux: window decoration completion callback is required")
	}
	state := window.state
	if state.config.WindowMode != platform.WindowTitled {
		return platform.ErrWindowDecorationUnsupported
	}
	win := state.win
	request := &x11DecorationRequest{done: done}
	win.mu.Lock()
	if win.connection == 0 || window.closed.Load() {
		win.mu.Unlock()
		return fmt.Errorf("linux: window is closed")
	}
	if win.decorationRequest != nil {
		win.mu.Unlock()
		return fmt.Errorf("linux: a decoration change is already pending")
	}
	win.decorationRequest = request
	win.mu.Unlock()
	state.application.Post(func() {
		win.mu.Lock()
		pending := win.decorationRequest == request
		win.mu.Unlock()
		if pending {
			state.requestDecoration(decorated, func(err error) { win.finishDecorationRequest(request, err) })
		}
	})
	return nil
}

// On close, request is nil and any accepted submission must complete, even
// if the window loop closed before taking its queued work.
func (win *window) finishDecorationRequest(request *x11DecorationRequest, err error) {
	win.mu.Lock()
	pending := win.decorationRequest
	if request != nil && request != pending {
		win.mu.Unlock()
		return
	}
	win.decorationRequest = nil
	win.mu.Unlock()
	if pending != nil {
		pending.done(err)
	}
}

// requestDecoration runs on the window thread. Completion reports a matching
// frame observed from the window manager, not merely a queued X11 request.
func (state *linuxWindowState) requestDecoration(decorated bool, done func(error)) {
	win := state.win
	if state.hidden || !state.exposed || win.minimized || win.frame.maximized || win.fullscreen.Load() ||
		state.config.WindowMode != platform.WindowTitled {
		done(fmt.Errorf("linux: decorations can change only on a mapped normal window"))
		return
	}
	if state.decorationChange != nil {
		done(fmt.Errorf("linux: a decoration change is already pending"))
		return
	}
	extents, err := win.api.atom(win.connection, "_NET_FRAME_EXTENTS")
	if err != nil {
		done(err)
		return
	}
	frame, ok := state.decorationExtents(extents)
	if !ok {
		done(fmt.Errorf("%w: the X11 window manager does not report frame extents", platform.ErrWindowDecorationUnsupported))
		return
	}
	if decorated == (frame != [4]uint32{}) {
		done(nil)
		return
	}
	motif, err := win.api.atom(win.connection, "_MOTIF_WM_HINTS")
	if err != nil {
		done(err)
		return
	}
	var before x11DecorationHints
	if !win.readDecorationProperty(motif, motif, before.words[:], &before.present) {
		done(fmt.Errorf("linux: could not read existing window decoration hints"))
		return
	}
	after := before
	if decorated {
		if state.normalDecoration == nil {
			done(fmt.Errorf("linux: no original window decorations to restore"))
			return
		}
		after = *state.normalDecoration
	} else {
		after.present = true
		after.words[0] |= 2 // MWM_HINTS_DECORATIONS
		after.words[2] = 0
	}
	geometry := state.decorationGeometry()
	if geometry.width == 0 || geometry.height == 0 {
		done(fmt.Errorf("linux: could not read the client rectangle"))
		return
	}
	change := &x11DecorationChange{decorated: decorated, before: before, after: after, oldFrame: frame,
		geometry: geometry, motif: motif, extents: extents, done: done}
	state.decorationChange = change
	win.requestDecorationHints(motif, after)
	state.armDecorationDeadline(change)
}

func (state *linuxWindowState) decorationGeometry() x11DecorationGeometry {
	win := state.win
	geometryCookie := win.api.getGeometry(win.connection, win.id)
	originCookie := win.api.translateCoordinates(win.connection, win.id, win.screen.Root, 0, 0)
	geometry := win.api.getGeometryReply(win.connection, geometryCookie, nil)
	origin := win.api.translateCoordinatesReply(win.connection, originCookie, nil)
	if geometry != nil {
		defer win.api.free(unsafe.Pointer(geometry))
	}
	if origin != nil {
		defer win.api.free(unsafe.Pointer(origin))
	}
	if geometry == nil || origin == nil {
		return x11DecorationGeometry{}
	}
	return x11DecorationGeometry{x: int32(origin.DstX), y: int32(origin.DstY),
		width: uint32(geometry.Width), height: uint32(geometry.Height)}
}

func (state *linuxWindowState) decorationExtents(atom uint32) ([4]uint32, bool) {
	var frame [4]uint32
	var present bool
	ok := state.win.readDecorationProperty(atom, xcbAtomCardinal, frame[:], &present)
	return frame, ok && present
}

func (win *window) readDecorationProperty(atom, kind uint32, words []uint32, present *bool) bool {
	cookie := win.api.getProperty(win.connection, 0, win.id, atom, kind, 0, uint32(len(words)))
	var nativeError unsafe.Pointer
	reply := win.api.getPropertyReply(win.connection, cookie, &nativeError)
	if reply != nil {
		defer win.api.free(unsafe.Pointer(reply))
	}
	if nativeError != nil {
		win.api.free(nativeError)
		return false
	}
	if reply == nil {
		return false
	}
	*present = reply.Type != 0
	if !*present {
		return true
	}
	if reply.Type != kind || reply.Format != 32 || reply.BytesAfter != 0 || reply.ValueLength != uint32(len(words)) {
		return false
	}
	copy(words, unsafe.Slice((*uint32)(win.api.getPropertyValue(reply)), len(words)))
	return true
}

func (win *window) writeDecorationHints(atom uint32, hints x11DecorationHints) {
	if hints.present {
		win.api.changeProperty(win.connection, xcbPropertyReplace, win.id, atom, atom, 32,
			uint32(len(hints.words)), unsafe.Pointer(&hints.words[0]))
	} else {
		win.api.deleteProperty(win.connection, win.id, atom)
	}
	win.api.flush(win.connection)
}

func (win *window) requestDecorationHints(atom uint32, hints x11DecorationHints) {
	// Xfwm does not restore decorations in response to a deleted property.
	// Explicitly restore them, then remove the temporary hint once observed.
	if !hints.present {
		hints = x11DecorationHints{present: true, words: [5]uint32{2, 0, 1, 0, 0}}
	}
	win.writeDecorationHints(atom, hints)
}

func (state *linuxWindowState) observeDecorationChange() {
	change := state.decorationChange
	if change == nil {
		return
	}
	frame, ok := state.decorationExtents(change.extents)
	if !ok || state.decorationGeometry() != change.geometry {
		return
	}
	if change.rollingBack {
		if frame == change.oldFrame {
			if !change.before.present {
				state.win.writeDecorationHints(change.motif, change.before)
			}
			state.finishDecorationChange(fmt.Errorf("linux: decoration change was not confirmed; previous frame restored"))
		}
		return
	}
	if change.decorated == (frame != [4]uint32{}) {
		if change.decorated {
			if !change.after.present {
				state.win.writeDecorationHints(change.motif, change.after)
			}
			state.normalDecoration = nil
		} else {
			before := change.before
			state.normalDecoration = &before
		}
		state.finishDecorationChange(nil)
	}
}

func (state *linuxWindowState) armDecorationDeadline(change *x11DecorationChange) {
	change.timer = time.AfterFunc(decorationDeadline, func() {
		state.application.Post(func() {
			if state.decorationChange != change {
				return
			}
			if change.rollingBack {
				state.finishDecorationChange(fmt.Errorf("linux: decoration change and restoration were not confirmed"))
				return
			}
			change.rollingBack = true
			state.win.requestDecorationHints(change.motif, change.before)
			state.armDecorationDeadline(change)
			state.observeDecorationChange()
		})
	})
}

func (state *linuxWindowState) finishDecorationChange(err error) {
	if change := state.decorationChange; change != nil {
		state.decorationChange = nil
		change.timer.Stop()
		change.done(err)
	}
}
