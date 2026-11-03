package windows

import (
	"fmt"
	"syscall"
	"unsafe"
)

const wmFullscreen = 0x8050

var fullscreenGetStyle = user.NewProc("GetWindowLongW")
var fullscreenSetStyle = user.NewProc("SetWindowLongW")
var getWindowPlacement = user.NewProc("GetWindowPlacement")
var setWindowPlacement = user.NewProc("SetWindowPlacement")
var fullscreenClearError = kernel.NewProc("SetLastError")

type windowPlacement struct {
	Length, Flags, ShowCommand uint32
	Minimum, Maximum           point
	Normal                     rect
}

// SetFullscreen queues an explicit state request on the owning window thread.
func (window *nativeWindow) SetFullscreen(fullscreen bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	value := uintptr(0)
	if fullscreen {
		value = 1
	}
	if ok, _, err := postMessage.Call(window.state.window, wmFullscreen, value, 0); ok == 0 {
		return fmt.Errorf("windows: request fullscreen: %w", err)
	}
	return nil
}

func (window *nativeWindow) Fullscreen() bool {
	return window != nil && !window.closed.Load() && window.state.fullscreen.Load()
}

func (state *windowState) setFullscreen(fullscreen bool) {
	if fullscreen == state.fullscreen.Load() {
		return
	}
	const styleIndex = ^uintptr(15) // GWL_STYLE = -16
	state.fullscreenChanging = true
	defer func() { state.fullscreenChanging = false }()
	if fullscreen {
		placement := windowPlacement{Length: uint32(unsafe.Sizeof(windowPlacement{}))}
		if ok, _, err := getWindowPlacement.Call(state.window, uintptr(unsafe.Pointer(&placement))); ok == 0 {
			state.fail(fmt.Errorf("windows: remember fullscreen placement: %w", err))
			return
		}
		monitor, _, _ := monitorFromWindow.Call(state.window, monitorNearest)
		info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
		if ok, _, err := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok == 0 {
			state.fail(fmt.Errorf("windows: get fullscreen monitor: %w", err))
			return
		}
		style, _, _ := fullscreenGetStyle.Call(state.window, styleIndex)
		state.fullscreenPlacement, state.fullscreenStyle = placement, uint32(style)
		state.fullscreen.Store(true)
		state.captionPress.end()
		if state.config.OnFullscreen != nil {
			state.config.OnFullscreen(true)
		}
		// Unzoom only after remembering WINDOWPLACEMENT, including its show state.
		if state.zoomed() || state.minimized {
			showWindow.Call(state.window, 9)   // SW_RESTORE
			if uint32(style)&0x10000000 == 0 { // preserve a hidden window's visibility
				showWindow.Call(state.window, 0) // SW_HIDE
			}
		}
		if err := state.setFullscreenStyle(uint32(style) &^ wsOverlappedWindow); err != nil {
			state.fail(err)
			return
		}
		bounds := info.Monitor
		if ok, _, err := setWindowPosition.Call(state.window, 0, uintptr(bounds.Left), uintptr(bounds.Top),
			uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x34); ok == 0 {
			state.fullscreen.Store(false)
			if state.config.OnFullscreen != nil {
				state.config.OnFullscreen(false)
			}
			if restoreErr := state.restoreFullscreenPlacement(); restoreErr != nil {
				state.fail(restoreErr)
			} else {
				state.fail(fmt.Errorf("windows: enter fullscreen: %w", err))
			}
			return
		}
	} else {
		state.fullscreen.Store(false)
		if state.config.OnFullscreen != nil {
			state.config.OnFullscreen(false)
		}
		if err := state.restoreFullscreenPlacement(); err != nil {
			state.fail(err)
			return
		}
	}
	state.captionPress = captionPress{}
	state.fullscreenChanging = false
	if !fullscreen {
		state.reportFrame()
	}
	if state.config.OnRedraw != nil {
		frame, err := state.config.OnRedraw()
		state.frame(frame, err)
	}
}

func (state *windowState) fitFullscreenMonitor() {
	monitor, _, _ := monitorFromWindow.Call(state.window, monitorNearest)
	info := monitorInfo{Size: uint32(unsafe.Sizeof(monitorInfo{}))}
	if ok, _, _ := getMonitorInfo.Call(monitor, uintptr(unsafe.Pointer(&info))); ok != 0 {
		bounds := info.Monitor
		if ok, _, err := setWindowPosition.Call(state.window, 0, uintptr(bounds.Left), uintptr(bounds.Top),
			uintptr(bounds.Right-bounds.Left), uintptr(bounds.Bottom-bounds.Top), 0x34); ok == 0 {
			state.fail(fmt.Errorf("windows: resize fullscreen monitor: %w", err))
		}
	}
}

func (state *windowState) setFullscreenStyle(style uint32) error {
	fullscreenClearError.Call(0)
	if previous, _, err := fullscreenSetStyle.Call(state.window, ^uintptr(15), uintptr(style)); previous == 0 && err != syscall.Errno(0) {
		return fmt.Errorf("windows: set fullscreen style: %w", err)
	}
	return nil
}

func (state *windowState) restoreFullscreenPlacement() error {
	if err := state.setFullscreenStyle(state.fullscreenStyle); err != nil {
		return err
	}
	if ok, _, err := setWindowPlacement.Call(state.window, uintptr(unsafe.Pointer(&state.fullscreenPlacement))); ok == 0 {
		return fmt.Errorf("windows: restore fullscreen placement: %w", err)
	}
	if ok, _, err := setWindowPosition.Call(state.window, 0, 0, 0, 0, 0, 0x37); ok == 0 { // FRAMECHANGED | NOMOVE | NOSIZE | NOZORDER | NOACTIVATE
		return fmt.Errorf("windows: restore fullscreen frame: %w", err)
	}
	return nil
}
