package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

type fullscreenControl interface {
	SetFullscreen(bool) error
	Fullscreen() bool
}

// SetFullscreen asks the native window to fill its monitor without window
// decorations. Leaving fullscreen restores its previous placement. Calls are
// safe from any goroutine. A nil error means the request was accepted, not that
// an asynchronous native transition has completed; OnFullscreen reports it.
// Unsupported hosts return an error wrapping ErrUnsupported.
func (window *Window) SetFullscreen(fullscreen bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	control, ok := window.control.(fullscreenControl)
	if !ok {
		return fmt.Errorf("%w: fullscreen windows", ErrUnsupported)
	}
	err := control.SetFullscreen(fullscreen)
	if errors.Is(err, platform.ErrFullscreenUnsupported) {
		return fmt.Errorf("%w: fullscreen windows", ErrUnsupported)
	}
	return err
}

// Fullscreen reports the fullscreen state last observed by the window's host.
// It is safe from any goroutine and is updated before OnFullscreen runs.
func (window *Window) Fullscreen() bool {
	if window == nil || window.closed.Load() {
		return false
	}
	return window.fullscreen.Load()
}
