package windows

import "unsafe"

// putBounds sets the controller's bounds. A RECT is passed by reference on x64.
func putBounds(controller uintptr, bounds rect) {
	comCall(controller, controllerPutBounds, uintptr(unsafe.Pointer(&bounds)))
}
