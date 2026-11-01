package windows

// Where a window that asked for a given origin lands: near where it asked for,
// but never so far off the screen that its user cannot reach it.

const (
	// visibleTitleWidth and visibleTitleHeight are how much of a window (logical
	// pixels) a remembered origin keeps on its monitor at the least: the caption
	// buttons and a strip of title bar beside them, which the user can drag the
	// window back by. The application draws its own frame, so a narrower sliver —
	// the buttons alone — shows a window that nothing can drag, and only its
	// maximize button works.
	visibleTitleWidth  = 240
	visibleTitleHeight = 48
)

// placedOrigin keeps a requested window frame's top-left inside the work area of
// the monitor it was headed for, physical pixels in and out. A position
// remembered on a monitor that is gone, or under another scale, would otherwise
// strand the window where only its buttons show. Most of the window may still
// sit off the screen: what is guaranteed is a strip wide and tall enough to
// grab.
func placedOrigin(work rect, scale float32, x, y, width, height int32) (int32, int32) {
	needWidth := min(int32(float32(visibleTitleWidth)*scale), width)
	needHeight := min(int32(float32(visibleTitleHeight)*scale), height)
	x = min(max(x, work.Left-width+needWidth), work.Right-needWidth)
	y = min(max(y, work.Top), work.Bottom-needHeight)
	return x, y
}
