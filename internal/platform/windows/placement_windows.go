package windows

// Where a window that asked for a given origin lands: near where it asked for,
// but never so far off the screen that its user cannot reach it.

const (
	// visibleTitleWidth and visibleTitleHeight are how much of a window (logical
	// pixels) a remembered origin keeps on its monitor at the least: a strip of
	// title bar the user can drag the window back by. The application draws its
	// own frame, so a narrower sliver shows a window that nothing can drag. The
	// caption buttons sit at the right end of the title bar; they stay in view
	// whenever the window is placed left of where it asked for, or fits the work
	// area, but a window pushed back from beyond the right edge shows its left
	// end, where the strip is, instead.
	visibleTitleWidth  = 240
	visibleTitleHeight = 48
)

// placedFrame keeps a requested window frame inside the work area of the
// monitor it was headed for, physical pixels in and out. A window larger than
// the work area is shrunk to it, as placeOnScreen does, so all of it and its
// caption buttons are reachable. A position remembered on a monitor that is
// gone would otherwise strand the window where nothing of it shows; the origin
// is pulled back until a strip wide and tall enough to grab is on the screen,
// though most of the window may still sit off it.
//
// scale is the one of the window's current monitor, not of the monitor the
// origin was remembered on, so a mixed-scale setup can still land the frame on
// another monitor than the one it was left on; this only keeps the result
// reachable.
func placedFrame(work rect, scale float32, x, y, width, height int32) (int32, int32, int32, int32) {
	width, height = min(width, work.Right-work.Left), min(height, work.Bottom-work.Top)
	needWidth := min(int32(float32(visibleTitleWidth)*scale), width)
	needHeight := min(int32(float32(visibleTitleHeight)*scale), height)
	x = min(max(x, work.Left-width+needWidth), work.Right-needWidth)
	y = min(max(y, work.Top), work.Bottom-needHeight)
	return x, y, width, height
}
