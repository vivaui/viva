//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// showTextMenu shows request as AppKit's menu at the press, waits for it to close,
// and runs the command chosen, presenting the frame that follows.
func showTextMenu(state *runState, view unsafe.Pointer, request platform.TextMenuRequest) error {
	count := len(request.Items)
	if count == 0 || request.Perform == nil {
		return nil
	}
	commands := make([]C.int, count)
	disabled := make([]C.int, count)
	titles := make([]*C.char, count)
	for index, item := range request.Items {
		commands[index] = C.int(item.Command)
		if item.Disabled {
			disabled[index] = 1
		}
		if item.Command != platform.TextMenuNone {
			titles[index] = C.CString(item.Title)
		}
	}
	defer func() {
		for _, title := range titles {
			if title != nil {
				C.free(unsafe.Pointer(title))
			}
		}
	}()
	chosen := platform.TextMenuCommand(C.viva_view_popup_text_menu(view, C.int(count), &commands[0],
		(**C.char)(unsafe.Pointer(&titles[0])), &disabled[0], C.double(request.X), C.double(request.Y)))
	if chosen == platform.TextMenuNone {
		return nil
	}
	frame, err := request.Perform(chosen)
	if err != nil {
		return err
	}
	return presentFrame(state, view, frame, false)
}
