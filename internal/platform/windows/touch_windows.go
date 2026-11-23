package windows

import (
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// A finger on a touch screen reaches the window as WM_POINTER messages. A window that
// leaves them to the default procedure gets the finger as a mouse: one press at the
// fingertip, no second finger, no way to tell a tap from a click, so the gestures
// (pan, fling, long press) could not be read from it. The window takes a touch pointer
// itself and leaves the mouse and the pen to the default, which turns them into the
// mouse messages the window already reads.
const (
	wmPointerUpdate         = 0x0245
	wmPointerDown           = 0x0246
	wmPointerUp             = 0x0247
	wmPointerCaptureChanged = 0x024c
	pointerTypeTouch        = 2
	pointerFlagInContact    = 0x4
)

var (
	getPointerType = user.NewProc("GetPointerType")
	getPointerInfo = user.NewProc("GetPointerInfo")
)

// pointerInfo is POINTER_INFO.
type pointerInfo struct {
	Type, ID, Frame, Flags uint32
	SourceDevice, Target   uintptr
	Pixel, Himetric        point
	PixelRaw, HimetricRaw  point
	Time, History          uint32
	InputData              int32
	KeyStates              uint32
	PerformanceCount       uint64
	ButtonChange           uint32
}

// touchPointer reads a WM_POINTER message for a finger and reports whether it did: a
// pointer that is not a finger is left to the default procedure.
func (state *windowState) touchPointer(kind, first uintptr) bool {
	id := uint32(first & 0xffff)
	var pointerType uint32
	if ok, _, _ := getPointerType.Call(uintptr(id), uintptr(unsafe.Pointer(&pointerType))); ok == 0 || pointerType != pointerTypeTouch {
		return false
	}
	var eventKind platform.PointerEventKind
	switch kind {
	case wmPointerDown:
		eventKind = platform.PointerDown
	case wmPointerUp:
		eventKind = platform.PointerUp
	case wmPointerUpdate:
		eventKind = platform.PointerMove
	default: // the system took the finger away, as for a gesture of its own
		eventKind = platform.PointerCancel
	}
	event := platform.PointerEvent{Kind: eventKind, Device: gfx.PointerTouch, PointerID: int32(id), Modifiers: modifiers()}
	if eventKind != platform.PointerCancel {
		var info pointerInfo
		if ok, _, _ := getPointerInfo.Call(uintptr(id), uintptr(unsafe.Pointer(&info))); ok == 0 {
			return true
		}
		if eventKind == platform.PointerMove && info.Flags&pointerFlagInContact == 0 {
			return true // a finger hovering over a pen-capable digitizer is no touch
		}
		client := info.Pixel
		screenToClient.Call(state.window, uintptr(unsafe.Pointer(&client)))
		event.X, event.Y = float32(client.X)/state.scale, float32(client.Y)/state.scale
		event.ScreenX, event.ScreenY = float32(info.Pixel.X)/state.scale, float32(info.Pixel.Y)/state.scale
		if eventKind == platform.PointerDown || eventKind == platform.PointerUp {
			event.Button = gfx.PointerButtonPrimary
		}
	}
	if eventKind == platform.PointerDown && state.config.WindowMode != platform.WindowNotification {
		setFocus.Call(state.window)
	}
	if state.config.OnPointer != nil {
		state.pointerEvents++
		frame, err := state.config.OnPointer(event)
		state.frame(frame, err)
	}
	return true
}
