package linux

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func formatRecord(id uint32, kind, depth byte, red, green, blue, alpha [2]uint16) []byte {
	record := make([]byte, renderFormatInfoSize)
	binary.LittleEndian.PutUint32(record, id)
	record[4], record[5] = kind, depth
	for index, field := range [][2]uint16{red, green, blue, alpha} {
		binary.LittleEndian.PutUint16(record[8+index*4:], field[0])
		binary.LittleEndian.PutUint16(record[10+index*4:], field[1])
	}
	return record
}

// RENDER lists many formats; the cursor needs the one that holds 8-bit alpha, red, green and
// blue in a 32-bit pixel with alpha on top, and no other, however similar.
func TestTheARGB32FormatIsPickedFromTheServersList(t *testing.T) {
	var formats []byte
	formats = append(formats, formatRecord(1, 1, 24, [2]uint16{16, 255}, [2]uint16{8, 255}, [2]uint16{0, 255}, [2]uint16{0, 0})...)    // RGB, no alpha
	formats = append(formats, formatRecord(2, 1, 32, [2]uint16{0, 255}, [2]uint16{8, 255}, [2]uint16{16, 255}, [2]uint16{24, 255})...) // ABGR
	formats = append(formats, formatRecord(3, 0, 32, [2]uint16{16, 255}, [2]uint16{8, 255}, [2]uint16{0, 255}, [2]uint16{24, 255})...) // not direct
	if _, found := findARGB32(formats); found {
		t.Fatal("a format with another channel order was taken")
	}
	formats = append(formats, formatRecord(7, 1, 32, [2]uint16{16, 255}, [2]uint16{8, 255}, [2]uint16{0, 255}, [2]uint16{24, 255})...)
	if id, found := findARGB32(formats); !found || id != 7 {
		t.Fatalf("ARGB32 = %d, %v, want 7", id, found)
	}
}

// Against a real X server: the picture becomes a cursor of the window without any request being
// refused, and a second picture replaces it. Needs what the other XCB smoke tests need.
func TestAnApplicationPictureBecomesTheWindowsCursorOnARealServer(t *testing.T) {
	if testing.Short() || os.Getenv("VIVA_LINUX_XCB_SMOKE") == "" {
		t.Skip("set VIVA_LINUX_XCB_SMOKE=1 with an X11 display")
	}
	win, err := openWindow(platform.Config{Title: "Viva cursor smoke", Width: 160, Height: 100, WindowMode: platform.WindowTitled})
	if err != nil {
		t.Fatal(err)
	}
	if !activeWindow.CompareAndSwap(nil, win) {
		win.close()
		t.Fatal("an active window already exists")
	}
	t.Cleanup(func() {
		activeWindow.Store(nil)
		win.close()
	})
	picture := func(width, height, hotX, hotY int, scale float32) *ui.CursorImage {
		pixels := make([]byte, width*height*4)
		for index := 0; index < len(pixels); index += 4 {
			copy(pixels[index:], []byte{40, 120, 220, 200})
		}
		image, err := ui.NewCursorImage(width, height, hotX, hotY, scale, pixels)
		if err != nil {
			t.Fatal(err)
		}
		return image
	}
	if err := SetCursorImage(picture(24, 32, 3, 4, 1)); err != nil {
		if win.render == nil {
			t.Skipf("this server cannot make the cursor: %v", err)
		}
		t.Fatal(err)
	}
	first := win.cursor
	// Large enough to need several requests to send.
	if err := SetCursorImage(picture(200, 150, 100, 75, 1)); err != nil {
		t.Fatal(err)
	}
	if win.cursor == 0 || win.cursor == first {
		t.Fatalf("cursor after the second picture = %d, was %d", win.cursor, first)
	}
}
