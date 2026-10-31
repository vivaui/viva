package linux

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

func TestResizingOnTheSameDisplayReusesMinimumHints(t *testing.T) {
	monitor := monitorGeometry{Width: 1920, Height: 1080, Primary: true}
	win := &window{
		tracker:     &scaleTracker{monitors: []monitorGeometry{monitor}},
		minimumSize: &minimumSizeCache{monitor: monitor, scale: 1, valid: true},
	}
	// No X connection is installed: a cache hit must not query or write the X
	// server, including as the client width and height change. Supplying the
	// position keeps this unit test on the ConfigureNotify fast path.
	for _, dimensions := range [][2]uint32{{1200, 800}, {960, 600}, {1400, 900}} {
		win.width, win.height = dimensions[0], dimensions[1]
		win.updateMinimumSizeAt(platform.Config{MinWidth: 960, MinHeight: 600}, 0, 0)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		win.updateMinimumSizeAt(platform.Config{MinWidth: 960, MinHeight: 600}, 0, 0)
	}); allocations != 0 {
		t.Fatalf("unchanged display allocated %g times", allocations)
	}
	t.Logf("optional minimum cache: %d bytes; per-window pointer: %d bytes", unsafe.Sizeof(minimumSizeCache{}), unsafe.Sizeof(win.minimumSize))
}

func TestNoMinimumKeepsTheNativeWindowUnconfigured(t *testing.T) {
	win := &window{}
	win.updateMinimumSizeAt(platform.Config{}, 0, 0)
	if win.minimumSize != nil {
		t.Fatal("window without a minimum allocated a cache")
	}
}

func TestMultiMonitorResizeReusesThePositionItAlreadyReported(t *testing.T) {
	left := monitorGeometry{Name: "left", Width: 1920, Height: 1080, Primary: true}
	right := monitorGeometry{Name: "right", X: 1920, Width: 1280, Height: 720}
	win := &window{
		tracker:     &scaleTracker{monitors: []monitorGeometry{left, right}},
		minimumSize: &minimumSizeCache{monitor: left, scale: 1, valid: true},
	}
	win.width, win.height = 1200, 800
	// No X connection is installed: with more than one monitor this must reuse
	// the position the ConfigureNotify pass reported instead of translating the
	// coordinates again.
	win.updateMinimumSizeAt(platform.Config{MinWidth: 960, MinHeight: 600}, 200, 100)
}
