package windows

import (
	"fmt"

	"github.com/vivaui/viva/internal/platform"
)

const wmAlwaysOnTop = 0x8051

func (window *nativeWindow) SetAlwaysOnTop(enabled bool) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	switch window.state.config.WindowMode {
	case platform.WindowPanel, platform.WindowPreview, platform.WindowNotification:
		return platform.ErrAlwaysOnTopUnsupported
	}
	value := uintptr(0)
	if enabled {
		value = 1
	}
	if applied, _, _ := sendMessage.Call(window.state.window, wmAlwaysOnTop, value, 0); applied == 0 {
		return fmt.Errorf("windows: set always-on-top failed")
	}
	return nil
}

func (state *windowState) setAlwaysOnTop(enabled bool) bool {
	level := ^uintptr(1) // HWND_NOTOPMOST
	if enabled {
		level = ^uintptr(0) // HWND_TOPMOST
	}
	applied, _, _ := setWindowPosition.Call(state.window, level, 0, 0, 0, 0, 0x13)
	return applied != 0
}
