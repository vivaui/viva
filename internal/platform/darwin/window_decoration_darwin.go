//go:build !ios

package darwin

/*
#include "native.h"
*/
import "C"

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

func (window *nativeWindow) SetDecorated(decorated bool, done func(error)) error {
	if window == nil {
		return fmt.Errorf("darwin: window is closed")
	}
	if done == nil {
		return fmt.Errorf("darwin: window decoration completion callback is required")
	}
	if window.mode != WindowTitled && window.mode != WindowCustomTitlebar {
		return platform.ErrWindowDecorationUnsupported
	}
	window.decorationMu.Lock()
	if window.closed.Load() {
		window.decorationMu.Unlock()
		return fmt.Errorf("darwin: window is closed")
	}
	if window.decoration != nil {
		window.decorationMu.Unlock()
		return fmt.Errorf("darwin: a decoration change is already pending")
	}
	request := &decorationRequest{done: done}
	window.decoration = request
	window.decorationMu.Unlock()
	Post(func() {
		if window.closed.Load() {
			window.finishDecoration(request, fmt.Errorf("darwin: window is closed"))
			return
		}
		var err error
		switch C.viva_application_change_decoration(window.token, C.int(boolInt(decorated))) {
		case 1:
		case -1:
			err = platform.ErrWindowDecorationUnsupported
		case -2:
			err = fmt.Errorf("darwin: AppKit did not preserve the window while changing its decorations")
		default:
			err = fmt.Errorf("darwin: window is not available for a decoration change")
		}
		window.finishDecoration(request, err)
	})
	return nil
}
