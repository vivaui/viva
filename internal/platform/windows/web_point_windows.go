package windows

var windowFromPoint = user.NewProc("WindowFromPoint")

// windowAtPoint is the window at a screen point. A POINT is 8 bytes, passed in one register on amd64 and arm64.
func windowAtPoint(at point) uintptr {
	window, _, _ := windowFromPoint.Call(uintptr(uint32(at.X)) | uintptr(uint32(at.Y))<<32)
	return window
}
