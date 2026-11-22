package windows

import (
	"fmt"
	"log"
	"sync"

	"github.com/vivaui/viva/internal/platform"
)

const (
	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

	smIconWidth = 11
)

var sendMessage = user.NewProc("SendMessageW")

const wmUpdateIcon = 0x8055 // after wmDragEnded

type iconRequest struct {
	mu   sync.Mutex
	icon platform.WindowIcon
}

func (window *nativeWindow) SetIcon(icon platform.WindowIcon) error {
	if window == nil || window.closed.Load() {
		return fmt.Errorf("windows: window is closed")
	}
	request := &window.state.pendingIcon
	request.mu.Lock()
	request.icon = icon
	request.mu.Unlock()
	if ok, _, err := postMessage.Call(window.state.window, wmUpdateIcon, 0, 0); ok == 0 {
		return fmt.Errorf("windows: request icon: %w", err)
	}
	return nil
}

func (state *windowState) applyIcon() {
	request := &state.pendingIcon
	request.mu.Lock()
	icon := request.icon
	request.icon = platform.WindowIcon{}
	request.mu.Unlock()
	if icon.Valid() && state.window != 0 {
		state.setIcons(icon)
	}
}

// setIcons gives the window the application's picture: the small one for its
// title bar and the big one for the taskbar button and Alt+Tab. Windows asks
// the window for these; the executable has no icon resource of its own.
func (state *windowState) setIcons(icon platform.WindowIcon) {
	if !icon.Valid() {
		return
	}
	if icon.ResourcePath != "" {
		if err := state.setTaskbarIcon(icon.ResourcePath); err != nil {
			log.Printf("viva: update taskbar icon: %v", err)
		}
	}
	for kind, metric := range [...]uintptr{smSmallIconWidth, smIconWidth} {
		size, _, _ := getSystemMetrics.Call(metric)
		if kind == iconBig {
			// The shell scales a big icon down for the dpi it draws at, never up.
			size *= 2
		}
		size = max(size, 16)
		// Windows draws both of these small: the title bar's icon and the
		// taskbar button. Crop the canvas margin away so the mark fills them.
		scaled := icon.TileMark(int(size))
		handle, err := newIconHandle(platform.StatusIcon{Size: scaled.Size, RGBA: scaled.RGBA})
		if err != nil {
			continue
		}
		previous := state.icons[kind]
		state.icons[kind] = handle
		sendMessage.Call(state.window, wmSetIcon, uintptr(kind), handle)
		if previous != 0 {
			destroyIcon.Call(previous)
		}
	}
}

// destroyIcons frees the icons setIcons made, once the window no longer shows them.
func (state *windowState) destroyIcons() {
	for index, handle := range state.icons {
		if handle != 0 {
			destroyIcon.Call(handle)
			state.icons[index] = 0
		}
	}
}
