//go:build windows && !amd64

package windows

// putBounds sets the controller's bounds. A RECT is 16 bytes, which arm64 and x86 pass by value, in
// registers (arm64) or on the stack (x86), as four 32-bit fields.
func putBounds(controller uintptr, bounds rect) {
	low := uintptr(uint32(bounds.Left)) | uintptr(uint32(bounds.Top))<<32
	high := uintptr(uint32(bounds.Right)) | uintptr(uint32(bounds.Bottom))<<32
	comCall(controller, controllerPutBounds, low, high)
}
