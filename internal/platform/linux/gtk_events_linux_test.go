//go:build linux && vivagtk

package linux

import (
	"testing"
	"unsafe"
)

// The event structs are read straight out of memory GDK owns, so their layout is an ABI the tests
// hold to the offsets of gdk/gdkevents.h on LP64 (amd64 and arm64).
func TestGDKEventLayoutsMatchGTK3(t *testing.T) {
	for _, field := range []struct {
		name      string
		got, want uintptr
	}{
		{"button.Time", unsafe.Offsetof(gdkEventButton{}.Time), 20},
		{"button.X", unsafe.Offsetof(gdkEventButton{}.X), 24},
		{"button.State", unsafe.Offsetof(gdkEventButton{}.State), 48},
		{"button.Button", unsafe.Offsetof(gdkEventButton{}.Button), 52},
		{"button.XRoot", unsafe.Offsetof(gdkEventButton{}.XRoot), 64},
		{"motion.State", unsafe.Offsetof(gdkEventMotion{}.State), 48},
		{"motion.XRoot", unsafe.Offsetof(gdkEventMotion{}.XRoot), 64},
		{"scroll.State", unsafe.Offsetof(gdkEventScroll{}.State), 40},
		{"scroll.Direction", unsafe.Offsetof(gdkEventScroll{}.Direction), 44},
		{"scroll.DeltaX", unsafe.Offsetof(gdkEventScroll{}.DeltaX), 72},
		{"scroll.DeltaY", unsafe.Offsetof(gdkEventScroll{}.DeltaY), 80},
		{"key.State", unsafe.Offsetof(gdkEventKey{}.State), 24},
		{"key.Keyval", unsafe.Offsetof(gdkEventKey{}.Keyval), 28},
		{"key.HardwareKeycode", unsafe.Offsetof(gdkEventKey{}.HardwareKeycode), 48},
		{"crossing.X", unsafe.Offsetof(gdkEventCrossing{}.X), 40},
		{"crossing.Mode", unsafe.Offsetof(gdkEventCrossing{}.Mode), 72},
		{"focus.In", unsafe.Offsetof(gdkEventFocus{}.In), 18},
		{"configure.X", unsafe.Offsetof(gdkEventConfigure{}.X), 20},
		{"windowState.Changed", unsafe.Offsetof(gdkEventWindowState{}.Changed), 20},
		{"windowState.New", unsafe.Offsetof(gdkEventWindowState{}.New), 24},
	} {
		if field.got != field.want {
			t.Errorf("%s is at offset %d, GTK 3 has it at %d", field.name, field.got, field.want)
		}
	}
	if got := unsafe.Sizeof(gValue{}); got != 24 {
		t.Errorf("GValue is %d bytes, want 24", got)
	}
	if got := unsafe.Sizeof(gdkGeometry{}); got != 56 {
		t.Errorf("GdkGeometry is %d bytes, want 56", got)
	}
}

// wl_interface and wl_message are written by hand where libwayland has no table for a protocol; the
// layout is the one wayland-util.h gives.
func TestWaylandInterfaceTablesAreLaidOutLikeLibwayland(t *testing.T) {
	iface := wlInterface("wp_test", 3, []wlMsg{{"a", "", nil}, {"b", "ii", []uintptr{0, 0}}}, []wlMsg{{"e", "u", []uintptr{0}}})
	if got := goString(*(*uintptr)(unsafePointer(iface))); got != "wp_test" {
		t.Fatalf("name = %q", got)
	}
	if version := *(*int32)(unsafePointer(iface + 8)); version != 3 {
		t.Fatalf("version = %d", version)
	}
	if methods := *(*int32)(unsafePointer(iface + 12)); methods != 2 {
		t.Fatalf("method count = %d", methods)
	}
	table := *(*uintptr)(unsafePointer(iface + 16))
	second := table + 24 // wl_message is name, signature, types
	if got, want := goString(*(*uintptr)(unsafePointer(second))), "b"; got != want {
		t.Fatalf("second request = %q, want %q", got, want)
	}
	if got := goString(*(*uintptr)(unsafePointer(second + 8))); got != "ii" {
		t.Fatalf("its signature = %q", got)
	}
	if events := *(*int32)(unsafePointer(iface + 24)); events != 1 {
		t.Fatalf("event count = %d", events)
	}
}

func TestWaylandPointerButtonsMapToGDKNumbers(t *testing.T) {
	for evdev, want := range map[uint32]uint32{0x110: 1, 0x112: 2, 0x111: 3, 0x113: 8, 0x116: 8, 0x114: 9, 0x115: 9, 0x120: 0} {
		if got := gdkButtonOfEvdev(evdev); got != want {
			t.Errorf("evdev %#x = GDK %d, want %d", evdev, got, want)
		}
	}
}
