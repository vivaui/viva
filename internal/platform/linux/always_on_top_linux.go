//go:build linux

package linux

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

func (window *nativeWindow) SetAlwaysOnTop(enabled bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("linux: window is closed")
	}
	state := window.state
	if state.config.WindowMode == platform.WindowPanel || state.config.WindowMode == platform.WindowPreview {
		return platform.ErrAlwaysOnTopUnsupported
	}
	win := state.win
	win.mu.Lock()
	defer win.mu.Unlock()
	if win.connection == 0 {
		return fmt.Errorf("linux: window is closed")
	}
	supported, err := win.supportsAbove()
	if err != nil {
		return err
	}
	if !supported {
		return platform.ErrAlwaysOnTopUnsupported
	}
	action := uint32(0)
	if enabled {
		action = 1
	}
	win.sendToRoot(win.frame.stateAtom, [5]uint32{action, win.frame.above, 0, sourceApp})
	if win.api.connectionHasError(win.connection) != 0 {
		return fmt.Errorf("linux: request always-on-top failed")
	}
	win.topmost.Store(enabled)
	win.topmostConfigured.Store(true)
	return nil
}
