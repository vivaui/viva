package windows

import "testing"

// A remembered origin on a monitor that is gone, or under another scale, must
// still leave a strip of the window on the screen that the user can grab: the
// caption buttons and a strip of title bar beside them. The report that found
// it was a window whose whole title bar sat off the screen, so nothing could
// drag it and only the maximize button worked.
func TestPlacedOriginKeepsAGrabableStripOnTheScreen(t *testing.T) {
	work := rect{Left: 0, Top: 0, Right: 1920, Bottom: 1040}
	const scale = 1.25
	// A window remembered left of a monitor that is no longer there.
	x, y := placedOrigin(work, scale, -1600, 100, 1750, 1125)
	if visible := min(x+1750, work.Right) - max(x, work.Left); visible < int32(float32(visibleTitleWidth)*scale) {
		t.Fatalf("x = %d leaves %dpx visible, want at least the %dpx strip", x, visible, int32(float32(visibleTitleWidth)*scale))
	}
	if y != 100 {
		t.Fatalf("y = %d, want the requested one: the window was on the screen", y)
	}
	// A position on the screen stays as it was asked.
	if x, y := placedOrigin(work, 1, 100, 100, 1400, 900); x != 100 || y != 100 {
		t.Fatalf("an on-screen origin moved to %d,%d", x, y)
	}
	// One beyond the right edge is pulled back the same way.
	x, _ = placedOrigin(work, 1, 1900, 100, 1400, 900)
	if visible := min(x+1400, work.Right) - max(x, work.Left); visible < visibleTitleWidth {
		t.Fatalf("x = %d leaves %dpx visible, want at least %d", x, visible, visibleTitleWidth)
	}
	// A window wider than the work area centers its overflow rather than being
	// pushed off the left.
	x, _ = placedOrigin(work, 1, -4000, 100, 4000, 900)
	if min(x+4000, work.Right)-max(x, work.Left) < visibleTitleWidth {
		t.Fatalf("x = %d leaves the wide window unreachably far out", x)
	}
	// A position below the taskbar is raised back.
	_, y = placedOrigin(work, 1, 100, 1200, 1400, 900)
	if min(y+900, work.Bottom)-max(y, work.Top) < visibleTitleHeight {
		t.Fatalf("y = %d leaves less than the title strip visible", y)
	}
}
