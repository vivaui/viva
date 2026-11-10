// Package field is the one form-field rhythm behind components.BeginField and
// ui.FormField: where a title, a control, a description and an error sit, and
// how the title and the metadata are painted. An application normally declares
// a field with one of those two; a component recipe of its own uses Measure and
// Paint so it keeps the same rhythm.
package field

import "github.com/vivaui/viva/gfx"

// Orientation mirrors the Base UI field group's vertical, horizontal,
// and container-responsive arrangements.
type Orientation uint8

const (
	Vertical Orientation = iota
	Horizontal
	Responsive
)

// Layout is where each part of a field sits. It is the one layout behind
// BeginField and ui.FormField; component authors building a field recipe of
// their own use Measure and Paint so it keeps the same rhythm.
type Layout struct {
	Bounds      gfx.Rect
	Label       gfx.Rect
	Control     gfx.Rect
	Description gfx.Rect
	Error       gfx.Rect
}

// Spec is what a field says beyond its bounds. Control, when set,
// makes the title a label of that control, as an htmlFor does.
type Spec struct {
	Title       string
	Description string
	Error       string
	Orientation Orientation
	Required    bool
	Invalid     bool
	Disabled    bool
	Control     gfx.ID
}

// TitleHeight is the row of a field's title.
func TitleHeight(metrics gfx.Metrics) float32 { return metrics.FontSizeM + 4 }

// Gap is what sits between a title and its control, and between the
// control and its description or error.
func Gap(metrics gfx.Metrics) float32 { return metrics.GapItem / 2 }

// TextHeight is the row of a description or an error.
func TextHeight(metrics gfx.Metrics) float32 { return metrics.FontSizeM + 7 }

// Measure lays a field out in bounds. A bounds height of zero or less is
// replaced by the field's own: a title, the control, and room for a description
// and an error, whether or not it has them. That is the height BeginField has
// always reported, so a page field reserves the same room as the painted one.
func Measure(theme gfx.Theme, bounds gfx.Rect, spec Spec) Layout {
	metrics := theme.Metrics
	orientation := spec.Orientation
	if orientation == Responsive {
		orientation = Vertical
		if bounds.Width >= 360 {
			orientation = Horizontal
		}
	}
	if bounds.Height <= 0 {
		bounds.Height = TitleHeight(metrics) + Gap(metrics) + metrics.ControlHeight + (Gap(metrics)+TextHeight(metrics))*2
	}
	layout := Layout{Bounds: bounds}
	if orientation == Horizontal {
		measureHorizontal(&layout, metrics, spec)
		return layout
	}
	measureVertical(&layout, metrics, spec)
	return layout
}

func measureVertical(layout *Layout, metrics gfx.Metrics, spec Spec) {
	bounds := layout.Bounds
	titleHeight := TitleHeight(metrics)
	layout.Label = gfx.Rect{X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: titleHeight}
	layout.Control = gfx.Rect{
		X: bounds.X, Y: bounds.Y + titleHeight + Gap(metrics),
		Width: bounds.Width, Height: metrics.ControlHeight,
	}
	y := layout.Control.Y + layout.Control.Height
	if spec.Description != "" {
		y += Gap(metrics)
		layout.Description = gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: TextHeight(metrics)}
		y += TextHeight(metrics)
	}
	if spec.Error != "" {
		y += Gap(metrics)
		layout.Error = gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: TextHeight(metrics)}
	}
}

func measureHorizontal(layout *Layout, metrics gfx.Metrics, spec Spec) {
	bounds := layout.Bounds
	labelWidth := bounds.Width * 0.34
	gap := metrics.PadSurface
	if labelWidth+gap > bounds.Width {
		labelWidth = bounds.Width
		gap = 0
	}
	layout.Label = gfx.Rect{X: bounds.X, Y: bounds.Y, Width: labelWidth, Height: TitleHeight(metrics)}
	if spec.Description != "" {
		layout.Description = gfx.Rect{
			X: bounds.X, Y: bounds.Y + layout.Label.Height + Gap(metrics),
			Width: labelWidth, Height: TextHeight(metrics),
		}
	}
	layout.Control = gfx.Rect{
		X: bounds.X + labelWidth + gap, Y: bounds.Y,
		Width: max32(0, bounds.Width-labelWidth-gap), Height: metrics.ControlHeight,
	}
	if spec.Error != "" {
		layout.Error = gfx.Rect{
			X: layout.Control.X, Y: layout.Control.Y + layout.Control.Height + Gap(metrics),
			Width: layout.Control.Width, Height: TextHeight(metrics),
		}
	}
}

// Paint writes the title, description and error of layout, and nothing of
// the control: the caller paints that into layout.Control. It must be given the
// same spec Measure was.
func Paint(canvas *gfx.Canvas, theme gfx.Theme, layout Layout, spec Spec) {
	if canvas == nil {
		return
	}
	metrics := theme.Metrics
	titleColor := theme.Foreground
	if spec.Invalid {
		titleColor = theme.Danger
	}
	if spec.Disabled {
		titleColor.Alpha *= 0.50
	}
	width := canvas.TextWidthWeight(spec.Title, metrics.FontSizeM, gfx.WeightMedium)
	if width > layout.Label.Width {
		width = layout.Label.Width
	}
	canvas.Label(spec.Title, gfx.Rect{
		X: layout.Label.X, Y: layout.Label.Y, Width: width, Height: layout.Label.Height,
	}, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: titleColor})
	canvas.HitLabel(spec.Control, gfx.Rect{
		X: layout.Label.X, Y: layout.Label.Y, Width: width, Height: layout.Label.Height,
	}, spec.Title, spec.Disabled)
	if spec.Required && width < layout.Label.Width {
		markerColor := theme.Danger
		if spec.Disabled {
			markerColor.Alpha *= 0.50
		}
		canvas.Label("*", gfx.Rect{
			X: layout.Label.X + width + 3, Y: layout.Label.Y,
			Width: layout.Label.Width - width - 3, Height: layout.Label.Height,
		}, gfx.TextStyle{Size: metrics.FontSizeM, Weight: gfx.WeightMedium, Color: markerColor})
	}
	if spec.Description != "" {
		descriptionColor := theme.MutedForeground
		if spec.Disabled {
			descriptionColor.Alpha *= 0.50
		}
		canvas.Label(spec.Description, layout.Description, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: descriptionColor,
		})
	}
	if spec.Error != "" {
		errorColor := theme.Danger
		if spec.Disabled {
			errorColor.Alpha *= 0.50
		}
		canvas.Label(spec.Error, layout.Error, gfx.TextStyle{
			Size: metrics.FontSizeM, Weight: gfx.WeightRegular, Color: errorColor,
		})
	}
}

func max32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}
