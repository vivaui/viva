package windows

import "testing"

func frameInside(work rect, x, y, width, height int32) bool {
	return x >= work.Left && y >= work.Top && x+width <= work.Right && y+height <= work.Bottom
}

// A remembered origin on a monitor that is gone must bring the whole window
// back onto the current display. Center it rather than leaving it against the
// first edge: a stale placement should not look like a new upper-left default.
func TestPlacedFrameCentersARememberedWindowOutsideTheScreen(t *testing.T) {
	work := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}

	// A window remembered left of a monitor that is no longer there.
	x, y, width, height := placedFrame(work, -1600, 100, 1750, 1125)
	if !frameInside(work, x, y, width, height) {
		t.Fatalf("remembered frame %d,%d %dx%d is outside %+v", x, y, width, height, work)
	}
	if x != 85 || y != 0 {
		t.Fatalf("remembered frame centered at %d,%d, want 85,0", x, y)
	}

	// A position on the screen stays as it was asked.
	if x, y, w, h := placedFrame(work, 100, 100, 1400, 900); x != 100 || y != 100 || w != 1400 || h != 900 {
		t.Fatalf("an on-screen frame moved to %d,%d %dx%d", x, y, w, h)
	}

	// Frames beyond every edge are centered in the work area.
	for _, requested := range []struct {
		x, y, width, height int32
	}{
		{x: 1900, y: 100, width: 1400, height: 900},
		{x: 100, y: 1200, width: 1400, height: 900},
		{x: -1600, y: -500, width: 1400, height: 900},
	} {
		x, y, width, height := placedFrame(work, requested.x, requested.y, requested.width, requested.height)
		if !frameInside(work, x, y, width, height) {
			t.Fatalf("requested frame %+v landed at %d,%d %dx%d outside %+v", requested, x, y, width, height, work)
		}
		if x != 260 || y != 70 {
			t.Fatalf("requested frame %+v centered at %d,%d, want 260,70", requested, x, y)
		}
	}
}

// A window remembered on a larger monitor must come back whole on a smaller
// one, caption buttons included, rather than reaching past its right edge.
func TestPlacedFrameShrinksAWindowThatDoesNotFit(t *testing.T) {
	work := rect{Left: 0, Top: 0, Right: 1366, Bottom: 728}
	x, y, width, height := placedFrame(work, 0, 0, 2000, 1200)
	if !frameInside(work, x, y, width, height) {
		t.Fatalf("frame %d,%d %dx%d is not inside the work area", x, y, width, height)
	}
	if x != 0 || y != 0 {
		t.Fatalf("oversized frame centered at %d,%d, want 0,0", x, y)
	}
}

// A monitor to the left of the primary one has negative coordinates. Centering
// there must preserve that monitor's own work area rather than pull it to zero.
func TestPlacedFrameKeepsAWindowOnALeftMonitor(t *testing.T) {
	work := rect{Left: -1920, Top: 0, Right: 0, Bottom: 1040}
	x, y, width, height := placedFrame(work, -5000, -200, 1400, 900)
	if !frameInside(work, x, y, width, height) {
		t.Fatalf("frame %d,%d %dx%d is outside %+v", x, y, width, height, work)
	}
	if x != -1660 || y != 70 {
		t.Fatalf("frame centered at %d,%d, want -1660,70", x, y)
	}
}
