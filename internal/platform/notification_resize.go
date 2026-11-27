package platform

// ResizeNotificationFrame computes the next outer rectangle for an unanchored
// WindowNotification changing size to width x height inside work. It keeps the
// nearest horizontal and vertical edges pinned so a bottom-right popup shrinks
// and expands in place instead of floating away from the screen corner, then
// clamps the result into work.
func ResizeNotificationFrame(current, work ScreenRect, width, height float32) ScreenRect {
	if work.Width > 0 && width > work.Width {
		width = work.Width
	}
	if work.Height > 0 && height > work.Height {
		height = work.Height
	}
	x := current.X
	if current.X+current.Width/2 >= work.X+work.Width/2 {
		x = current.X + current.Width - width
	}
	y := current.Y
	if current.Y+current.Height/2 >= work.Y+work.Height/2 {
		y = current.Y + current.Height - height
	}
	x, y = RestoreNotification(x, y, width, height, work)
	return ScreenRect{X: x, Y: y, Width: width, Height: height}
}
