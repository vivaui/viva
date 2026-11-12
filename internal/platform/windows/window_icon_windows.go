package windows

import "github.com/vivaui/viva/internal/platform"

const (
	wmSetIcon = 0x0080
	iconSmall = 0
	iconBig   = 1

	smIconWidth = 11
)

var sendMessage = user.NewProc("SendMessageW")

// setIcons gives the window the application's picture: the small one for its
// title bar and the big one for the taskbar button and Alt+Tab. Windows asks
// the window for these; the executable has no icon resource of its own.
func (state *windowState) setIcons(icon platform.WindowIcon) {
	if !icon.Valid() {
		return
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
		state.icons[kind] = handle
		sendMessage.Call(state.window, wmSetIcon, uintptr(kind), handle)
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
