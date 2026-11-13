package linux

import (
	"encoding/binary"
	"testing"
	"unsafe"
)

func TestSetSizeCommandUsesLogicalContentSize(t *testing.T) {
	var gotWidth, gotHeight uint32
	var gotMask uint16
	flushed := false
	win := &window{
		id:     7,
		margin: frameMargin(1.5),
		api: &xcbAPI{
			configureWindow: func(_ uintptr, window uint32, mask uint16, values *uint32) xcbVoidCookie {
				if window != 7 {
					t.Fatalf("configured window = %d, want 7", window)
				}
				gotMask = mask
				size := unsafe.Slice(values, 2)
				gotWidth, gotHeight = size[0], size[1]
				return xcbVoidCookie{}
			},
			flush: func(uintptr) int32 {
				flushed = true
				return 1
			},
		},
	}
	win.storeScale(1.5)
	state := &linuxWindowState{win: win}
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], commandSetSize)
	binary.LittleEndian.PutUint32(data[4:8], 420)
	binary.LittleEndian.PutUint32(data[8:12], 300)
	if err := state.command(commandSetSize, data); err != nil {
		t.Fatal(err)
	}
	margin := 2 * frameMargin(1.5)
	if want := deviceLength(420, 1.5) + margin; gotWidth != want {
		t.Fatalf("configured width = %d, want %d", gotWidth, want)
	}
	if want := deviceLength(300, 1.5) + margin; gotHeight != want {
		t.Fatalf("configured height = %d, want %d", gotHeight, want)
	}
	if want := uint16(xcbConfigWindowWidth | xcbConfigWindowHeight); gotMask != want {
		t.Fatalf("configure mask = %#x, want %#x", gotMask, want)
	}
	if !flushed {
		t.Fatal("size request was not flushed")
	}
}

func TestSetSizeCommandIgnoresMinimizedWindows(t *testing.T) {
	configured := false
	win := &window{
		id:        7,
		minimized: true,
		api: &xcbAPI{
			configureWindow: func(uintptr, uint32, uint16, *uint32) xcbVoidCookie {
				configured = true
				return xcbVoidCookie{}
			},
		},
	}
	state := &linuxWindowState{win: win}
	data := make([]byte, 12)
	binary.LittleEndian.PutUint32(data[0:4], commandSetSize)
	binary.LittleEndian.PutUint32(data[4:8], 420)
	binary.LittleEndian.PutUint32(data[8:12], 300)
	if err := state.command(commandSetSize, data); err != nil {
		t.Fatal(err)
	}
	if configured {
		t.Fatal("minimized window was resized")
	}
}
