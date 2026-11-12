package windows

import (
	"syscall"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// staticWindow is a hidden window of the system's own class, a surface to make a
// swap chain on without the event loop.
func staticWindow(t *testing.T) uintptr {
	t.Helper()
	class, _ := syscall.UTF16PtrFromString("STATIC")
	window, _, err := createWindow.Call(0, uintptr(unsafe.Pointer(class)), 0, wsPopup, 0, 0, 64, 64, 0, 0, 0, 0)
	if window == 0 {
		t.Fatal(err)
	}
	t.Cleanup(func() { destroyWindow.Call(window) })
	return window
}

// TestRendererMakesItsDeviceAgain recovers a renderer that still has its device,
// as a recovery from a lost one does, and checks that what it holds in memory is
// uploaded again: the rectangles to a new buffer, the labels as they are drawn.
func TestRendererMakesItsDeviceAgain(t *testing.T) {
	var drawn renderer
	if err := drawn.initialize(staticWindow(t), platform.Color{Alpha: 1}); err != nil {
		t.Skipf("no graphics device: %v", err)
	}
	defer drawn.close()
	rectangles := []render.Rectangle{
		{Bounds: scene.Rect{X: 2, Y: 2, Width: 20, Height: 20}, Color: scene.Color{Red: 1, Alpha: 1}},
		{Bounds: scene.Rect{X: 8, Y: 8, Width: 20, Height: 20}, Color: scene.Color{Green: 1, Alpha: 1}},
	}
	labels := []platform.TextLabel{{Value: "Viva", X: 2, Y: 30, Width: 40, Height: 20, FontSize: 12, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}}
	if err := drawn.apply(platform.Frame{Update: initialUpdate(len(rectangles)), Rectangles: rectangles, TextLabels: labels}); err != nil {
		t.Fatal(err)
	}
	if err := drawn.present(64, 64, 1); err != nil {
		t.Fatal(err)
	}
	if drawn.deviceLost() {
		t.Fatal("a healthy device reads as lost")
	}
	for _, software := range []bool{false, true} {
		picture := drawn.labels[0].picture
		if err := drawn.recover(software); err != nil {
			t.Fatalf("recover(software %v): %v", software, err)
		}
		if drawn.capacity != len(rectangles) || drawn.buffer == 0 || drawn.view == 0 {
			t.Fatalf("the rectangles were not uploaded again: capacity %d", drawn.capacity)
		}
		if drawn.labels[0].view != 0 || drawn.labels[0].label.Value != "Viva" {
			t.Fatal("a label kept a texture of the old device, or lost its text")
		}
		if picture == nil || picture.view != 0 || picture.texture != 0 || drawn.labelPictures.bytes != 0 {
			t.Fatal("device recovery retained a picture from the old device")
		}
		if err := drawn.present(64, 64, 1); err != nil {
			t.Fatalf("present after recover(software %v): %v", software, err)
		}
		if drawn.labels[0].view == 0 {
			t.Fatal("the label was not drawn on the new device")
		}
		if drawn.labels[0].picture == picture {
			t.Fatal("device recovery reused a revoked picture")
		}
	}
}
