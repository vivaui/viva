package style

import "github.com/vivaui/viva/gfx"

type InputTextStyle struct {
	// Size is the font size in points; zero is the theme's body size.
	Size float32
	// Weight is the font weight, 100 to 900 as in gfx.TextStyle; zero is regular.
	Weight float32
	// Color is the color of the text; the zero color, which is transparent, is the theme's
	// foreground.
	Color gfx.Color
	// Mono sets the text in a fixed-pitch face, MonoFace choosing which (gfx.TextStyle's own).
	// The platform's text service draws one proportional face, so a mono field is painted
	// by the component: its glyphs, selection, composition underline and caret are drawn in
	// the field, and the platform is kept for what only it does (typing, the input method
	// and the clipboard) with its own text and overlays hidden. The snapshot must then say
	// where the caret, the selection and the composition are (Caret, SelectionStart and so
	// on), and may give the caret's blink phase in Blink.
	Mono     bool
	MonoFace gfx.MonoFace
}

// TextStyle is the gfx.TextStyle the field's text is drawn and measured in under a theme: the zero
// size, weight and color are the theme's body size, regular weight and foreground.
func (s InputTextStyle) TextStyle(theme *gfx.Theme) gfx.TextStyle {
	ts := gfx.TextStyle{Size: s.Size, Weight: s.Weight, Color: s.Color, Mono: s.Mono, MonoFace: s.MonoFace}
	if ts.Size <= 0 {
		ts.Size = theme.Metrics.FontSizeM
	}
	if ts.Weight <= 0 {
		ts.Weight = gfx.WeightRegular
	}
	if ts.Color == (gfx.Color{}) {
		ts.Color = theme.Foreground
	}
	return ts
}
