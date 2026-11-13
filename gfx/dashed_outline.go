package gfx

// DashedRoundedOutline strokes a dashed line inside bounds without painting
// the interior, as RoundedOutline does solid. On the rectangle's whole
// perimeter the repetition is fitted so the pattern closes where it began: the
// dashes are one repeating rhythm around the outline, six parts paint to four
// parts gap, not a fixed pixel length whose remainder lands as a short stub.
func (canvas *Canvas) DashedRoundedOutline(bounds Rect, radius, width float32, color Color) {
	if width <= 0 || color.Alpha <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if canvas.pixelScale > 0 {
		bounds = canvas.snapBox(bounds, radius)
		width = canvas.borderWidth(width)
	}
	if bounds.Width <= width*2 || bounds.Height <= width*2 {
		// Too small to dash: the whole box is the line, as RoundedOutline has it.
		canvas.Rounded(bounds, radius, color)
		return
	}
	// The stroke's centerline runs half the width inside the outline, so the
	// painted line still sits entirely within bounds.
	centerline := Rect{X: bounds.X + width/2, Y: bounds.Y + width/2, Width: bounds.Width - width, Height: bounds.Height - width}
	radius = min(max(radius-width/2, 0), min(centerline.Width, centerline.Height)/2)
	canvas.pathPoints = appendRoundedRectContour(canvas.pathPoints[:0], centerline, radius)
	canvas.contourEnds = append(canvas.contourEnds[:0], int32(len(canvas.pathPoints)))
	canvas.contourClosed = append(canvas.contourClosed[:0], true)
	canvas.dashStrokeContours(color.Alpha >= 0.999, width/2, CapButt, JoinMiter, color)
}
