//go:build !android

package linux

import (
	"encoding/binary"
	"os"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
)

// The fixture uses a private X server: changing keyboard focus is part of this test.
func TestNotificationNativeX11(t *testing.T) {
	if os.Getenv("VIVA_TEST_NOTIFICATION_XVFB") != "1" {
		t.Skip("requires a private Xvfb server")
	}
	owner, err := openWindow(platform.Config{Width: 160, Height: 100})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.close()
	var setFocus func(uintptr, uint8, uint32, uint32) xcbVoidCookie
	var getFocus func(uintptr) xcbVoidCookie
	var focusReply func(uintptr, xcbVoidCookie, *unsafe.Pointer) unsafe.Pointer
	purego.RegisterLibFunc(&setFocus, owner.api.library, "xcb_set_input_focus")
	purego.RegisterLibFunc(&getFocus, owner.api.library, "xcb_get_input_focus")
	purego.RegisterLibFunc(&focusReply, owner.api.library, "xcb_get_input_focus_reply")
	setFocus(owner.connection, 0, owner.id, 0)
	focused := func() uint32 {
		reply := focusReply(owner.connection, getFocus(owner.connection), nil)
		if reply == nil {
			t.Fatal("missing focus reply")
		}
		defer owner.api.free(reply)
		return *(*uint32)(unsafe.Add(reply, 8))
	}
	if focused() != owner.id {
		t.Fatal("owner did not acquire keyboard focus")
	}
	config := platform.Config{Width: 336, Height: 136, WindowMode: platform.WindowNotification}
	win, err := openWindow(config)
	if err != nil {
		t.Fatal(err)
	}
	defer win.close()
	x, y := win.rootOrigin()
	wantX, wantY := win.panelOrigin(config)
	if x != int32(wantX) || y != int32(wantY) {
		t.Fatalf("origin %d,%d want %d,%d", x, y, wantX, wantY)
	}
	getProperty := func(name string, kind uint32) uint32 {
		t.Helper()
		atom, err := win.api.atom(win.connection, name)
		if err != nil {
			t.Fatal(err)
		}
		buf, ok := win.tracker.property(win.id, atom, kind)
		if !ok || len(buf) != 4 {
			t.Fatalf("property %s: %x", name, buf)
		}
		return binary.LittleEndian.Uint32(buf)
	}
	notification, err := win.api.atom(win.connection, "_NET_WM_WINDOW_TYPE_NOTIFICATION")
	if err != nil || getProperty("_NET_WM_WINDOW_TYPE", xcbAtomAtom) != notification {
		t.Fatal("desktop notification type missing")
	}
	if getProperty("_NET_WM_USER_TIME", xcbAtomCardinal) != 0 || focused() != owner.id {
		t.Fatal("opening the desktop popup took the keyboard")
	}
	wmHints, err := win.api.atom(win.connection, "WM_HINTS")
	if err != nil {
		t.Fatal(err)
	}
	hints, ok := win.tracker.property(win.id, wmHints, wmHints)
	if !ok || len(hints) < 8 || binary.LittleEndian.Uint32(hints)&1 == 0 || binary.LittleEndian.Uint32(hints[4:]) != 0 {
		t.Fatal("notification did not opt out of keyboard focus in WM_HINTS")
	}
}
