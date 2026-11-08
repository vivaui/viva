package components

import "github.com/vivaui/viva/ui"

// BadgeVariant selects one of the shadcn Vega badge treatments. Destructive is
// Vega's: a danger-tinted pill with a danger label, not a solid red one.
type BadgeVariant uint8

const (
	BadgeDefault BadgeVariant = iota
	BadgeSecondary
	BadgeDestructive
	BadgeOutline
	BadgeGhost
	BadgeLink
)

// BadgeExtent is the size PaintBadge gives a badge with the label: as wide as its
// text plus padding (px-2 and the 1px transparent border Vega gives every
// variant), so a layout can reserve room for it (a flex Slot) before it is
// painted.
func BadgeExtent(canvas *ui.Canvas, theme ui.Theme, label string) (width, height float32) {
	scale := theme.Metrics.ControlHeight / 32
	return canvas.TextWidthWeight(label, theme.Metrics.FontSizeS, ui.WeightMedium) + 18*scale, 20 * scale
}

// PaintBadge paints a shadcn Vega badge variant. Badges are
// presentational; callers that need a link or button should declare that
// interaction separately so its action and accessible name stay explicit.
func PaintBadge(canvas *ui.Canvas, theme ui.Theme, origin ui.Rect, label string, variant BadgeVariant) ui.Rect {
	if canvas == nil {
		return ui.Rect{}
	}
	width, height := BadgeExtent(canvas, theme, label)
	bounds := ui.Rect{X: origin.X, Y: origin.Y, Width: width, Height: height}
	background := theme.Primary
	foreground := theme.PrimaryForeground
	border := ui.Color{}
	paintBorder := false
	paintBackground := true
	switch variant {
	case BadgeSecondary:
		background = theme.Secondary
		foreground = theme.SecondaryForeground
	case BadgeDestructive:
		background = withAlpha(theme.Danger, 0.10)
		if isDark(&theme) {
			background.Alpha = 0.20
		}
		foreground = theme.Danger
	case BadgeOutline:
		background = theme.Background
		background.Alpha = 0
		foreground = theme.Foreground
		border = theme.Border
		paintBorder = true
	case BadgeGhost:
		paintBackground = false
		foreground = theme.Foreground
	case BadgeLink:
		paintBackground = false
		foreground = theme.Primary
	}

	radius := bounds.Height / 2
	if paintBorder {
		paintBordered(canvas, theme, bounds, radius, border, background)
	} else if paintBackground {
		canvas.Rounded(bounds, radius, background)
	}
	canvas.LabelCenter(label, bounds, ui.TextStyle{
		Size: theme.Metrics.FontSizeS, Weight: ui.WeightMedium, Color: foreground,
	})
	return bounds
}
