//go:build linux && vivagtk

package linux

import (
	"unsafe"

	"github.com/vivaui/viva/internal/input"
	"github.com/vivaui/viva/ui"
)

// GdkEvent layouts for GTK 3 on LP64 Linux (amd64 and arm64). GDK hands the shell a pointer to the
// event; reading it through these structs avoids a call per field. gtk_events_linux_test.go checks
// the offsets against the documented layout.

type gdkEventType int32

const (
	gdkMotionNotify  gdkEventType = 3
	gdkButtonPress   gdkEventType = 4
	gdk2ButtonPress  gdkEventType = 5
	gdk3ButtonPress  gdkEventType = 6
	gdkButtonRelease gdkEventType = 7
	gdkKeyPress      gdkEventType = 8
	gdkKeyRelease    gdkEventType = 9
	gdkEnterNotify   gdkEventType = 10
	gdkLeaveNotify   gdkEventType = 11
	gdkFocusChange   gdkEventType = 12
	gdkConfigure     gdkEventType = 13
	gdkMap           gdkEventType = 14
	gdkUnmap         gdkEventType = 15
	gdkScroll        gdkEventType = 31
	gdkWindowState   gdkEventType = 32
)

// Event masks of gtk_widget_add_events.
const (
	gdkExposureMask      = 1 << 1
	gdkPointerMotionMask = 1 << 2
	gdkButtonPressMask   = 1 << 8
	gdkButtonReleaseMask = 1 << 9
	gdkKeyPressMask      = 1 << 10
	gdkKeyReleaseMask    = 1 << 11
	gdkEnterNotifyMask   = 1 << 12
	gdkLeaveNotifyMask   = 1 << 13
	gdkFocusChangeMask   = 1 << 14
	gdkStructureMask     = 1 << 15
	gdkScrollMask        = 1 << 21
	gdkSmoothScrollMask  = 1 << 23
)

// GdkModifierType bits.
const (
	gdkShiftMask   = 1 << 0
	gdkLockMask    = 1 << 1
	gdkControlMask = 1 << 2
	gdkMod1Mask    = 1 << 3 // Alt
	gdkSuperMask   = 1 << 26
	gdkMetaMask    = 1 << 28
	gdkButton1Mask = 1 << 8
)

// Window states of GdkEventWindowState.
const (
	gdkWindowStateIconified = 1 << 1
	gdkWindowStateMaximized = 1 << 2
	gdkWindowStateFocused   = 1 << 7
)

// Scroll directions of GdkEventScroll.
const (
	gdkScrollUp int32 = iota
	gdkScrollDown
	gdkScrollLeft
	gdkScrollRight
	gdkScrollSmooth
)

// Crossing modes (GdkCrossingMode) that mean a grab, not the pointer or focus going elsewhere.
const (
	gdkCrossingGrab   = 1
	gdkCrossingUngrab = 2
)

type gdkEventButton struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [3]byte
	Time      uint32
	X, Y      float64
	Axes      uintptr
	State     uint32
	Button    uint32
	Device    uintptr
	XRoot     float64
	YRoot     float64
}

type gdkEventMotion struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [3]byte
	Time      uint32
	X, Y      float64
	Axes      uintptr
	State     uint32
	IsHint    int16
	_         [2]byte
	Device    uintptr
	XRoot     float64
	YRoot     float64
}

type gdkEventScroll struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [3]byte
	Time      uint32
	X, Y      float64
	State     uint32
	Direction int32
	Device    uintptr
	XRoot     float64
	YRoot     float64
	DeltaX    float64
	DeltaY    float64
}

type gdkEventKey struct {
	Type            gdkEventType
	_               [4]byte
	Window          uintptr
	SendEvent       int8
	_               [3]byte
	Time            uint32
	State           uint32
	Keyval          uint32
	Length          int32
	_               [4]byte
	String          uintptr
	HardwareKeycode uint16
	Group           uint8
}

type gdkEventCrossing struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [7]byte
	Subwindow uintptr
	Time      uint32
	_         [4]byte
	X, Y      float64
	XRoot     float64
	YRoot     float64
	Mode      int32
	Detail    int32
	Focus     int32
	State     uint32
}

type gdkEventFocus struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [1]byte
	In        int16
}

type gdkEventConfigure struct {
	Type          gdkEventType
	_             [4]byte
	Window        uintptr
	SendEvent     int8
	_             [3]byte
	X, Y          int32
	Width, Height int32
}

type gdkEventWindowState struct {
	Type      gdkEventType
	_         [4]byte
	Window    uintptr
	SendEvent int8
	_         [3]byte
	Changed   uint32
	New       uint32
}

// eventType reads the type every GdkEvent starts with.
func eventType(event uintptr) gdkEventType { return *(*gdkEventType)(unsafePointer(event)) }

// unsafePointer turns an address C gave back into a pointer for reading it.
func unsafePointer(address uintptr) unsafe.Pointer { return unsafe.Add(unsafe.Pointer(nil), address) }

// unsafeSlice reads n bytes at an address C gave back.
func unsafeSlice(address uintptr, n int) []byte {
	return unsafe.Slice((*byte)(unsafePointer(address)), n)
}

// gdkModifiers translates GDK's modifier state.
func gdkModifiers(state uint32) input.Modifiers {
	var modifiers input.Modifiers
	if state&gdkShiftMask != 0 {
		modifiers |= input.ModifierShift
	}
	if state&gdkControlMask != 0 {
		modifiers |= input.ModifierControl
	}
	if state&gdkMod1Mask != 0 {
		modifiers |= input.ModifierAlt
	}
	if state&(gdkSuperMask|gdkMetaMask) != 0 {
		modifiers |= input.ModifierMeta
	}
	return modifiers
}

// gdkPointerButton maps GDK's button numbers (the X11 numbering, also used for Wayland's evdev
// buttons by GDK) to Viva's.
func gdkPointerButton(button uint32) ui.PointerButton {
	switch button {
	case 1:
		return ui.PointerButtonPrimary
	case 2:
		return ui.PointerButtonMiddle
	case 3:
		return ui.PointerButtonSecondary
	case 8:
		return ui.PointerButtonBack
	case 9:
		return ui.PointerButtonForward
	}
	return ui.PointerButtonNone
}
