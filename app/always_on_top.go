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
	if window.topmost.Load() == enabled {
		return nil
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

// AlwaysOnTop reports the last successful level request for this window.
func (window *Window) AlwaysOnTop() bool {
	return window != nil && !window.closed.Load() && window.topmost.Load()
}
