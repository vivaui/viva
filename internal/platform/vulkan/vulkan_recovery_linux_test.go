//go:build linux

package vulkan

import (
	"errors"
	"testing"
	"time"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// newWindowRenderer makes the renderer of a window surface, skipping where there is
// no Vulkan driver or no surface to present to.
func newWindowRenderer(t *testing.T, labels []platform.TextLabel) *Renderer {
	t.Helper()
	renderer, _ := newResizableRenderer(t, labels)
	return renderer
}

// newResizableRenderer is newWindowRenderer with a function that asks the surface for
// another size, reporting false where the surface has no size of its own.
func newResizableRenderer(t *testing.T, labels []platform.TextLabel) (*Renderer, func(width, height uint32) bool) {
	t.Helper()
	source, resize := windowSurface(t, 120, 80)
	rectangles := []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 10, Width: 60, Height: 30}, Color: scene.Color{Red: 1, Alpha: 1}}}
	renderer, err := New(Config{
		Surface: source, Width: 120, Height: 80, Scale: 1, Clear: platform.Color{Alpha: 1},
		Rectangles: rectangles, Labels: labels, Rasterizer: newSolidRasterizer,
	})
	if errors.Is(err, platform.ErrNoGraphics) {
		t.Skipf("no Vulkan device: %v", err)
	}
	if err != nil {
		t.Skipf("no Vulkan surface: %v", err)
	}
	t.Cleanup(renderer.Close)
	return renderer, resize
}

func TestPresentRebuildsALostDevice(t *testing.T) {
	renderer := newWindowRenderer(t, []platform.TextLabel{{Value: "recover", X: 10, Y: 50, Width: 80, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}})
	if err := renderer.Present(); err != nil {
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
	if err := renderer.Present(); err != nil {
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
	if err := renderer.Present(); err != nil {
		t.Fatalf("present after recovery: %v", err)
	}
}

func TestPresentRebuildsALostSurface(t *testing.T) {
	renderer := newWindowRenderer(t, nil)
	acquire := renderer.api.acquireNextImage
	lost := 1
	renderer.api.acquireNextImage = func(device, swapchain uintptr, timeout uint64, semaphore, fence uintptr, index *uint32) int32 {
		if lost > 0 {
			lost--
			return vkErrorSurfaceLost
		}
		return acquire(device, swapchain, timeout, semaphore, fence, index)
	}
	if err := renderer.Present(); err != nil {
		t.Fatalf("present after a lost surface: %v", err)
	}
	if renderer.recoveries != 1 {
		t.Fatalf("recoveries = %d, want 1", renderer.recoveries)
	}
}

func TestPresentGivesUpOnADeviceThatStaysLost(t *testing.T) {
	renderer := newWindowRenderer(t, nil)
	renderer.api.queuePresent = func(uintptr, *vkPresentInfo) int32 { return vkErrorDeviceLost }
	err := renderer.Present()
	if !isGraphicsLoss(err) {
		t.Fatalf("present = %v, want the loss", err)
	}
	if renderer.recoveries != maxLossRecoveries {
		t.Fatalf("recoveries = %d, want %d", renderer.recoveries, maxLossRecoveries)
	}
}

func TestResizeRecordsCommandsAfterStageRecovery(t *testing.T) {
	for _, loss := range []struct {
		name   string
		status int32
	}{{"device", vkErrorDeviceLost}, {"surface", vkErrorSurfaceLost}} {
		t.Run(loss.name, func(t *testing.T) {
			renderer, resize := newResizableRenderer(t, nil)
			if resize(160, 100) {
				deadline := time.Now().Add(3 * time.Second)
				for {
					if width, height := renderer.SurfaceSize(160, 100); width == 160 && height == 100 {
						break
					}
					if time.Now().After(deadline) {
						t.Fatal("the surface never reached 160x100")
					}
					time.Sleep(time.Millisecond)
				}
			}
			frame := platform.Frame{
				Rectangles: []render.Rectangle{{Kind: scene.KindRectangle, Bounds: scene.Rect{Width: 160, Height: 100}, Color: scene.Color{Red: 1, Alpha: 1}}},
				Update:     render.Update{Count: 1, Rebuild: true},
			}

			recorded := make(map[uintptr]bool)
			reset := renderer.api.resetCommandPool
			renderer.api.resetCommandPool = func(device, pool uintptr, flags uint32) int32 {
				clear(recorded)
				return reset(device, pool, flags)
			}
			begin := renderer.api.beginCommandBuffer
			renderer.api.beginCommandBuffer = func(command uintptr, info *vkCommandBufferBeginInfo) int32 {
				delete(recorded, command) // beginning a buffer resets it
				return begin(command, info)
			}
			end := renderer.api.endCommandBuffer
			renderer.api.endCommandBuffer = func(command uintptr) int32 {
				status := end(command)
				if status == vkSuccess {
					recorded[command] = true
				}
				return status
			}
			wait := renderer.api.waitForFences
			lost := true
			renderer.api.waitForFences = func(device uintptr, count uint32, fences *uintptr, all uint32, timeout uint64) int32 {
				if lost {
					lost = false
					return loss.status
				}
				return wait(device, count, fences, all, timeout)
			}
			submit := renderer.api.queueSubmit
			renderer.api.queueSubmit = func(queue uintptr, count uint32, info *vkSubmitInfo, fence uintptr) int32 {
				if info.CommandCount != 0 && !recorded[*info.Commands] {
					t.Errorf("submitted reset command buffer after %d recoveries at %dx%d", renderer.recoveries, renderer.width, renderer.height)
					return vkErrorInitializationFailed // keep invalid commands away from the driver
				}
				return submit(queue, count, info, fence)
			}
			// What a window does when it grows: stage the new scene, resize the
			// swapchain to the new size, present.
			if _, err := renderer.Stage(frame); err != nil {
				t.Fatalf("stage: %v", err)
			}
			if err := renderer.Resize(160, 100); err != nil {
				t.Fatalf("resize: %v", err)
			}
			if err := renderer.Present(); err != nil {
				t.Fatalf("present: %v", err)
			}
			if renderer.recoveries != 1 || renderer.submissions != 1 {
				t.Fatalf("recoveries=%d submissions=%d, want one recovered frame", renderer.recoveries, renderer.submissions)
			}
		})
	}
}
