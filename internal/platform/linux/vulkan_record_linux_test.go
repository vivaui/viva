//go:build linux

package linux

import (
	"testing"
	"unsafe"
)

// TestRendererReRecordingResetsCommands guards the command-pool rule that a
// buffer left executable by a previous frame must be reset before it can be
// begun again. The pool has no implicit-reset flag, so re-recording paths
// (scale change, text refresh, frame margin) must reset it themselves.
func TestRendererReRecordingResetsCommands(t *testing.T) {
	var order []string
	record := func(event string) { order = append(order, event) }
	renderer := vulkanRenderer{
		api: &vulkanAPI{
			deviceWaitIdle: func(uintptr) int32 {
				record("wait")
				return vkSuccess
			},
			resetCommandPool: func(uintptr, uintptr, uint32) int32 {
				record("reset")
				return vkSuccess
			},
			beginCommandBuffer: func(uintptr, *vkCommandBufferBeginInfo) int32 {
				record("begin")
				return vkSuccess
			},
			cmdBeginRenderPass:   func(uintptr, *vkRenderPassBeginInfo, uint32) {},
			cmdEndRenderPass:     func(uintptr) {},
			endCommandBuffer:     func(uintptr) int32 { record("end"); return vkSuccess },
			cmdBindPipeline:      func(uintptr, uint32, uintptr) {},
			cmdSetViewport:       func(uintptr, uint32, uint32, *vkViewport) {},
			cmdSetScissor:        func(uintptr, uint32, uint32, *vkRect2D) {},
			cmdPushConstants:     func(uintptr, uintptr, uint32, uint32, uint32, unsafe.Pointer) {},
			cmdBindVertexBuffers: func(uintptr, uint32, uint32, *uintptr, *uint64) {},
			cmdDraw:              func(uintptr, uint32, uint32, uint32, uint32) {},
			cmdClearAttachments:  func(uintptr, uint32, *vkClearAttachment, uint32, *vkClearRect) {},
		},
		device:       1,
		commandPool:  2,
		width:        100,
		height:       80,
		commands:     []uintptr{3},
		framebuffers: []uintptr{4},
	}

	if err := renderer.setScaleOnce(1.5); err != nil {
		t.Fatalf("set scale: %v", err)
	}
	want := []string{"wait", "reset", "begin", "end"}
	assertCommandOrder(t, order, want)

	order = order[:0]
	if err := renderer.refreshText(); err != nil {
		t.Fatalf("refresh text: %v", err)
	}
	assertCommandOrder(t, order, want)
}

func assertCommandOrder(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("command order = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("command order = %v, want %v", got, want)
		}
	}
}
