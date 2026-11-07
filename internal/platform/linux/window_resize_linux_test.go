package linux

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

func TestMovingWindowDoesNotQueryVulkanSurface(t *testing.T) {
	queries := 0
	var frames []ui.Rect
	state := &linuxWindowState{
		win: &window{id: 1, width: 100, height: 100},
		config: platform.Config{
			OnFrameChange: func(frame ui.Rect) { frames = append(frames, frame) },
		},
		renderer: &vulkanRenderer{
			physical: 1, surface: 2, width: 100, height: 100,
			api: &vulkanAPI{getSurfaceCapabilities: func(_, _ uintptr, caps *vkSurfaceCapabilities) int32 {
				queries++
				caps.Current = vkExtent2D{Width: 100, Height: 100}
				return vkSuccess
			}},
		},
	}
	for x := int16(10); x < 30; x++ {
		event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, X: x, Y: 20, Width: 100, Height: 100}
		if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
			t.Fatalf("move: closed=%v err=%v", closed, err)
		}
	}
	if queries != 0 || state.resizeEvents != 0 {
		t.Fatalf("pure movement queried the surface %d times and resized %d times", queries, state.resizeEvents)
	}
	if len(frames) != 20 || frames[len(frames)-1] != (ui.Rect{X: 29, Y: 20, Width: 100, Height: 100}) {
		t.Fatalf("movement stopped reporting the content frame: %v", frames)
	}
}

func TestObsoleteConfigureDoesNotCacheSurfaceForLaterPresent(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkErrorOutOfDate, vkSuccess, &recreations)
	renderer.physical, renderer.surface = 1, 2
	renderer.width, renderer.height = 100, 100
	queries := 0
	current := uint32(100)
	baseQuery := renderer.api.getSurfaceCapabilities
	renderer.api.getSurfaceCapabilities = func(device, surface uintptr, caps *vkSurfaceCapabilities) int32 {
		queries++
		status := baseQuery(device, surface, caps)
		caps.Current.Width = current
		return status
	}
	state := &linuxWindowState{win: &window{id: 1, width: 100, height: 100}, renderer: renderer}
	// The server has already reached the size we drew; this queued intermediate
	// Configure must not leave its capabilities for a later OUT_OF_DATE recovery.
	event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, Width: 90, Height: 100}
	if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
		t.Fatalf("obsolete configure: closed=%v err=%v", closed, err)
	}
	current = 120
	if err := renderer.presentRebuilding(); err != nil {
		t.Fatal(err)
	}
	if queries != 2 || renderer.width != current || recreations != 1 {
		t.Fatalf("recovery used obsolete geometry: queries=%d width=%d recreations=%d", queries, renderer.width, recreations)
	}
}

func TestResizeSharesSurfaceQueryOnlyWithItsOwnSwapchain(t *testing.T) {
	recreations := 0
	renderer := newSuboptimalTestRenderer(vkSuccess, vkSuccess, &recreations)
	renderer.physical, renderer.surface = 1, 2
	renderer.width, renderer.height = 100, 100
	queries := 0
	baseQuery := renderer.api.getSurfaceCapabilities
	renderer.api.getSurfaceCapabilities = func(device, surface uintptr, caps *vkSurfaceCapabilities) int32 {
		queries++
		status := baseQuery(device, surface, caps)
		caps.Current.Width = 120
		return status
	}
	state := &linuxWindowState{win: &window{id: 1, width: 100, height: 100}, renderer: renderer}
	event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, Width: 120, Height: 100}
	if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
		t.Fatalf("resize: closed=%v err=%v", closed, err)
	}
	if queries != 1 || recreations != 1 || renderer.width != 120 || renderer.hasPendingCapabilities {
		t.Fatalf("resize queries=%d recreations=%d width=%d pending=%v", queries, recreations, renderer.width, renderer.hasPendingCapabilities)
	}
}
