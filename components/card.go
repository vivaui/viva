package components

import "github.com/vivaui/viva/gfx"

// CardSize selects the official default or compact card treatment.
type CardSize uint8

const (
	CardSizeDefault CardSize = iota
	CardSizeSmall
)

// CardOptions describes the sections of a card. All sizes are content heights
// in logical pixels; a zero height leaves the section out. The card supplies the
// padding around and between them (--card-spacing, which Size selects).
type CardOptions struct {
	Size CardSize
	// HeaderHeight and FooterHeight are the heights the caller needs for their
	// content; the content section takes whatever is left.
	HeaderHeight float32
	FooterHeight float32
	// HeaderBorder draws the header's border-b with a spacing of padding under
	// its content; FooterBorder draws the footer's border-t with padding above.
	HeaderBorder bool
	FooterBorder bool
}

// CardLayout is where the sections of a painted card are. Each rectangle is the
// content box, already inset by the card's horizontal spacing. A section that
// was not asked for is the zero rectangle.
type CardLayout struct {
	Header  gfx.Rect
	Content gfx.Rect
	Footer  gfx.Rect
}

// PaintCard paints a shadcn card surface and returns the layout of its sections:
// py spacing above and below, gap spacing between sections, px spacing at the
// sides, and the optional header and footer rules. The surface is the same at
// both sizes; Size changes only the spacing.
func PaintCard(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options CardOptions) CardLayout {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return CardLayout{}
	}
	radius := theme.Metrics.RadiusSurface + 4
	ring := theme.Foreground
	ring.Alpha *= 0.10
	canvas.Elevate(bounds, radius, gfx.ShadowXS) // shadow-xs
	canvas.RoundedBorder(bounds, radius, 1, ring, theme.Card)

	spacing := CardSpacing(theme, options.Size)
	width := max(bounds.Width-spacing*2, 0)
	x := bounds.X + spacing
	var layout CardLayout
	top, bottom := bounds.Y+spacing, bounds.Y+bounds.Height-spacing
	if options.HeaderHeight > 0 {
		layout.Header = gfx.Rect{X: x, Y: top, Width: width, Height: options.HeaderHeight}
		top += options.HeaderHeight
		if options.HeaderBorder {
			top += spacing
			canvas.Fill(gfx.Rect{X: bounds.X + 1, Y: top, Width: max(bounds.Width-2, 0), Height: 1}, theme.Border)
			top++
		}
		top += spacing
	}
	if options.FooterHeight > 0 {
		layout.Footer = gfx.Rect{X: x, Y: bottom - options.FooterHeight, Width: width, Height: options.FooterHeight}
		bottom = layout.Footer.Y
		if options.FooterBorder {
			bottom -= spacing + 1
			canvas.Fill(gfx.Rect{X: bounds.X + 1, Y: bottom, Width: max(bounds.Width-2, 0), Height: 1}, theme.Border)
		}
		bottom -= spacing
	}
	layout.Content = gfx.Rect{X: x, Y: top, Width: width, Height: max(bottom-top, 0)}
	return layout
}

// CardSpacing returns the official content spacing for a card size.
func CardSpacing(theme gfx.Theme, size CardSize) float32 {
	if size == CardSizeSmall {
		return 16 * theme.Metrics.ControlHeight / 32
	}
	return 24 * theme.Metrics.ControlHeight / 32
}

// CardTitleStyle is CardTitle: text-base medium, text-sm in a small card.
func CardTitleStyle(theme gfx.Theme, size CardSize) gfx.TextStyle {
	fontSize := theme.Metrics.FontSizeM + 2
	if size == CardSizeSmall {
		fontSize = theme.Metrics.FontSizeM
	}
	return gfx.TextStyle{Size: fontSize, Weight: gfx.WeightMedium, Color: theme.CardForeground}
}

// CardDescriptionStyle is CardDescription: text-sm in the muted foreground.
func CardDescriptionStyle(theme gfx.Theme) gfx.TextStyle {
	return gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground}
}

// CardHeaderHeight is the height of a header holding a title line and, when
// hasDescription, a description line under it with the header's gap-1.
func CardHeaderHeight(theme gfx.Theme, size CardSize, hasDescription bool) float32 {
	height := CardTitleStyle(theme, size).Size * 1.5 // leading-normal
	if hasDescription {
		height += theme.Metrics.GapItem/2 + theme.Metrics.FontSizeM + 7
	}
	return height
}

// Card paints the official default card surface.
func Card(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) {
	PaintCard(canvas, theme, bounds, CardOptions{})
}
