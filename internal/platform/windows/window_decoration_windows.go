package windows

import (
	"errors"
	"fmt"
	"sync"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const wmChangeDecoration = 0x8056

var (
	decorationGetFocus  = user.NewProc("GetFocus")
	decorationGetActive = user.NewProc("GetActiveWindow")
	decorationGetMenu   = user.NewProc("GetMenu")
	decorationIsVisible = user.NewProc("IsWindowVisible")
)

type windowDecoration struct {
	mu      sync.Mutex
	closed  bool
	pending *decorationRequest
	// The owner thread alone reads and changes the style and placement state.
	changing bool
	removed  bool
	style    uint32
}

type decorationRequest struct {
	decorated bool
	done      func(error)
}

func (window *nativeWindow) SetDecorated(decorated bool, done func(error)) error {
	if window == nil || window.state == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	if done == nil {
		return fmt.Errorf("windows: window decoration completion callback is required")
	}
	state := window.state
	if state.config.WindowMode != platform.WindowTitled {
		return platform.ErrWindowDecorationUnsupported
	}
	d := &state.decoration
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	if d.pending != nil {
		return fmt.Errorf("windows: a decoration change is already pending")
	}
	d.pending = &decorationRequest{decorated: decorated, done: done}
	if posted, _, err := postMessage.Call(state.window, wmChangeDecoration, 0, 0); posted == 0 {
		d.pending = nil
		return fmt.Errorf("windows: post decoration change: %w", err)
	}
	return nil
}

func (state *windowState) applyDecoration() {
	d := &state.decoration
	d.mu.Lock()
	request := d.pending
	d.mu.Unlock()
	if request == nil || d.changing {
		return
	}
	err := state.changeDecoration(request.decorated)
	d.mu.Lock()
	if d.pending != request {
		d.mu.Unlock()
		return // destruction already owns this completion
	}
	d.pending = nil
	d.mu.Unlock()
	request.done(err)
}

func (d *windowDecoration) close() *decorationRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.closed = true
	request := d.pending
	d.pending = nil
	return request
}

func (state *windowState) changeDecoration(decorated bool) error {
	hwnd := state.window
	visible, _, _ := decorationIsVisible.Call(hwnd)
	if !state.ready || visible == 0 || state.iconic() || state.zoomed() ||
		state.fullscreen.Load() || state.fullscreenChanging || state.sizingMove {
		return fmt.Errorf("windows: decorations can change only on a visible normal window")
	}
	d := &state.decoration
	if decorated == !d.removed {
		return nil
	}
	before, err := readDecorationStyle(hwnd, ^uintptr(15)) // GWL_STYLE
	if err != nil {
		return err
	}
	extended, err := readDecorationStyle(hwnd, ^uintptr(19)) // GWL_EXSTYLE
	if err != nil {
		return err
	}
	client, ok := decorationClientRect(hwnd)
	var outer rect
	if read, _, _ := getWindowRect.Call(hwnd, uintptr(unsafe.Pointer(&outer))); !ok || read == 0 {
		return fmt.Errorf("windows: could not read the current window rectangle")
	}
	after := before &^ wsOverlappedWindow
	if decorated {
		after = (before &^ wsOverlappedWindow) | d.style
	}
	target, err := decorationOuterRect(hwnd, client, after, extended)
	if err != nil {
		return err
	}
	focus, _, _ := decorationGetFocus.Call()
	active, _, _ := decorationGetActive.Call()
	d.changing = true
	defer func() { d.changing = false }()
	if err := state.setFullscreenStyle(after); err != nil {
		return err
	}
	d.removed = !decorated
	err = positionDecorationFrame(hwnd, target)
	currentStyle, styleErr := readDecorationStyle(hwnd, ^uintptr(15))
	currentExtended, extendedErr := readDecorationStyle(hwnd, ^uintptr(19))
	current, ok := decorationClientRect(hwnd)
	newFocus, _, _ := decorationGetFocus.Call()
	newActive, _, _ := decorationGetActive.Call()
	newVisible, _, _ := decorationIsVisible.Call(hwnd)
	if state.window == hwnd && err == nil && styleErr == nil && extendedErr == nil &&
		ok && current == client && currentStyle == after && currentExtended&wsExTopmost == extended&wsExTopmost &&
		newFocus == focus && newActive == active && newVisible != 0 {
		if !decorated {
			d.style = before & wsOverlappedWindow
		}
		return nil
	}
	err = errors.Join(err, styleErr, extendedErr, fmt.Errorf("windows: decoration change did not preserve the window"))
	d.removed = decorated
	if state.window != hwnd {
		return err
	}
	if restoreErr := state.setFullscreenStyle(before); restoreErr != nil {
		return errors.Join(err, restoreErr)
	}
	return errors.Join(err, positionDecorationFrame(hwnd, outer))
}

func readDecorationStyle(hwnd, index uintptr) (uint32, error) {
	fullscreenClearError.Call(0)
	style, _, err := fullscreenGetStyle.Call(hwnd, index)
	if style == 0 && err != syscall.Errno(0) {
		return 0, fmt.Errorf("windows: get decoration style: %w", err)
	}
	return uint32(style), nil
}

func decorationClientRect(hwnd uintptr) (rect, bool) {
	var client rect
	var origin point
	if ok, _, _ := getClientRect.Call(hwnd, uintptr(unsafe.Pointer(&client))); ok == 0 {
		return rect{}, false
	}
	if ok, _, _ := clientToScreen.Call(hwnd, uintptr(unsafe.Pointer(&origin))); ok == 0 {
		return rect{}, false
	}
	return rect{Left: origin.X, Top: origin.Y, Right: origin.X + client.Right, Bottom: origin.Y + client.Bottom}, true
}

func decorationOuterRect(hwnd uintptr, client rect, style, extended uint32) (rect, error) {
	menu, _, _ := decorationGetMenu.Call(hwnd)
	if menu != 0 {
		return rect{}, platform.ErrWindowDecorationUnsupported
	}
	frame := rect{Right: client.Right - client.Left, Bottom: client.Bottom - client.Top}
	var dpi uintptr
	if getDPI.Find() == nil {
		dpi, _, _ = getDPI.Call(hwnd)
	}
	var ok uintptr
	var err error
	if dpi != 0 && adjustWindow.Find() == nil {
		ok, _, err = adjustWindow.Call(uintptr(unsafe.Pointer(&frame)), uintptr(style), 0, uintptr(extended), dpi)
	} else {
		ok, _, err = adjustWindowLegacy.Call(uintptr(unsafe.Pointer(&frame)), uintptr(style), 0, uintptr(extended))
	}
	if ok == 0 {
		return rect{}, fmt.Errorf("windows: calculate decorated window rectangle: %w", err)
	}
	frame.Left += client.Left
	frame.Right += client.Left
	frame.Top += client.Top
	frame.Bottom += client.Top
	return frame, nil
}

func positionDecorationFrame(hwnd uintptr, target rect) error {
	const noZOrder, noActivate, frameChanged, noOwnerZOrder = 0x4, 0x10, 0x20, 0x200
	if ok, _, err := setWindowPosition.Call(hwnd, 0, uintptr(target.Left), uintptr(target.Top),
		uintptr(target.Right-target.Left), uintptr(target.Bottom-target.Top), noZOrder|noActivate|frameChanged|noOwnerZOrder); ok == 0 {
		return fmt.Errorf("windows: apply decoration frame: %w", err)
	}
	return nil
}
