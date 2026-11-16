package windows

import (
	"math"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	// dwmBorderColor is DWMWA_BORDER_COLOR, Windows 11 21H2 (build 22000) and
	// later. The system draws a window's border in this color; without one it
	// blends the border with what is behind the window.
	dwmBorderColor = 34
	// dwmColorDefault is DWMWA_COLOR_DEFAULT: the system's own border color.
	dwmColorDefault = 0xFFFFFFFF
)

// colorRef is a color as the COLORREF DwmSetWindowAttribute takes, a
// 0x00BBGGRR word. Components outside 0..1 clamp to the nearest byte.
func colorRef(color platform.Color) uint32 {
	channel := func(value float64) uint32 {
		return uint32(math.Round(min(max(value, 0), 1) * 255))
	}
	return channel(color.Red) | channel(color.Green)<<8 | channel(color.Blue)<<16
}

// applyBorderColor gives a custom-titlebar window's system border the color of
// the window's own edge. A window that extends its frame to keep the shadow
// gets a border that the system composites over the desktop, not over the
// window, so it is translucent: the desktop shows through a 1 DIP ring around
// the window at 100% scale and wider at higher scale, which reads as a
// transparent edge in light mode. An opaque color the window is painted with
// makes the border continuous. Material-backed windows and ones that name no
// opaque color keep the system's border.
func (state *windowState) applyBorderColor() {
	if state.window == 0 {
		return
	}
	value, colored := windowBorderColor(state.config, state.customFrame(), state.backdropActive)
	if !colored && !state.borderColored {
		return
	}
	if state.borderColored == colored && state.borderColor == value {
		return
	}
	state.borderColored, state.borderColor = colored, value
	dwmSetWindowAttribute.Call(state.window, dwmBorderColor, uintptr(unsafe.Pointer(&value)), unsafe.Sizeof(value))
}

// windowBorderColor is the border color a window asks for, and whether it asks
// for one at all: a custom-titlebar window that is not backed by a material and
// whose edge color is opaque.
func windowBorderColor(config platform.Config, customFrame, backdropActive bool) (uint32, bool) {
	if !customFrame || backdropActive {
		return dwmColorDefault, false
	}
	color := config.ClearColor
	if config.WindowBorderColor != nil {
		color = config.WindowBorderColor()
	}
	if color.Alpha < 1 {
		return dwmColorDefault, false
	}
	return colorRef(color), true
}
