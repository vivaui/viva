package app

import "fmt"

type titleControl interface {
	SetTitle(string) error
}

// SetTitle changes the window's title: the text of its system title bar, the
// task switcher and the taskbar or Dock window list. A window with
// WindowCustomTitlebar keeps no visible system title bar, but the title is still
// what the system lists and what assistive technology reads. Calls are safe from
// any goroutine; the change is applied on the window's thread. Unsupported hosts
// return an error wrapping ErrUnsupported.
func (window *Window) SetTitle(title string) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	control, ok := window.control.(titleControl)
	if !ok {
		return fmt.Errorf("%w: window titles", ErrUnsupported)
	}
	return control.SetTitle(title)
}
