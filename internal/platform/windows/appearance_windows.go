package windows

import (
	"sync/atomic"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	// wmApplyAppearance is posted to a window to give its title bar the look in force.
	wmApplyAppearance = 32774

	dwmUseImmersiveDarkMode       = 20 // Windows 10 2004 and later, and 11
	dwmUseImmersiveDarkModeBefore = 19 // the same switch before it was documented

	spiGetHighContrast = 0x0042
	hcfHighContrastOn  = 0x00000001

	rdwFrame = 0x0400
)

// The look the application pinned for what the system draws: 0 follows the
// system, 1 is light, 2 is dark.
var appearancePin atomic.Int32

var redrawWindow = user.NewProc("RedrawWindow")

// SetAppearance pins the look of what the system draws for the application's
// windows, which is their title bar: light, dark, or (neither) the system's own.
func SetAppearance(light, dark bool) {
	mode := int32(0)
	switch {
	case light:
		mode = 1
	case dark:
		mode = 2
	}
	appearancePin.Store(mode)
	windowStates.Range(func(window, _ any) bool {
		postMessage.Call(window.(uintptr), wmApplyAppearance, 0, 0)
		return true
	})
}

// darkCaption reports whether title bars are drawn dark.
func darkCaption() bool {
	switch appearancePin.Load() {
	case 1:
		return false
	case 2:
		return true
	}
	dark, known := DarkAppearance()
	return dark && known
}

// applyTitlebarTheme gives a window whose title bar the system draws the light or
// dark look in force, and reports whether that changed the look it had. Windows
// draws that bar in the light look unless asked.
func (state *windowState) applyTitlebarTheme() (changed bool) {
	if state.window == 0 || state.config.WindowMode != platform.WindowTitled || dwmSetWindowAttribute.Find() != nil {
		return false
	}
	dark := darkCaption()
	if state.backdropActive {
		dark = state.backdropStyle.Dark
	}
	if state.titlebarThemed && state.titlebarDark == dark {
		return false
	}
	state.titlebarThemed, state.titlebarDark = true, dark
	value := int32(0)
	if dark {
		value = 1
	}
	status, _, _ := dwmSetWindowAttribute.Call(state.window, dwmUseImmersiveDarkMode, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
	if hresult(status) != nil {
		dwmSetWindowAttribute.Call(state.window, dwmUseImmersiveDarkModeBefore, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
	}
	return true
}

// settingsChanged follows a system setting that moved: the title bar is drawn
// again if its colors are no longer the ones in force. Most settings that move
// say nothing about it, and cost nothing.
func (state *windowState) settingsChanged() {
	state.backdropApplied = false
	state.backdropPolicyKnown = false
	state.applyBackdrop()
	if state.applyTitlebarTheme() && state.ready && redrawWindow.Find() == nil {
		redrawWindow.Call(state.window, 0, 0, rdwFrame)
	}
}

// HighContrast reports whether the user turned on a high-contrast theme, which
// asks applications to draw in the system's colors with strong borders.
func HighContrast() bool {
	if systemParametersInfo.Find() != nil {
		return false
	}
	var contrast struct {
		Size, Flags   uint32
		DefaultScheme *uint16
	}
	contrast.Size = uint32(unsafe.Sizeof(contrast))
	ok, _, _ := systemParametersInfo.Call(spiGetHighContrast, unsafe.Sizeof(contrast), uintptr(unsafe.Pointer(&contrast)), 0)
	return ok != 0 && contrast.Flags&hcfHighContrastOn != 0
}
