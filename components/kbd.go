package components

import "github.com/vivaui/viva/gfx"

// Kbd paints one compact keyboard keycap. Zero width or height selects a
// content-fitted dimension; the returned rectangle is the actual painted size.
func Kbd(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, label string) gfx.Rect {
	metrics := theme.Metrics
	if bounds.Height <= 0 {
		bounds.Height = metrics.FontSizeS + 9
	}
	if bounds.Width <= 0 {
		bounds.Width = canvas.TextWidthWeight(label, metrics.FontSizeS, gfx.WeightMedium) + 8
		if bounds.Width < bounds.Height {
			bounds.Width = bounds.Height
		}
	}
	radius := kbdRadius(theme)
	// Unlike shadcn's, a keycap has a hairline: it keeps one readable on the
	// muted fill of a menu or popover, where the fill alone is close to the
	// surface behind it.
	canvas.RoundedBorder(bounds, radius, 1, theme.Border, theme.Muted)
	canvas.LabelCenter(label, bounds, gfx.TextStyle{
		Size: metrics.FontSizeS, Weight: gfx.WeightMedium, Color: theme.MutedForeground,
	})
	return bounds
}

// kbdRadius is a keycap's corner radius. rounded-sm is three quarters of
// rounded-md in shadcn's radius scale (0.6 of the base radius against 0.8), and
// RadiusControl is the md.
func kbdRadius(theme gfx.Theme) float32 { return theme.Metrics.RadiusControl * 0.75 }

// KbdGroup incrementally paints keycaps with the source recipe's compact gap.
type KbdGroup struct {
	canvas *gfx.Canvas
	theme  gfx.Theme
	x      float32
	y      float32
	height float32
}

// BeginKbdGroup starts an allocation-free keycap row.
func BeginKbdGroup(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) KbdGroup {
	return KbdGroup{canvas: canvas, theme: theme, x: bounds.X, y: bounds.Y, height: bounds.Height}
}

// Add paints one keycap and advances by the Vega one-space gap.
func (group *KbdGroup) Add(label string) gfx.Rect {
	bounds := Kbd(group.canvas, group.theme, gfx.Rect{X: group.x, Y: group.y, Height: group.height}, label)
	group.x += bounds.Width + 4
	return bounds
}

const menuKeysGap = float32(4)

// menuKbdTheme is the theme a menu's keycaps are painted with: a point smaller
// than a standalone Kbd, which a row would otherwise be crowded by, and 6px
// taller than its text instead of 9.
func menuKbdTheme(theme gfx.Theme) gfx.Theme {
	theme.Metrics.FontSizeS = max(theme.Metrics.FontSizeS-1, 10)
	return theme
}

// menuKbdHeight is the height of a menu's keycaps, in theme made by menuKbdTheme.
func menuKbdHeight(theme gfx.Theme) float32 { return theme.Metrics.FontSizeS + 6 }

// menuKeysWidth is the width of a row of keycaps as paintMenuKeys draws it.
func menuKeysWidth(canvas *gfx.Canvas, theme gfx.Theme, keys []string) float32 {
	theme = menuKbdTheme(theme)
	height := menuKbdHeight(theme)
	var width float32
	for index, key := range keys {
		width += max(canvas.TextWidthWeight(key, theme.Metrics.FontSizeS, gfx.WeightMedium)+8, height)
		if index > 0 {
			width += menuKeysGap
		}
	}
	return width
}

// paintMenuKeys draws keys as keycaps, vertically centered in bounds and
// dimmed when the row is disabled.
func paintMenuKeys(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, keys []string, disabled bool) {
	if disabled {
		theme.Muted.Alpha *= 0.5
		theme.MutedForeground.Alpha *= 0.5
	}
	theme = menuKbdTheme(theme)
	height := menuKbdHeight(theme)
	group := BeginKbdGroup(canvas, theme, gfx.Rect{X: bounds.X, Y: bounds.Y + (bounds.Height-height)/2, Height: height})
	for _, key := range keys {
		group.Add(key)
	}
}
