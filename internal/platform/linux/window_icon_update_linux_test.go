package linux

import (
	"encoding/binary"
	"os"
	"slices"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestNativeX11WindowIconPropertyUpdates(t *testing.T) {
	if os.Getenv("DISPLAY") == "" {
		t.Skip("requires X11 or Xvfb")
	}
	win, err := openWindow(platform.Config{Title: "Viva icon property test", Width: 120, Height: 80})
	if err != nil {
		t.Fatal(err)
	}
	defer win.close()
	window := &nativeWindow{state: &linuxWindowState{win: win}}
	for _, color := range []byte{50, 210} {
		icon := platform.WindowIcon{Size: 16, RGBA: make([]byte, 16*16*4)}
		for i := 0; i < len(icon.RGBA); i += 4 {
			copy(icon.RGBA[i:], []byte{color, 40, 70, 255})
		}
		if err := window.SetIcon(icon); err != nil {
			t.Fatal(err)
		}
		if err := window.state.command(commandSetIcon, nil); err != nil {
			t.Fatal(err)
		}
		data, format := windowProperty(t, win, "_NET_WM_ICON", xcbAtomCardinal)
		want := netWMIcon(icon)
		if format != 32 || len(data) != len(want)*4 {
			t.Fatalf("native icon property: format %d, %d bytes", format, len(data))
		}
		for i, pixel := range want {
			if got := binary.LittleEndian.Uint32(data[i*4:]); got != pixel {
				t.Fatalf("native property pixel %d: %#x, want %#x", i, got, pixel)
			}
		}
	}
}

func TestWindowIconUpdateWritesTheLatestIconOnTheEventLoop(t *testing.T) {
	var ignored []titleProperty
	win := newTitleTestWindow(t, &ignored)
	win.api.free = func(unsafe.Pointer) {}
	var written []uint32
	win.api.internAtom = func(uintptr, uint8, uint16, *byte) xcbAtomCookie {
		return xcbAtomCookie{Sequence: 73}
	}
	win.api.changeProperty = func(_ uintptr, _ uint8, _ uint32, property, kind uint32, format uint8, length uint32, data unsafe.Pointer) xcbVoidCookie {
		if property != 73 || kind != xcbAtomCardinal || format != 32 {
			t.Fatalf("wrong icon property: %d %d %d", property, kind, format)
		}
		written = slices.Clone(unsafe.Slice((*uint32)(data), int(length)))
		return xcbVoidCookie{}
	}
	window := &nativeWindow{state: &linuxWindowState{win: win}}
	var last platform.WindowIcon
	for _, red := range []byte{100, 200} {
		last = platform.WindowIcon{Size: 1, RGBA: []byte{red, 20, 30, 255}}
		if err := window.SetIcon(last); err != nil {
			t.Fatal(err)
		}
	}
	if written != nil {
		t.Fatal("the caller wrote X11 properties outside the event loop")
	}
	if err := window.state.command(commandSetIcon, nil); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(written, netWMIcon(last)) {
		t.Fatal("the event loop did not publish the latest icon")
	}
	window.closed.Store(true)
	if err := window.SetIcon(last); err == nil {
		t.Fatal("a closed window accepted an icon")
	}
}
