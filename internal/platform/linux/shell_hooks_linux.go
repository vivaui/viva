package linux

import "sync/atomic"

// shellHooks is how a window shell that owns the display connection itself (the GTK shell) takes
// the desktop services that reach the display server: the clipboard, the primary selection and the
// cursor. The XCB backend answers them over its own connection; GTK has its own, and a Wayland
// session has no X selection at all. Nil means the XCB backend's.
var shellHooks atomic.Pointer[desktopHooks]

type desktopHooks struct {
	clipboardText     func() (string, error)
	setClipboardText  func(string) error
	hasClipboardImage func() bool
	clipboardImagePNG func() ([]byte, error)
	primaryText       func() (string, error)
	setPrimaryText    func(string) error
	setCursorShape    func(uint8) error
}
