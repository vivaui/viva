package windows

// Where a window that asked for a given origin lands: where it asked for when
// that is still usable, or centered in the current work area when it is not.

// placedFrame keeps a requested window frame inside the work area of the
// monitor it was headed for, in physical pixels. A frame that is already fully
// inside keeps its remembered position. A frame that no longer fits, because a
// monitor, resolution, or scale changed, is centered instead of being pushed
// against an edge; a frame larger than the work area is also shrunk to fit.
func placedFrame(work rect, x, y, width, height int32) (int32, int32, int32, int32) {
	width, height = min(width, work.Right-work.Left), min(height, work.Bottom-work.Top)
	if x < work.Left || y < work.Top || x+width > work.Right || y+height > work.Bottom {
		x = work.Left + (work.Right-work.Left-width)/2
		y = work.Top + (work.Bottom-work.Top-height)/2
	}
	return x, y, width, height
}
