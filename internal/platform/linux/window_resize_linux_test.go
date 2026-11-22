//go:build linux

package linux

import (
	"slices"
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// Moving a window changes no size: the renderer is not asked about its surface, not
// resized, and the window still reports where its content is.
func TestMovingWindowDoesNotResizeOrQueryTheSurface(t *testing.T) {
	var frames []gfx.Rect
	renderer := &fakeWindowRenderer{deviceWidth: 100, deviceHeight: 100}
	state := &linuxWindowState{
		win: &window{id: 1, width: 100, height: 100},
		config: platform.Config{
			OnFrameChange: func(frame gfx.Rect) { frames = append(frames, frame) },
		},
		renderer: renderer,
	}
	for x := int16(10); x < 30; x++ {
		event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, X: x, Y: 20, Width: 100, Height: 100}
		if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
			t.Fatalf("move: closed=%v err=%v", closed, err)
		}
	}
	if renderer.count("surfaceSize") != 0 || renderer.count("resize") != 0 || state.resizeEvents != 0 {
		t.Fatalf("pure movement asked about the surface %d times, resized %d times and reported %d resizes: %v",
			renderer.count("surfaceSize"), renderer.count("resize"), state.resizeEvents, renderer.calls)
	}
	if len(frames) != 20 || frames[len(frames)-1] != (gfx.Rect{X: 29, Y: 20, Width: 100, Height: 100}) {
		t.Fatalf("movement stopped reporting the content frame: %v", frames)
	}
}

// What a configure event made the renderer read about its surface is not kept for later:
// an obsolete event that needed no resize must not leave capabilities for a later
// present to rebuild the swapchain by.
func TestConfigureDoesNotLeaveSurfaceCapabilitiesBehind(t *testing.T) {
	renderer := &fakeWindowRenderer{deviceWidth: 100, deviceHeight: 100, serverSize: [2]uint32{100, 100}}
	state := &linuxWindowState{win: &window{id: 1, width: 100, height: 100}, renderer: renderer}
	// The server has already reached the size we drew: this queued intermediate
	// Configure needs no resize.
	event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, Width: 90, Height: 100}
	if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
		t.Fatalf("obsolete configure: closed=%v err=%v", closed, err)
	}
	if renderer.count("resize") != 0 {
		t.Fatalf("an obsolete configure resized the renderer: %v", renderer.calls)
	}
	if last := renderer.calls[len(renderer.calls)-1]; last != "forget" {
		t.Fatalf("the configure left the surface capabilities it read: %v", renderer.calls)
	}
}

// A resize reads the surface once and resizes the swapchain by that reading: the window
// asks the renderer for the surface's size, then resizes it, then presents.
func TestResizeAsksForTheSurfaceThenResizesThenPresents(t *testing.T) {
	renderer := &fakeWindowRenderer{deviceWidth: 100, deviceHeight: 100}
	state := &linuxWindowState{win: &window{id: 1, width: 100, height: 100}, renderer: renderer}
	event := xcbConfigureEvent{ResponseType: xcbConfigureNotify | 0x80, Window: 1, Width: 120, Height: 100}
	if closed, err := state.handleEvent(unsafe.Pointer(&event)); closed || err != nil {
		t.Fatalf("resize: closed=%v err=%v", closed, err)
	}
	surface, resize, present := slices.Index(renderer.calls, "surfaceSize"), slices.Index(renderer.calls, "resize"), slices.Index(renderer.calls, "present")
	if surface < 0 || !(surface < resize && resize < present) {
		t.Fatalf("a resize made the calls %v, want surfaceSize, resize, present in that order", renderer.calls)
	}
	if renderer.deviceWidth != 120 || state.resizeEvents != 1 {
		t.Fatalf("the renderer is %d wide after %d resize events", renderer.deviceWidth, state.resizeEvents)
	}
}
