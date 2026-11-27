package windows

import (
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

const (
	spiGetWheelScrollLines = 0x0068
	spiGetWheelScrollChars = 0x006c
	defaultWheelUnits      = 3
)

// wheelSettings is how far one notch of the wheel scrolls, as the user set it in
// Mouse settings: lines for the vertical wheel, characters for the horizontal one,
// or platform.WheelPageScroll for a page.
type wheelSettings struct{ lines, chars uint32 }

// readWheelSettings asks the system; a setting it does not give reads as the
// default of three.
func readWheelSettings() wheelSettings {
	settings := wheelSettings{lines: defaultWheelUnits, chars: defaultWheelUnits}
	if systemParametersInfo.Find() != nil {
		return settings
	}
	var value uint32
	if ok, _, _ := systemParametersInfo.Call(spiGetWheelScrollLines, 0, uintptr(unsafe.Pointer(&value)), 0); ok != 0 {
		settings.lines = value
	}
	if ok, _, _ := systemParametersInfo.Call(spiGetWheelScrollChars, 0, uintptr(unsafe.Pointer(&value)), 0); ok != 0 {
		settings.chars = value
	}
	return settings
}

// wheelEvent is the scroll a WM_MOUSEWHEEL or WM_MOUSEHWHEEL stands for, at the
// pointer. A positive wheel delta is away from the user: up, or right.
func (state *windowState) wheelEvent(horizontal bool, first, second uintptr) platform.ScrollEvent {
	position := point{X: int32(int16(second)), Y: int32(int16(second >> 16))}
	screenToClient.Call(state.window, uintptr(unsafe.Pointer(&position)))
	units, extent := state.wheel.lines, state.pageExtent(false)
	if horizontal {
		units, extent = state.wheel.chars, state.pageExtent(true)
	}
	wheelDelta := int16(first >> 16)
	pixels, precise := platform.WheelPixels(int(wheelDelta), units, extent)
	event := platform.ScrollEvent{
		Modifiers: modifiers(), X: float32(position.X) / state.scale, Y: float32(position.Y) / state.scale,
		Precise: precise,
	}
	if horizontal {
		event.DeltaX = pixels
	} else {
		event.DeltaY = -pixels
	}
	if units == platform.WheelPageScroll {
		// The system asks for whole screens, and extent is only the window's: the
		// region the pointer is over moves Pages of its own page. A notch away
		// from the user is up, as a negative DeltaY.
		pages := float32(wheelDelta) / platform.WheelNotch
		if !horizontal {
			pages = -pages
		}
		event.Pages = pages
	}
	return event
}

// pageExtent is the size of the window's client area along an axis in logical
// pixels, what a page of scrolling moves.
func (state *windowState) pageExtent(horizontal bool) float32 {
	var client rect
	getClientRect.Call(state.window, uintptr(unsafe.Pointer(&client)))
	if horizontal {
		return float32(client.Right) / state.scale
	}
	return float32(client.Bottom) / state.scale
}
