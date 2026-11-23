//go:build !android

package linux

import (
	"encoding/binary"
	"unsafe"

	"github.com/ebitengine/purego"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// A finger on a touch screen reaches an X11 window through the X Input extension, version
// 2.2 or later. Without it the server turns a finger into the pointer: one press at the
// fingertip, no second finger, and no telling a tap from a click, so the gestures (pan,
// fling, long press) could not be read from it. A window that selects the touch events
// gets the finger as touch events and none of that emulated pointer. The extension and
// its library are optional: a machine without them keeps the finger as a pointer.
const (
	xcbGenericEventType = 35 // XCB_GE_GENERIC: the events of extensions, which are more than 32 bytes
	xiAllMasterDevices  = 1
	xiTouchBegin        = 18
	xiTouchUpdate       = 19
	xiTouchEnd          = 20
	xiTouchMask         = 1<<xiTouchBegin | 1<<xiTouchUpdate | 1<<xiTouchEnd
	xiTouchPendingEnd   = 1 << 16 // flags of a touch end the server cancelled: a gesture of the shell's took it
	// The offsets in the event as libxcb returns it: libxcb puts the 32-bit full sequence
	// number after the first 32 bytes of a generic event (xcb_input_touch_begin_event_t's
	// full_sequence), so what the wire has from root_x on is four bytes further.
	xiEventOffsetTime    = 12
	xiEventOffsetDetail  = 16
	xiEventOffsetRootX   = 36
	xiEventOffsetEventX  = 44
	xiEventOffsetFlags   = 60
	xiEventOffsetMods    = 76 // the effective modifiers, after base, latched and locked
	xiEventMinimumLength = 84
)

// xinputTouch is the touch events of one window.
type xinputTouch struct {
	library uintptr
	opcode  uint8 // the extension's major opcode, which its events carry
}

type xinputVersionReply struct {
	ResponseType uint8
	Padding      uint8
	Sequence     uint16
	Length       uint32
	Major, Minor uint16
	Pad          [20]byte
}

// loadXInputTouch opens libxcb-xinput, checks the server speaks version 2.2 and selects
// the touch events of the window. A request for an extension the server lacks would shut
// the connection down, so the extension is looked up first. It returns nil where any of
// that is missing.
func loadXInputTouch(api *xcbAPI, connection uintptr, window uint32) *xinputTouch {
	library, err := purego.Dlopen("libxcb-xinput.so.0", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	fail := func() *xinputTouch {
		purego.Dlclose(library)
		return nil
	}
	identity, err := purego.Dlsym(library, "xcb_input_id")
	if err != nil || identity == 0 {
		return fail()
	}
	var extensionData func(uintptr, uintptr) *xcbExtensionReply
	purego.RegisterLibFunc(&extensionData, api.library, "xcb_get_extension_data")
	present := extensionData(connection, identity)
	if present == nil || present.Present == 0 {
		return fail()
	}
	opcode := present.MajorOpcode
	var queryVersion func(uintptr, uint16, uint16) xcbVoidCookie
	var versionReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) *xinputVersionReply
	var selectEvents func(uintptr, uint32, uint16, unsafe.Pointer) xcbVoidCookie
	purego.RegisterLibFunc(&queryVersion, library, "xcb_input_xi_query_version")
	purego.RegisterLibFunc(&versionReply, library, "xcb_input_xi_query_version_reply")
	purego.RegisterLibFunc(&selectEvents, library, "xcb_input_xi_select_events")
	var nativeError unsafe.Pointer
	version := versionReply(connection, queryVersion(connection, 2, 2), &nativeError)
	if nativeError != nil {
		api.free(nativeError)
	}
	if version == nil {
		return fail()
	}
	supported := version.Major > 2 || version.Major == 2 && version.Minor >= 2
	api.free(unsafe.Pointer(version))
	if !supported {
		return fail()
	}
	// xcb_input_event_mask_t: the device, the mask's length in words, and the mask.
	var mask [8]byte
	binary.LittleEndian.PutUint16(mask[0:], xiAllMasterDevices)
	binary.LittleEndian.PutUint16(mask[2:], 1)
	binary.LittleEndian.PutUint32(mask[4:], xiTouchMask)
	selectEvents(connection, window, 1, unsafe.Pointer(&mask[0]))
	api.flush(connection)
	return &xinputTouch{library: library, opcode: opcode}
}

// xinputTouchEvent is the part of an XI2 touch event this backend reads, from the event
// as the server sent it.
type xinputTouchEvent struct {
	kind                 platform.PointerEventKind
	id                   int32
	x, y, rootX, rootY   float32 // device pixels
	modifiers, timestamp uint32
}

// read decodes event when it is a touch event of the extension, and says whether it was.
// The positions are 16.16 fixed point; the touch id is the event's detail.
func (touch *xinputTouch) read(event unsafe.Pointer) (xinputTouchEvent, bool) {
	generic := (*xcbGenericEvent)(event)
	if generic.ResponseType&0x7f != xcbGenericEventType {
		return xinputTouchEvent{}, false
	}
	header := unsafe.Slice((*byte)(event), xiEventMinimumLength)
	if header[1] != touch.opcode {
		return xinputTouchEvent{}, false
	}
	word := func(offset int) uint32 { return binary.LittleEndian.Uint32(header[offset:]) }
	fixed := func(offset int) float32 { return float32(int32(word(offset))) / 65536 }
	result := xinputTouchEvent{
		id: int32(word(xiEventOffsetDetail)), timestamp: word(xiEventOffsetTime),
		x: fixed(xiEventOffsetEventX), y: fixed(xiEventOffsetEventX + 4),
		rootX: fixed(xiEventOffsetRootX), rootY: fixed(xiEventOffsetRootX + 4),
		modifiers: word(xiEventOffsetMods),
	}
	switch binary.LittleEndian.Uint16(header[8:]) {
	case xiTouchBegin:
		result.kind = platform.PointerDown
	case xiTouchUpdate:
		result.kind = platform.PointerMove
	case xiTouchEnd:
		result.kind = platform.PointerUp
		if word(xiEventOffsetFlags)&xiTouchPendingEnd != 0 {
			result.kind = platform.PointerCancel
		}
	default:
		return xinputTouchEvent{}, false
	}
	return result, true
}

// touch tells the application about a finger. The application's touch runtime reads
// the gestures from these; the text input is not told, as a drag on a text field
// scrolls what it is in rather than selecting.
func (router eventRouter) touch(event xinputTouchEvent, margin float32) error {
	if router.config.OnPointer == nil {
		return nil
	}
	scale := router.displayScale()
	pointerEvent := platform.PointerEvent{
		Kind: event.kind, Device: gfx.PointerTouch, PointerID: event.id,
		Modifiers: xcbModifiers(uint16(event.modifiers)),
		X:         (event.x - margin) / scale, Y: (event.y - margin) / scale,
		ScreenX: event.rootX / scale, ScreenY: event.rootY / scale,
	}
	if event.kind == platform.PointerDown || event.kind == platform.PointerUp {
		pointerEvent.Button = gfx.PointerButtonPrimary
	}
	frame, err := router.config.OnPointer(pointerEvent)
	return router.presentFrame(frame, err)
}
