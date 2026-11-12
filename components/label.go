package components

import "github.com/vivaui/viva/gfx"

// LabelPainter is a compact field-label declaration.
type LabelPainter struct {
	canvas   *gfx.Canvas
	theme    gfx.Theme
	bounds   gfx.Rect
	disabled bool
	required bool
	control  gfx.ID
}

// BeginLabel starts a label declaration.
func BeginLabel(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect) LabelPainter {
	return LabelPainter{canvas: canvas, theme: theme, bounds: bounds}
}

// Disabled uses the muted label treatment.
func (label LabelPainter) Disabled() LabelPainter {
	label.disabled = true
	return label
}

// Required appends a semantic danger-colored required marker.
func (label LabelPainter) Required() LabelPainter {
	label.required = true
	return label
}

// For makes the label the label of a control, as `htmlFor` does: pressing the
// text focuses and clicks the control (see gfx.Canvas.HitLabel), and the text
// names it for assistive technology. A disabled label does nothing.
func (label LabelPainter) For(control gfx.ID) LabelPainter {
	label.control = control
	return label
}

// Paint draws the label text and optional required marker.
func (label LabelPainter) Paint(text string) {
	metrics := label.theme.Metrics
	if label.bounds.Height <= 0 {
		label.bounds.Height = metrics.FontSizeM + 3
	}
	foreground := label.theme.Foreground
	if label.disabled {
		foreground.Alpha *= 0.50
	}
	width := label.canvas.TextWidthWeight(text, metrics.FontSizeM, gfx.WeightMedium)
	if label.bounds.Width > 0 && width > label.bounds.Width {
		width = label.bounds.Width
	}
	if width < 0 {
		width = 0
	}
	label.canvas.Label(text, gfx.Rect{
		X: label.bounds.X, Y: label.bounds.Y, Width: width, Height: label.bounds.Height,
	}, gfx.TextStyle{
		Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: foreground,
	})
	if !label.control.IsZero() {
		label.canvas.HitLabel(label.control, gfx.Rect{X: label.bounds.X, Y: label.bounds.Y, Width: width, Height: label.bounds.Height}, text, label.disabled)
	}
	if label.required && width < label.bounds.Width {
		markerColor := label.theme.Danger
		if label.disabled {
			markerColor.Alpha *= 0.50
		}
		label.canvas.Label("*", gfx.Rect{
			X: label.bounds.X + width + 3, Y: label.bounds.Y,
			Width: label.bounds.Width - width - 3, Height: label.bounds.Height,
		}, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: markerColor,
		})
	}
}
