package gallery

import "github.com/vivaui/viva/gfx"

func paintMark(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) {
	// Two cells and the gap between them span the full bounds.
	unit := bounds.Width / 2.3
	gap := unit * 0.3
	canvas.Rounded(gfx.Rect{X: bounds.X, Y: bounds.Y, Width: unit, Height: unit}, 2, theme.Foreground)
	canvas.Rounded(gfx.Rect{X: bounds.X + unit + gap, Y: bounds.Y, Width: unit, Height: unit}, 2, theme.Foreground)
	canvas.Rounded(gfx.Rect{X: bounds.X, Y: bounds.Y + unit + gap, Width: unit, Height: unit}, 2, theme.Foreground)
	canvas.Rounded(gfx.Rect{X: bounds.X + unit + gap, Y: bounds.Y + unit + gap, Width: unit, Height: unit}, unit/2, theme.Primary)
}

// Lucide chevron-left and chevron-right, hand-compiled to the gfx.Icon form.
var (
	iconChevronLeft = gfx.Icon{
		ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(15, 18), gfx.LineTo(9, 12), gfx.LineTo(15, 6)},
	}
	iconChevronRight = gfx.Icon{
		ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound,
		Commands: []gfx.PathCommand{gfx.MoveTo(9, 18), gfx.LineTo(15, 12), gfx.LineTo(9, 6)},
	}
)

// paintIconButton paints a ghost icon button: the hover surface, the icon
// centered exactly in its bounds, and the labelled hit region. A text glyph
// would sit wherever the font puts it; an icon is centered by construction. It
// reports whether this paint is delivering a click on the button.
func paintIconButton(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, icon gfx.Icon, label string) bool {
	if canvas.Hovered() == id || canvas.Pressed() == id {
		canvas.Rounded(bounds, theme.Metrics.RadiusControl, theme.Accent)
	}
	const size = float32(16)
	canvas.DrawIcon(icon, gfx.Rect{
		X: bounds.X + (bounds.Width-size)/2, Y: bounds.Y + (bounds.Height-size)/2, Width: size, Height: size,
	}, theme.Foreground)
	canvas.HitWithOptions(id, bounds, label, gfx.HitOptions{Role: gfx.RoleButton})
	return canvas.Clicked(id)
}
