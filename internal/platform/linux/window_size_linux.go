package linux

import "fmt"

// SetSize posts a logical content-size request to the X11 event loop. The
// window manager's later ConfigureNotify reports the size that was granted.
func (window *nativeWindow) SetSize(width, height int) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("linux: window is closed")
	}
	if width < 1 || height < 1 || width > 65535 || height > 65535 {
		return fmt.Errorf("linux: invalid window size %dx%d", width, height)
	}
	window.state.win.postClientSize(uint32(width), uint32(height))
	return nil
}
