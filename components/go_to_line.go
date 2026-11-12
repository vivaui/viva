package components

import "github.com/vivaui/viva/gfx"

// GoToLineSnapshot is one caller-owned presentation of an editor's go to line
// bar: one input and the line count it takes a number from. Parsing "12" or
// "12:5" and moving the caret are the application's.
type GoToLineSnapshot struct {
	// Input is the field; its ID must be set. Its Placeholder tells the range,
	// such as "Line 1–240".
	Input InputSnapshot
	// Invalid paints the field invalid: the text is not a line that exists.
	Invalid bool
}

const goToLineWidth = float32(220)

// GoToLineHeight is how tall the bar is for the theme.
func GoToLineHeight(theme gfx.Theme) float32 {
	return findBarRow(theme) + 2*findBarPadding
}

// PaintGoToLine paints the bar at the top of bounds, as a floating surface like
// the find bar, 220 wide at most, and returns where the field is. Enter and
// Escape belong to the bar while its field has the focus.
func PaintGoToLine(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot GoToLineSnapshot) gfx.Rect {
	bounds.Width = min(bounds.Width, goToLineWidth)
	bounds.Height = GoToLineHeight(theme)
	if canvas == nil || bounds.Width <= 0 {
		return bounds
	}
	radius := theme.Metrics.RadiusControl
	canvas.Elevate(bounds, radius, gfx.ShadowMD)
	paintBordered(canvas, theme, bounds, radius, theme.Border, theme.Popover)
	field := gfx.Rect{X: bounds.X + findBarPadding, Y: bounds.Y + findBarPadding, Width: max(bounds.Width-2*findBarPadding, 0), Height: findBarRow(theme)}
	input := snapshot.Input
	input.Invalid = input.Invalid || snapshot.Invalid
	PaintInput(canvas, theme, field, input)
	return field
}
