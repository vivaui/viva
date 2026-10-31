package ui

// RoundedOutline strokes inside bounds without painting the interior. It is
// useful over translucent fills, where a solid border backing would show through.
func (canvas *Canvas) RoundedOutline(bounds Rect, radius, width float32, color Color) {
	if width <= 0 || color.Alpha <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if canvas.pixelScale > 0 {
		bounds = canvas.snapBox(bounds, radius)
		width = canvas.borderWidth(width)
	}
	if bounds.Width <= width*2 || bounds.Height <= width*2 {
		canvas.Rounded(bounds, radius, color)
		return
	}
	half := width / 2
	r := max(min(radius, bounds.Width/2, bounds.Height/2)-half, 0)
	left, top, right, bottom := half, half, bounds.Width-half, bounds.Height-half
	const k = float32(0.55228475)
	arc := r * k
	commands := [...]PathCommand{
		MoveTo(left+r, top), LineTo(right-r, top),
		CubicTo(right-r+arc, top, right, top+r-arc, right, top+r),
		LineTo(right, bottom-r),
		CubicTo(right, bottom-r+arc, right-r+arc, bottom, right-r, bottom),
		LineTo(left+r, bottom),
		CubicTo(left+r-arc, bottom, left, bottom-r+arc, left, bottom-r),
		LineTo(left, top+r),
		CubicTo(left, top+r-arc, left+r-arc, top, left+r, top), ClosePath(),
	}
	canvas.DrawPath(Path{width: bounds.Width, height: bounds.Height, commands: commands[:]}, bounds,
		PathStyle{Stroke: color, StrokeWidth: width, Join: JoinMiter})
}
