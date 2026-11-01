package windows

import "testing"

// A remembered origin on a monitor that is gone, or under another scale, must
// still leave a strip of the window on the screen that the user can grab. The
// report that found it was a window whose whole title bar sat off the screen,
// so nothing could drag it and only the maximize button worked.
func TestPlacedFrameKeepsAGrabableStripOnTheScreen(t *testing.T) {
	work := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	visibleWidth := func(x, width int32) int32 { return min(x+width, work.Right) - max(x, work.Left) }
	visibleHeight := func(y, height int32) int32 { return min(y+height, work.Bottom) - max(y, work.Top) }

	// A window remembered left of a monitor that is no longer there.
	const scale = 1.25
	x, y, _, _ := placedFrame(work, scale, -1600, 100, 1750, 1125)
	if visible, want := visibleWidth(x, 1750), int32(float32(visibleTitleWidth)*scale); visible < want {
		t.Fatalf("x = %d leaves %dpx visible, want at least the %dpx strip", x, visible, want)
	}
	if y != 100 {
		t.Fatalf("y = %d, want the requested one: the window was on the screen", y)
	}
	// A position on the screen stays as it was asked.
	if x, y, w, h := placedFrame(work, 1, 100, 100, 1400, 900); x != 100 || y != 100 || w != 1400 || h != 900 {
		t.Fatalf("an on-screen frame moved to %d,%d %dx%d", x, y, w, h)
	}
	// One beyond the right edge is pulled back the same way.
	x, _, _, _ = placedFrame(work, 1, 1900, 100, 1400, 900)
	if visible := visibleWidth(x, 1400); visible < visibleTitleWidth {
		t.Fatalf("x = %d leaves %dpx visible, want at least %d", x, visible, visibleTitleWidth)
	}
	// A position below the taskbar is raised back.
	_, y, _, _ = placedFrame(work, 1, 100, 1200, 1400, 900)
	if visible := visibleHeight(y, 900); visible < visibleTitleHeight {
		t.Fatalf("y = %d leaves %dpx of the title strip visible, want at least %d", y, visible, visibleTitleHeight)
	}
}

// A window remembered on a larger monitor must come back whole on a smaller
// one, caption buttons included, rather than reaching past its right edge.
func TestPlacedFrameShrinksAWindowThatDoesNotFit(t *testing.T) {
	work := rect{Left: 0, Top: 0, Right: 1366, Bottom: 728}
	x, y, width, height := placedFrame(work, 1, 0, 0, 2000, 1200)
	if x < work.Left || y < work.Top || x+width > work.Right || y+height > work.Bottom {
		t.Fatalf("frame %d,%d %dx%d is not inside the work area", x, y, width, height)
	}
}
