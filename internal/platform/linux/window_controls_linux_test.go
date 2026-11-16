package linux

import (
	"encoding/binary"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestEdgesMapToXMoveResizeDirections(t *testing.T) {
	// _NET_WM_MOVERESIZE: 0 top left, 1 top, 2 top right, 3 right, 4 bottom
	// right, 5 bottom, 6 bottom left, 7 left.
	for want, edge := range []platform.WindowEdge{
		platform.EdgeTopLeft, platform.EdgeTop, platform.EdgeTopRight, platform.EdgeRight,
		platform.EdgeBottomRight, platform.EdgeBottom, platform.EdgeBottomLeft, platform.EdgeLeft,
	} {
		if got := moveResizeDirection(edge); got != uint32(want) {
			t.Errorf("edge %d = direction %d, want %d", edge, got, want)
		}
	}
}

func TestMaximizeButtonSetsBothDirections(t *testing.T) {
	var words [5]uint32
	win := &window{
		api: &xcbAPI{
			sendEvent: func(_ uintptr, _ uint8, _, _ uint32, data *byte) xcbVoidCookie {
				message := (*xcbClientEvent)(unsafe.Pointer(data))
				for index := range words {
					words[index] = binary.LittleEndian.Uint32(message.Data[index*4:])
				}
				return xcbVoidCookie{}
			},
			flush: func(uintptr) int32 { return 0 },
		},
		screen: &xcbScreen{Root: 1},
		frame: frameState{
			stateAtom:           10,
			maximizedVertical:   20,
			maximizedHorizontal: 30,
		},
	}

	// EWMH 5.7: data.l[0] is the action (0 remove, 1 add), l[1] and l[2] are
	// the properties it applies to, l[3] is the source indication (1 for a
	// normal application). The literals are the protocol's, not this package's.
	win.toggleMaximize()
	if words != [5]uint32{1, 20, 30, 1, 0} {
		t.Fatalf("maximize message = %v, want both directions added", words)
	}

	win.frame.maximized = true
	win.toggleMaximize()
	if words != [5]uint32{0, 20, 30, 1, 0} {
		t.Fatalf("restore message = %v, want both directions removed", words)
	}
}

func TestADoubleClickNeedsTwoClosePressesInTime(t *testing.T) {
	var win window
	press := func(time uint32, x, y int16) bool {
		return win.doubleClick(&xcbInputEvent{Time: time, EventX: x, EventY: y})
	}
	if press(1000, 100, 10) {
		t.Fatal("the first press is not a double click")
	}
	if !press(1200, 102, 11) {
		t.Fatal("two close presses did not make a double click")
	}
	if press(1300, 102, 11) {
		t.Fatal("a third press right after a double click made another")
	}
	if press(2000, 100, 10) || press(2600, 100, 10) {
		t.Fatal("presses far apart in time made a double click")
	}
	if press(3500, 100, 10) || press(3600, 300, 10) {
		t.Fatal("presses far apart in space made a double click")
	}
}

func TestDragSlopScalesWithDisplay(t *testing.T) {
	var win window
	if slop := win.dragSlop(); slop != 4 {
		t.Fatalf("dragSlop at 1x = %d, want 4", slop)
	}
	if slop := win.doubleClickSlop(); slop != 4 {
		t.Fatalf("doubleClickSlop at 1x = %d, want 4", slop)
	}

	win.storeScale(1.25)
	if slop := win.dragSlop(); slop != 5 {
		t.Fatalf("dragSlop at 1.25x = %d, want 5", slop)
	}
	if slop := win.doubleClickSlop(); slop != 5 {
		t.Fatalf("doubleClickSlop at 1.25x = %d, want 5", slop)
	}

	win.storeScale(2)
	if slop := win.dragSlop(); slop != 8 {
		t.Fatalf("dragSlop at 2x = %d, want 8", slop)
	}
	if slop := win.doubleClickSlop(); slop != 8 {
		t.Fatalf("doubleClickSlop at 2x = %d, want 8", slop)
	}
}

func TestTitlebarDoubleClickSlopTolerantToJitter(t *testing.T) {
	var win window
	win.storeScale(2)
	press := func(time uint32, x, y int16) bool {
		return win.doubleClick(&xcbInputEvent{Time: time, EventX: x, EventY: y})
	}
	if press(1000, 100, 10) {
		t.Fatal("the first press is not a double click")
	}
	// At 2x scale, slop is 8px. A 5px jitter between clicks is tolerated.
	if !press(1200, 105, 13) {
		t.Fatal("two presses with small cursor jitter should make a double click on HiDPI")
	}
}
