package app

import (
	"errors"
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

type alwaysOnTopControl interface {
	SetAlwaysOnTop(bool) error
}

// SetAlwaysOnTop keeps this window above ordinary windows until disabled.
// It is safe from any goroutine and does not change the level of other windows.
// Hosts that cannot guarantee the request return ErrUnsupported.
//
// Every call is forwarded to the host, even when it repeats the last request,
// because the user or window manager can change the level behind the
// application's back (a Linux title-bar menu, for one). On Windows the call
// waits for the window thread to apply the level, so do not hold a lock the
// window's callbacks also take. On GTK/X11 the request is applied
// asynchronously and is best effort.
func (window *Window) SetAlwaysOnTop(enabled bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	window.topmostMu.Lock()
	defer window.topmostMu.Unlock()
	if window.closed.Load() {
		return fmt.Errorf("app: window is closed")
	}
	control, ok := window.control.(alwaysOnTopControl)
	if !ok {
		return fmt.Errorf("%w: always-on-top window", ErrUnsupported)
	}
	if err := control.SetAlwaysOnTop(enabled); err != nil {
		if errors.Is(err, platform.ErrAlwaysOnTopUnsupported) {
			return fmt.Errorf("%w: always-on-top window", ErrUnsupported)
		}
		return err
	}
	window.topmost.Store(enabled)
	return nil
}

// AlwaysOnTop reports the last successful level request for this window. It does
// not track changes the user makes outside the application.
func (window *Window) AlwaysOnTop() bool {
	return window != nil && !window.closed.Load() && window.topmost.Load()
}
