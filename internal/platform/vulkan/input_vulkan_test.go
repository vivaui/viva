//go:build linux

package vulkan

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// A frame stages its scene and leaves the commands to the next submit: an image is
// recorded once for a scene, a frame that only changes instance data (a caret
// blinking) records nothing, and one that changes the batches records again.
func TestRendererRecordsAnImageOnlyWhenItsCommandsAreStale(t *testing.T) {
	waits, resets, begins, ends := 0, 0, 0, 0
	storage := make([]byte, 64*vkInstanceBytes)
	renderer := Renderer{
		api: &vulkanAPI{
			waitForFences: func(uintptr, uint32, *uintptr, uint32, uint64) int32 {
				waits++
				return vkSuccess
			},
			resetCommandPool: func(uintptr, uintptr, uint32) int32 {
				resets++
				return vkSuccess
			},
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 {
				begins++
				return vkSuccess
			},
			cmdBeginRenderPass:   func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdClearAttachments:  func(uintptr, uint32, *vkClearAttachment, uint32, *vkClearRect) {},
			cmdBindPipeline:      func(uintptr, uint32, uintptr) {},
			cmdSetViewport:       func(uintptr, uint32, uint32, *vkViewport) {},
			cmdSetScissor:        func(uintptr, uint32, uint32, *vkRect2D) {},
			cmdPushConstants:     func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer) {},
			cmdBindVertexBuffers: func(uintptr, uint32, uint32, *uintptr, *uint64) {},
			cmdDraw:              func(uintptr, uint32, uint32, uint32, uint32) {},
			cmdEndRenderPass:     func(uintptr) {},
			endCommandBuffer: func(uintptr) int32 {
				ends++
				return vkSuccess
			},
		},
		device:       1,
		commandPool:  2,
		fence:        3,
		width:        100,
		height:       80,
		commands:     []uintptr{4, 5},
		framebuffers: []uintptr{6, 7},
		scene:        vulkanScene{capacity: 64, mapped: unsafe.Pointer(&storage[0])},
	}
	if changed, err := renderer.Apply(platform.Frame{}); err != nil || changed {
		t.Fatalf("empty apply changed=%v err=%v", changed, err)
	}
	if waits != 0 || resets != 0 {
		t.Fatalf("empty apply waited=%d reset=%d", waits, resets)
	}
	renderer.recordExtent = vkExtent2D{Width: 100, Height: 80}
	rectangles := []render.Rectangle{{Bounds: scene.Rect{Width: 10, Height: 10}, Color: scene.Color{Alpha: 1}}}
	changed, err := renderer.Apply(platform.Frame{Update: render.Update{Rebuild: true, Count: 1}, Rectangles: rectangles})
	if err != nil || !changed {
		t.Fatalf("changed apply changed=%v err=%v", changed, err)
	}
	if waits != 1 || resets != 0 || begins != 0 {
		t.Fatalf("staging waits=%d resets=%d begins=%d, want the fence and no recording", waits, resets, begins)
	}
	submit := func() {
		t.Helper()
		if err := renderer.ensureRecorded(0); err != nil {
			t.Fatal(err)
		}
	}
	submit()
	submit()
	if begins != 1 || ends != 1 {
		t.Fatalf("one scene recorded image 0 begins=%d ends=%d, want once", begins, ends)
	}
	rectangles[0].Color.Alpha = 0.5 // a caret fading: new instance data, same batches
	if _, err := renderer.Apply(platform.Frame{Update: render.Update{First: 0, Count: 1}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	submit()
	if begins != 1 {
		t.Fatalf("a frame that changed instance data recorded again: begins=%d", begins)
	}
	rectangles = append(rectangles, render.Rectangle{Bounds: scene.Rect{Width: 5, Height: 5}, Color: scene.Color{Alpha: 1}, Layer: 3})
	if _, err := renderer.Apply(platform.Frame{Update: render.Update{First: 1, Count: 1}, Rectangles: rectangles}); err != nil {
		t.Fatal(err)
	}
	submit()
	if begins != 2 {
		t.Fatalf("a frame that changed the batches did not record again: begins=%d", begins)
	}
	rectangles[0].Bounds.Width = 99
	if renderer.rectangles[0].Bounds.Width != 10 {
		t.Fatal("renderer retained caller-owned rectangle storage")
	}
}
