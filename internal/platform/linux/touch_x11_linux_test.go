//go:build !android

package linux

import (
	"encoding/binary"
	"os"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// xiTouchEventBytes is a touch event as the server sends it: a generic event of the
// extension, the touch's id in the detail, 16.16 fixed point positions.
func xiTouchEventBytes(opcode uint8, eventType uint16, id uint32, x, y float32, flags uint32) []byte {
	event := make([]byte, 96)
	event[0], event[1] = xcbGenericEventType, opcode
	binary.LittleEndian.PutUint16(event[8:], eventType)
	binary.LittleEndian.PutUint32(event[xiEventOffsetTime:], 4242)
	binary.LittleEndian.PutUint32(event[xiEventOffsetDetail:], id)
	put := func(offset int, value float32) {
		binary.LittleEndian.PutUint32(event[offset:], uint32(int32(value*65536)))
	}
	put(xiEventOffsetRootX, x+100)
	put(xiEventOffsetRootX+4, y+200)
	put(xiEventOffsetEventX, x)
	put(xiEventOffsetEventX+4, y)
	binary.LittleEndian.PutUint32(event[xiEventOffsetFlags:], flags)
	return event
}

func TestATouchEventIsReadFromTheServersBytes(t *testing.T) {
	touch := &xinputTouch{opcode: 131}
	for _, c := range []struct {
		eventType uint16
		flags     uint32
		want      platform.PointerEventKind
	}{
		{xiTouchBegin, 0, platform.PointerDown},
		{xiTouchUpdate, 0, platform.PointerMove},
		{xiTouchEnd, 0, platform.PointerUp},
		{xiTouchEnd, xiTouchPendingEnd, platform.PointerCancel},
	} {
		bytes := xiTouchEventBytes(131, c.eventType, 7, 12.5, -3.25, c.flags)
		event, ok := touch.read(unsafe.Pointer(&bytes[0]))
		if !ok || event.kind != c.want || event.id != 7 || event.x != 12.5 || event.y != -3.25 || event.rootX != 112.5 || event.rootY != 196.75 {
			t.Errorf("type %d flags %#x: read %+v, %v; want kind %v at 12.5, -3.25 as touch 7", c.eventType, c.flags, event, ok, c.want)
		}
	}
}

func TestOtherEventsAreNotTouches(t *testing.T) {
	touch := &xinputTouch{opcode: 131}
	other := xiTouchEventBytes(130, xiTouchBegin, 1, 0, 0, 0) // another extension's
	if _, ok := touch.read(unsafe.Pointer(&other[0])); ok {
		t.Error("an event of another extension was read as a touch")
	}
	ownership := xiTouchEventBytes(131, 21, 1, 0, 0, 0) // XI_TouchOwnership is not selected, and not a finger moving
	if _, ok := touch.read(unsafe.Pointer(&ownership[0])); ok {
		t.Error("a touch ownership event was read as a touch")
	}
	core := make([]byte, 96)
	core[0] = xcbButtonPress
	if _, ok := touch.read(unsafe.Pointer(&core[0])); ok {
		t.Error("a core event was read as a touch")
	}
}

// Selecting the touch events must be accepted by a real server: a mistake in the request
// would shut the connection down or answer with an error.
func TestTheServerAcceptsTheTouchSelection(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display")
	}
	win, err := openWindow(platform.Config{Title: "Viva touch smoke", Width: 160, Height: 100, WindowMode: platform.WindowTitled})
	if err != nil {
		t.Fatal(err)
	}
	defer win.close()
	touch := loadXInputTouch(win.api, win.connection, win.id)
	if touch == nil {
		t.Skip("the server has no X Input 2.2 or libxcb-xinput is missing")
	}
	// A round trip makes the server answer every request before it, an error included.
	win.api.flush(win.connection)
	if _, err := win.readMaximized(); err != nil {
		t.Fatalf("a round trip after selecting touch events: %v", err)
	}
	if win.api.connectionHasError(win.connection) != 0 {
		t.Fatal("the connection failed after selecting touch events")
	}
	for event := win.api.pollForEvent(win.connection); event != nil; event = win.api.pollForEvent(win.connection) {
		if (*xcbGenericEvent)(event).ResponseType == 0 {
			t.Errorf("the server answered the touch selection with an X error: code %d", unsafe.Slice((*byte)(event), 32)[1])
		}
		win.api.free(event)
	}
}
