package ui_test

import (
	"runtime"
	"testing"
	"time"

	"github.com/vivaui/viva/ui"
)

// A picture the page stops showing is the garbage collector's once the frame
// that last showed it is past: the frame's reused arrays do not keep it.
func TestAPictureThePageNoLongerShowsIsReleased(t *testing.T) {
	show := true
	released := make(chan struct{}, 1)
	pixels := make([]byte, 1<<20)
	runtime.AddCleanup(&pixels[0], func(chan struct{}) { released <- struct{}{} }, released)
	bitmap := ui.Bitmap{Pix: pixels, Width: 512, Height: 512, Stride: 2048, ID: 1}
	pixels = nil
	d := openPage(t, func(c ui.C) {
		ui.Text(c, "gallery")
		if show {
			ui.Image(c, bitmap).Size(64, 64)
		}
	})
	show, bitmap = false, ui.Bitmap{}
	for range 3 {
		d.repaint()
	}
	for range 20 {
		runtime.GC()
		select {
		case <-released:
			return
		case <-time.After(10 * time.Millisecond):
		}
	}
	t.Fatal("the picture is still reachable after the page stopped showing it")
}
