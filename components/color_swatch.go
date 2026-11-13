package components

import "github.com/vivaui/viva/gfx"

// ColorSwatch is a circular color choice. The caller owns its selection and
// supplies a stable ID and accessible name. A transparent color means no color.
type ColorSwatch struct {
	canvas             *gfx.Canvas
	theme              gfx.Theme
	id                 gfx.ID
	bounds             gfx.Rect
	color              gfx.Color
	label              string
	selected, disabled bool
	skipTab            bool
	flush              bool
}

// BeginColorSwatch centers a color disc inside bounds, leaving room for its
// selection and keyboard focus rings. The full bounds are the hit target.
func BeginColorSwatch(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, color gfx.Color) ColorSwatch {
	return ColorSwatch{canvas: canvas, theme: theme, id: id, bounds: bounds, color: color, label: "Color"}
}

func (s ColorSwatch) Label(label string) ColorSwatch     { s.label = label; return s }
func (s ColorSwatch) Selected(selected bool) ColorSwatch { s.selected = selected; return s }
func (s ColorSwatch) Disabled() ColorSwatch              { s.disabled = true; return s }

// Flush paints the colour disc against the border instead of leaving a
// background-coloured ring between them. A swatch in a dense row then reads as
// one solid colour inside a hairline circle rather than a smaller disc with a
// visible gap.
func (s ColorSwatch) Flush() ColorSwatch { s.flush = true; return s }

// TabStop says whether Tab may enter this choice. A radio set gives its
// selected choice the tab stop, marks the others false, and uses
// RadioGroupKeyDown for arrow navigation.
func (s ColorSwatch) TabStop(tabStop bool) ColorSwatch { s.skipTab = !tabStop; return s }

type ColorSwatchResult struct {
	canvas   *gfx.Canvas
	id       gfx.ID
	disabled bool
}

func (r ColorSwatchResult) Clicked() bool {
	return !r.disabled && r.canvas != nil && r.canvas.Clicked(r.id)
}

// Paint declares a keyboard-accessible radio choice. Its ring communicates
// selection independently of the color, including the no-color choice.
func (s ColorSwatch) Paint() ColorSwatchResult {
	r := ColorSwatchResult{s.canvas, s.id, s.disabled}
	if s.canvas == nil {
		return r
	}
	side := min(s.bounds.Width, s.bounds.Height)
	if side <= 10 {
		return r
	}
	b := gfx.Rect{X: s.bounds.X + (s.bounds.Width-side)/2, Y: s.bounds.Y + (s.bounds.Height-side)/2, Width: side, Height: side}
	border := s.theme.Input
	if s.selected {
		border = s.theme.Foreground
	}
	if !s.disabled && (s.canvas.FocusVisible() == s.id || s.canvas.Hovered() == s.id) && !s.id.IsZero() {
		border = s.theme.Ring
	}
	if s.disabled {
		border.Alpha *= 0.4
	}
	s.canvas.Rounded(b, side/2, border)
	var disc gfx.Rect
	if s.flush {
		disc = gfx.Rect{X: b.X + 1, Y: b.Y + 1, Width: max(side-2, 0), Height: max(side-2, 0)}
	} else {
		inner := gfx.Rect{X: b.X + 2, Y: b.Y + 2, Width: side - 4, Height: side - 4}
		s.canvas.Rounded(inner, inner.Width/2, s.theme.Background)
		disc = gfx.Rect{X: b.X + 5, Y: b.Y + 5, Width: side - 10, Height: side - 10}
	}
	color := s.color
	if color.Alpha == 0 {
		color = s.theme.Muted
	}
	if s.disabled {
		color.Alpha *= 0.4
	}
	s.canvas.Rounded(disc, disc.Width/2, color)
	if s.color.Alpha == 0 {
		s.canvas.DrawIcon(noColorIcon, disc, s.theme.MutedForeground)
	}
	if !s.id.IsZero() {
		s.canvas.HitWithOptions(s.id, s.bounds, s.label, gfx.HitOptions{
			Role: gfx.RoleRadio, Checked: s.selected, Selected: s.selected,
			Disabled: s.disabled, SkipTab: s.skipTab,
		})
	}
	return r
}

var noColorIcon = gfx.Icon{ViewBox: 24, Stroke: 1.5, Cap: gfx.CapRound, Commands: []gfx.PathCommand{gfx.MoveTo(6, 18), gfx.LineTo(18, 6)}}
