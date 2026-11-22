//go:build ios

package darwin

/*
#include "native.h"
*/
import "C"

import (
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// vivaTouchEvent is a finger of the window's view: kind is a PointerEventKind,
// id tells the fingers touching at once apart, and the coordinates are logical
// pixels in the view and on the screen.
//
//export vivaTouchEvent
func vivaTouchEvent(handle C.uintptr_t, view unsafe.Pointer, kind C.int, id C.int, x, y, screenX, screenY C.double) {
	state, ok := runStateFor(handle)
	if !ok {
		return
	}
	deliverPointer(state, view, PointerEvent{
		Kind:      platform.PointerEventKind(kind),
		Button:    gfx.PointerButtonPrimary,
		X:         float32(x),
		Y:         float32(y),
		ScreenX:   float32(screenX),
		ScreenY:   float32(screenY),
		Device:    gfx.PointerTouch,
		PointerID: int32(id),
	})
}

// vivaIOSSafeArea is how far the system covers each edge of the window's view, in points.
// The resize that follows carries it to the application.
//
//export vivaIOSSafeArea
func vivaIOSSafeArea(handle C.uintptr_t, view unsafe.Pointer, top, left, bottom, right C.double) {
	state, ok := runStateFor(handle)
	if !ok {
		return
	}
	state.safeArea = gfx.Insets{Top: float32(top), Left: float32(left), Bottom: float32(bottom), Right: float32(right)}
}
