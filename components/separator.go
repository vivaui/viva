package components

import "github.com/vivaui/viva/gfx"

// SeparatorOrientation selects the explicit official separator direction.
type SeparatorOrientation uint8

const (
	SeparatorHorizontal SeparatorOrientation = iota
	SeparatorVertical
)

// PaintSeparator paints one one-pixel divider centered in the supplied layout
// bounds. The bounds may therefore retain a larger hit-free layout gap.
func PaintSeparator(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, orientation SeparatorOrientation) {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	canvas.Container(bounds, gfx.RoleSeparator, "")
	if orientation == SeparatorVertical {
		x := bounds.X + (bounds.Width-1)/2
		canvas.Fill(gfx.Rect{X: x, Y: bounds.Y, Width: 1, Height: bounds.Height}, theme.Border)
		return
	}
	y := bounds.Y + (bounds.Height-1)/2
	canvas.Fill(gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: 1}, theme.Border)
}
