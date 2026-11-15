package darwin

/*
#include "native.h"
*/
import "C"

import "fmt"

// SetSize queues a content size request on the AppKit thread. The
// system may clamp the result to the window minimum or the screen.
func (window *nativeWindow) SetSize(width, height int) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("darwin: window is closed")
	}
	if width < 1 || height < 1 || width > 65535 || height > 65535 {
		return fmt.Errorf("darwin: invalid window size %dx%d", width, height)
	}
	C.viva_application_set_window_size(window.token, C.double(width), C.double(height))
	return nil
}
