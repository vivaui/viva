//go:build !android

package linux

import (
	"os"
	"testing"
	"time"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// Mapping/reparenting or uncovering can invalidate pixels again after the
// first Expose. Recover from the server's damage request without pointer input
// or changes to the application's retained scene.
func TestTrayWindowRepaintsLaterExposure(t *testing.T) {
	if os.Getenv("VIVA_TEST_NOTIFICATION_XVFB") != "1" {
		t.Skip("requires a private Xvfb server")
	}
	t.Setenv("VIVA_SOFTWARE_RENDERING", "1")
	a := &linuxApplication{}
	control, err := a.OpenWindow(platform.Config{
		Title: "Viva tray exposure", Width: 180, Height: 66, WindowMode: platform.WindowPanel,
		Rectangles: []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 180, Height: 66}, Color: scene.Color{Red: 1, Alpha: 1}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.closeAll()
	s := control.(*nativeWindow).state
	drain := func() {
		deadline := time.Now().Add(100 * time.Millisecond)
		for time.Now().Before(deadline) {
			e := s.nextEvent()
			if e == nil {
				time.Sleep(time.Millisecond)
				continue
			}
			closed, err := s.handleEvent(e)
			s.win.api.free(e)
			if err != nil || closed {
				t.Fatalf("dispatch: closed=%v err=%v", closed, err)
			}
		}
	}
	drain()
	if !s.exposed {
		t.Fatal("window never mapped")
	}
	var clearArea func(uintptr, uint8, uint32, int16, int16, uint16, uint16) xcbVoidCookie
	purego.RegisterLibFunc(&clearArea, s.win.api.library, "xcb_clear_area")
	for range 2 {
		clearArea(s.win.connection, 1, s.win.id, 0, 0, 180, 66)
		s.win.api.flush(s.win.connection)
		drain()
		pixel := readWindowPixels(t, s.win, 20, 20, 1, 1)
		if pixel[0] != 0 || pixel[1] != 0 || pixel[2] != 255 {
			t.Fatalf("exposed tray remains blank without pointer input: %v", pixel[:4])
		}
	}
	presents := s.renderer.stats().presents
	drain()
	if got := s.renderer.stats().presents; got != presents {
		t.Fatalf("idle tray keeps repainting: %d -> %d", presents, got)
	}
}

func TestExposureDoesNotPaintHiddenOrForeignWindow(t *testing.T) {
	r := &fakeWindowRenderer{}
	s := &linuxWindowState{win: &window{id: 9}, renderer: r, hidden: true}
	e := xcbExposeEvent{ResponseType: xcbExpose, Window: 9}
	if _, err := s.handleEvent(unsafe.Pointer(&e)); err != nil {
		t.Fatal(err)
	}
	s.hidden = false
	e.Window = 10
	if _, err := s.handleEvent(unsafe.Pointer(&e)); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 0 {
		t.Fatal("painted a hidden or unrelated window", r.calls)
	}
}
