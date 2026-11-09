package components

import "github.com/vivaui/viva/gfx"

// Collapsible paints one unstyled disclosure trigger. The official primitive
// intentionally adds no surface or icon; callers compose those explicitly.
// The application owns open state and paints content into the returned bounds.
type Collapsible struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	id       gfx.ID
	bounds   gfx.Rect
	disabled bool
}

// BeginCollapsible starts a disclosure container declaration.
func BeginCollapsible(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect) Collapsible {
	return Collapsible{canvas: canvas, theme: theme, id: id, bounds: bounds}
}

// Disabled renders and routes the trigger as inert.
func (collapsible Collapsible) Disabled() Collapsible {
	collapsible.disabled = true
	return collapsible
}

// Paint draws the trigger label. contentHeight is reserved only while open;
// the returned rectangle is empty while closed.
func (collapsible Collapsible) Paint(title string, open bool, contentHeight float32) gfx.Rect {
	header, content := collapsible.Bare(title, open, contentHeight)
	if header == (gfx.Rect{}) {
		return content
	}
	foreground := collapsible.theme.Foreground
	if collapsible.disabled {
		foreground.Alpha *= 0.50
	}
	collapsible.canvas.Label(title, header,
		gfx.TextStyle{Size: collapsible.theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground})
	return content
}

// Bare is CollapsibleTrigger without a look of its own (asChild): it reserves
// the trigger row, declares its hit with label and the expanded state, and paints
// nothing, so the caller draws any trigger it likes (a ghost icon button beside
// a heading, say) into the returned header. contentHeight is reserved only while
// open; content is empty while closed.
func (collapsible Collapsible) Bare(label string, open bool, contentHeight float32) (header, content gfx.Rect) {
	if collapsible.canvas == nil || collapsible.bounds.Width <= 0 {
		return gfx.Rect{}, gfx.Rect{}
	}
	if contentHeight < 0 {
		contentHeight = 0
	}
	headerHeight := collapsible.theme.Metrics.ControlHeight
	totalHeight := headerHeight
	if open {
		totalHeight += contentHeight
	}
	if collapsible.bounds.Height <= 0 || collapsible.bounds.Height < totalHeight {
		collapsible.bounds.Height = totalHeight
	}
	header = gfx.Rect{X: collapsible.bounds.X, Y: collapsible.bounds.Y, Width: collapsible.bounds.Width, Height: headerHeight}
	if !collapsible.id.IsZero() {
		collapsible.canvas.HitWithOptions(collapsible.id, header, label, gfx.HitOptions{
			Role: gfx.RoleButton, Disabled: collapsible.disabled, FocusableWhenDisabled: true, Expanded: open, Expandable: true,
		})
	}
	content = gfx.Rect{
		X:     collapsible.bounds.X,
		Y:     collapsible.bounds.Y + headerHeight,
		Width: collapsible.bounds.Width,
	}
	if open {
		content.Height = collapsible.bounds.Height - headerHeight
	}
	return header, content
}
