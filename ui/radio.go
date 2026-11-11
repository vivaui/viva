package ui

import (
	"github.com/vivaui/viva/gfx"
)

// radioDiameter is the circle of a Radio, as shadcn's 16px radio is; the indicator
// inside it is half that.
const (
	radioDiameter  = float32(16)
	radioIndicator = float32(8)
	radioRing      = float32(3)
)

// Radio adds one option of a radio group with the theme's look: a click sets
// *value to option, and it is a radio button that is chosen while *value is
// option. It is RadioBase with a circle and a label painted over it, so Box.Disabled
// makes it inert and dimmed like the other controls, and the label is the option's
// accessible name.
//
//	ui.Column(c).Gap(8).Children(func() {
//		ui.Radio(c, &m.size, Small, "Small")
//		ui.Radio(c, &m.size, Large, "Large")
//	})
//
// Name it with c.Key when the options are reordered.
func Radio[T comparable](c C, value *T, option T, label string) Box {
	id := c.page.controlID(pkRadio)
	b := c.f.RadioBase(id, label, *value == option)
	choose(c, b, value, option)
	theme := c.Theme()
	gap := theme.Metrics.GapItem
	var width float32
	if canvas := c.Canvas(); canvas != nil {
		width = canvas.TextWidthWeight(label, theme.Metrics.FontSizeM, gfx.WeightMedium)
	}
	b.MinHeight(theme.Metrics.ControlHeight).MinWidth(radioDiameter + gap + width)
	b.Draw(paintRadioOption)
	return b
}

// paintRadioOption paints the circle, its indicator and the label of a Radio, using
// the state the declaration left: chosen, disabled, and the focus ring when the
// keyboard is on it.
func paintRadioOption(d *DrawContext) {
	theme := d.Theme
	state := d.State
	gap := theme.Metrics.GapItem
	circle := Rect{
		X: d.Rect.X, Y: d.Rect.Y + (d.Rect.Height-radioDiameter)/2,
		Width: radioDiameter, Height: radioDiameter,
	}
	border, fill := theme.Input, Color{}
	if state.Checked {
		border, fill = theme.Primary, theme.Primary
	}
	if state.Disabled {
		border.Alpha *= 0.50
		fill.Alpha *= 0.50
	}
	if state.FocusVisible && !state.Disabled {
		ring := theme.Ring
		ring.Alpha *= 0.50
		d.Canvas.Rounded(Rect{
			X: circle.X - radioRing, Y: circle.Y - radioRing,
			Width: circle.Width + 2*radioRing, Height: circle.Height + 2*radioRing,
		}, radioDiameter/2+radioRing, ring)
	}
	d.Canvas.RoundedBorder(circle, radioDiameter/2, 1, border, fill)
	if state.Checked {
		indicator := Rect{
			X: circle.X + (radioDiameter-radioIndicator)/2, Y: circle.Y + (radioDiameter-radioIndicator)/2,
			Width: radioIndicator, Height: radioIndicator,
		}
		d.Canvas.Rounded(indicator, radioIndicator/2, theme.PrimaryForeground)
	}
	foreground := theme.Foreground
	if state.Disabled {
		foreground.Alpha *= 0.50
	}
	d.Canvas.Label(d.Label, Rect{
		X: circle.X + radioDiameter + gap, Y: d.Rect.Y,
		Width: max(d.Rect.Width-radioDiameter-gap, 0), Height: d.Rect.Height,
	}, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground})
}
