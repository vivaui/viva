package linux

import (
	"encoding/binary"
	"unsafe"

	"github.com/ebitengine/purego"
)

// workArea is the part of the desktop the window manager leaves to windows
// (_NET_WORKAREA of the current desktop), false when it does not say.
func (win *window) workArea() (pixelRect, bool) {
	tracker := win.tracker
	if tracker == nil || win.api == nil || win.connection == 0 {
		return pixelRect{}, false
	}
	areaAtom, err := win.api.atom(win.connection, "_NET_WORKAREA")
	if err != nil {
		return pixelRect{}, false
	}
	blob, ok := tracker.property(win.screen.Root, areaAtom, xcbAtomCardinal)
	if !ok || len(blob) < 16 {
		return pixelRect{}, false
	}
	desktop := 0
	if currentAtom, err := win.api.atom(win.connection, "_NET_CURRENT_DESKTOP"); err == nil {
		if current, ok := tracker.property(win.screen.Root, currentAtom, xcbAtomCardinal); ok && len(current) >= 4 {
			desktop = int(binary.LittleEndian.Uint32(current))
		}
	}
	if (desktop+1)*16 > len(blob) {
		desktop = 0
	}
	field := func(index int) int { return int(binary.LittleEndian.Uint32(blob[desktop*16+index*4:])) }
	area := pixelRect{field(0), field(1), field(2), field(3)}
	return area, !area.empty()
}

type queryPointerReply struct {
	ResponseType uint8
	SameScreen   uint8
	Sequence     uint16
	Length       uint32
	Root, Child  uint32
	RootX, RootY int16
	WinX, WinY   int16
	Mask         uint16
	Pad          [2]byte
}

// pointerPosition is where the pointer is on the screen, false when the server
// does not say.
func (win *window) pointerPosition() (x, y int, ok bool) {
	if win.api == nil || win.connection == 0 {
		return 0, 0, false
	}
	var query func(uintptr, uint32) xcbVoidCookie
	var queryReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) *queryPointerReply
	purego.RegisterLibFunc(&query, win.api.library, "xcb_query_pointer")
	purego.RegisterLibFunc(&queryReply, win.api.library, "xcb_query_pointer_reply")
	var nativeError unsafe.Pointer
	reply := queryReply(win.connection, query(win.connection, win.screen.Root), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return 0, 0, false
	}
	defer win.api.free(unsafe.Pointer(reply))
	return int(reply.RootX), int(reply.RootY), true
}

type queryKeymapReply struct {
	ResponseType uint8
	Pad          uint8
	Sequence     uint16
	Length       uint32
	Keys         [32]byte
}

// keysDown is the server's bitmap of keycodes that are down, false when it
// does not say. One bit per keycode, in keycode order.
func (win *window) keysDown() ([32]byte, bool) {
	var keys [32]byte
	if win.api == nil || win.connection == 0 {
		return keys, false
	}
	var query func(uintptr) xcbVoidCookie
	var queryReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) *queryKeymapReply
	purego.RegisterLibFunc(&query, win.api.library, "xcb_query_keymap")
	purego.RegisterLibFunc(&queryReply, win.api.library, "xcb_query_keymap_reply")
	var nativeError unsafe.Pointer
	reply := queryReply(win.connection, query(win.connection), &nativeError)
	if nativeError != nil {
		win.api.free(nativeError)
	}
	if reply == nil {
		return keys, false
	}
	defer win.api.free(unsafe.Pointer(reply))
	return reply.Keys, true
}

// placementMonitors are the monitors the scale tracker last read.
func (win *window) placementMonitors() []monitorGeometry {
	if win.tracker == nil {
		return nil
	}
	return win.tracker.monitors
}
