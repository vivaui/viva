//go:build linux

package linux

import (
	"errors"
	"os"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// newWindowRenderer opens a window and the renderer of it, skipping where there
// is no display or no Vulkan driver.
func newWindowRenderer(t *testing.T, labels []platform.TextLabel) (*window, *vulkanRenderer) {
	t.Helper()
	if os.Getenv("DISPLAY") == "" {
		t.Skip("no X display")
	}
	win, err := openWindow(platform.Config{Title: "Viva recovery", Width: 120, Height: 80})
	if err != nil {
		t.Skipf("no X window: %v", err)
	}
	t.Cleanup(win.close)
	rectangles := []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 10, Width: 60, Height: 30}, Color: scene.Color{Red: 1, Alpha: 1}}}
	renderer, err := newVulkanRenderer(newXcbSurfaceProvider(win.connection, win.id), win.width, win.height, 1, platform.Color{Alpha: 1}, rectangles, labels, 0)
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan device: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(renderer.close)
	return win, renderer
}

func TestPresentRebuildsALostDevice(t *testing.T) {
	_, renderer := newWindowRenderer(t, []platform.TextLabel{{Value: "recover", X: 10, Y: 50, Width: 80, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}})
	if err := renderer.present(); err != nil {
		t.Fatal(err)
	}
	present := renderer.api.queuePresent
	lost := 1
	renderer.api.queuePresent = func(queue uintptr, info *vkPresentInfo) int32 {
		if lost > 0 {
			lost--
			return vkErrorDeviceLost
		}
		return present(queue, info)
	}
	if err := renderer.present(); err != nil {
		t.Fatalf("present after a lost device: %v", err)
	}
	if renderer.recoveries != 1 {
		t.Fatalf("recoveries = %d, want 1", renderer.recoveries)
	}
	if renderer.device == 0 {
		t.Fatal("the device was not made again")
	}
	if renderer.text.labels[0].view == 0 {
		t.Fatal("the label texture was not uploaded again")
	}
	if err := renderer.present(); err != nil {
		t.Fatalf("present after recovery: %v", err)
	}
}

func TestPresentRebuildsALostSurface(t *testing.T) {
	_, renderer := newWindowRenderer(t, nil)
	acquire := renderer.api.acquireNextImage
	lost := 1
	renderer.api.acquireNextImage = func(device, swapchain uintptr, timeout uint64, semaphore, fence uintptr, index *uint32) int32 {
		if lost > 0 {
			lost--
			return vkErrorSurfaceLost
		}
		return acquire(device, swapchain, timeout, semaphore, fence, index)
	}
	if err := renderer.present(); err != nil {
		t.Fatalf("present after a lost surface: %v", err)
	}
	if renderer.recoveries != 1 {
		t.Fatalf("recoveries = %d, want 1", renderer.recoveries)
	}
}

func TestPresentGivesUpOnADeviceThatStaysLost(t *testing.T) {
	_, renderer := newWindowRenderer(t, nil)
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkErrorDeviceLost }
	err := renderer.present()
	if !isGraphicsLoss(err) {
		t.Fatalf("present = %v, want the loss", err)
	}
	if renderer.recoveries != maxLossRecoveries {
		t.Fatalf("recoveries = %d, want %d", renderer.recoveries, maxLossRecoveries)
	}
}
