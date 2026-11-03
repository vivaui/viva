//go:build linux

package linux

import "fmt"

func (window *nativeWindow) SetFullscreen(fullscreen bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("linux: window is closed")
	}
	command := uint32(commandExitFullscreen)
	if fullscreen {
		command = commandEnterFullscreen
	}
	window.state.win.postClientMessage(window.state.win.commandAtom, command)
	return nil
}

func (window *nativeWindow) Fullscreen() bool {
	return window != nil && !window.closed.Load() && window.state.win.fullscreen.Load()
}

func (win *window) observeFullscreen(fullscreen bool) {
	if win.fullscreen.Swap(fullscreen) == fullscreen {
		return
	}
	win.frame.pending = false
	if win.fullscreenHandler != nil {
		win.fullscreenHandler(fullscreen)
	}
}
