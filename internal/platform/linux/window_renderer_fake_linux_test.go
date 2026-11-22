//go:build linux && !android

package linux

import (
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
)

// fakeWindowRenderer is a windowRenderer that records the calls a window makes, for the
// tests of what the window does with its renderer (the renderers have their own).
type fakeWindowRenderer struct {
	calls        []string
	width, scale float32
	deviceWidth  uint32
	deviceHeight uint32
	// serverSize is what surfaceSize reports: the size the surface has now.
	serverSize [2]uint32
	// failAt2x makes present fail while the scale is 2.
	failAt2x bool
	// shared, when set, also gets every call, to order them against what else a test records.
	shared *[]string
}

func (renderer *fakeWindowRenderer) record(call string) {
	renderer.calls = append(renderer.calls, call)
	if renderer.shared != nil {
		*renderer.shared = append(*renderer.shared, call)
	}
}

func (renderer *fakeWindowRenderer) apply(platform.Frame) (bool, error) {
	renderer.record("apply")
	return true, nil
}
func (renderer *fakeWindowRenderer) stage(platform.Frame) (bool, error) {
	renderer.record("stage")
	return true, nil
}
func (renderer *fakeWindowRenderer) present() error {
	renderer.record("present")
	if renderer.failAt2x && renderer.scale == 2 {
		return errFakeLoss
	}
	return nil
}
func (renderer *fakeWindowRenderer) presentFull() error { return renderer.present() }
func (renderer *fakeWindowRenderer) resize(width, height uint32) error {
	renderer.record("resize")
	renderer.deviceWidth, renderer.deviceHeight = width, height
	return nil
}
func (renderer *fakeWindowRenderer) setScale(scale float32) error {
	renderer.record("setScale")
	renderer.scale = scale
	return nil
}
func (renderer *fakeWindowRenderer) setFrameMargin(uint32) error { return nil }
func (renderer *fakeWindowRenderer) surfaceSize(width, height uint32) (uint32, uint32) {
	renderer.record("surfaceSize")
	if renderer.serverSize != [2]uint32{} {
		return renderer.serverSize[0], renderer.serverSize[1]
	}
	return width, height
}
func (renderer *fakeWindowRenderer) refreshText() error { return nil }
func (renderer *fakeWindowRenderer) forgetSurfaceCapabilities() {
	renderer.record("forget")
}
func (renderer *fakeWindowRenderer) alpha() bool         { return false }
func (renderer *fakeWindowRenderer) frameMargin() uint32 { return 0 }
func (renderer *fakeWindowRenderer) deviceSize() (uint32, uint32) {
	return renderer.deviceWidth, renderer.deviceHeight
}
func (renderer *fakeWindowRenderer) firstRectangle() (render.Rectangle, bool) {
	return render.Rectangle{}, false
}
func (renderer *fakeWindowRenderer) stats() rendererStats { return rendererStats{backend: "fake"} }
func (renderer *fakeWindowRenderer) close()               {}

// count is how many times call was made.
func (renderer *fakeWindowRenderer) count(call string) int {
	n := 0
	for _, made := range renderer.calls {
		if made == call {
			n++
		}
	}
	return n
}

type fakeLoss struct{}

func (fakeLoss) Error() string { return "fake: the device was lost" }

var errFakeLoss error = fakeLoss{}
