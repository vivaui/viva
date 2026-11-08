package components

import "github.com/vivaui/viva/ui"

// ToggleVariant selects the neutral or bordered treatment.
type ToggleVariant uint8

const (
	ToggleDefault ToggleVariant = iota
	ToggleOutline
)

// ToggleSize selects compact relative control metrics.
type ToggleSize uint8

const (
	ToggleSizeDefault ToggleSize = iota
	ToggleSizeSmall
	ToggleSizeLarge
)

// Toggle paints one two-state compact action. The application owns active
// state and responds to the stable Canvas hit ID.
type Toggle struct {
	canvas   *ui.Canvas
	theme    ui.Theme
	id       ui.ID
	bounds   ui.Rect
	active   bool
	size     ToggleSize
	invalid  bool
	disabled bool
}

// Small selects the compact size variant.
func (toggle Toggle) Small() Toggle {
	toggle.size = ToggleSizeSmall
	return toggle
}

// Large selects the large size variant.
func (toggle Toggle) Large() Toggle {
	toggle.size = ToggleSizeLarge
	return toggle
}

// Invalid applies the destructive validation treatment.
func (toggle Toggle) Invalid() Toggle {
	toggle.invalid = true
	return toggle
}

// BeginToggle starts a toggle declaration.
func BeginToggle(canvas *ui.Canvas, theme ui.Theme, id ui.ID, bounds ui.Rect, active bool) Toggle {
	return Toggle{canvas: canvas, theme: theme, id: id, bounds: bounds, active: active}
}

// Disabled renders and routes the toggle as inert.
func (toggle Toggle) Disabled() Toggle {
	toggle.disabled = true
	return toggle
}

// SizeToFit widens the toggle to its label plus Vega's px-2.5, never narrower
// than it is tall (min-w).
func (toggle Toggle) SizeToFit(label string) Toggle {
	toggle.bounds.Height = toggle.controlHeight()
	toggle.bounds.Width = toggleFittedWidth(toggle.canvas, toggle.theme, label, toggle.bounds.Height, 10)
	return toggle
}

// BoundsWidth exposes the fitted width for callers laying out neighbors.
func (toggle Toggle) BoundsWidth() float32 { return toggle.bounds.Width }

// toggleFittedWidth is a toggle's width around label: padding on both sides, at
// least as wide as it is tall.
func toggleFittedWidth(canvas *ui.Canvas, theme ui.Theme, label string, height, padding float32) float32 {
	return max(canvas.TextWidthWeight(label, theme.Metrics.FontSizeM, ui.WeightMedium)+padding*2, height)
}

// ToggleResult is what painting a toggle reports back. Clicked is true in the
// event pass of a click on it (see ui.Canvas.Clicked), so the code that declares
// a toggle is the code that flips it:
//
//	if components.BeginToggle(canvas, theme, bold, bounds, m.bold).Default("B").Clicked() {
//		m.bold = !m.bold
//	}
type ToggleResult struct {
	canvas *ui.Canvas
	id     ui.ID
}

// Clicked reports whether this paint is delivering a click on the toggle.
func (result ToggleResult) Clicked() bool {
	return result.canvas != nil && result.canvas.Clicked(result.id)
}

// Default paints the borderless treatment.
func (toggle Toggle) Default(label string) ToggleResult { return toggle.paint(label, ToggleDefault) }

// Outline paints the bordered treatment.
func (toggle Toggle) Outline(label string) ToggleResult { return toggle.paint(label, ToggleOutline) }

func (toggle Toggle) paint(label string, variant ToggleVariant) ToggleResult {
	result := ToggleResult{canvas: toggle.canvas, id: toggle.id}
	if toggle.canvas == nil {
		return result
	}
	if toggle.bounds.Height <= 0 {
		toggle.bounds.Height = toggle.controlHeight()
	}
	if toggle.bounds.Width <= 0 {
		toggle.bounds.Width = toggle.bounds.Height
	}
	enabled := !toggle.disabled && !toggle.id.IsZero()
	hovered := enabled && toggle.canvas.Hovered() == toggle.id
	pressed := enabled && toggle.canvas.Pressed() == toggle.id
	focused := enabled && toggle.canvas.FocusVisible() == toggle.id
	radius := toggle.theme.Metrics.RadiusControl
	border := toggle.theme.Input
	if focused || toggle.invalid {
		focus := scaledAlpha(toggle.theme.Ring, 0.50)
		border = toggle.theme.Ring
		if toggle.invalid {
			focus, border = invalidRing(&toggle.theme), invalidBorder(&toggle.theme)
		}
		toggle.canvas.Rounded(expand(toggle.bounds, 3), radius+3, focus)
	}

	fill := toggle.theme.Background
	fill.Alpha = 0
	foreground := toggle.theme.Foreground
	if toggle.active {
		fill = toggle.theme.Muted
	}
	if hovered {
		fill = toggle.theme.Muted
		foreground = toggle.theme.Foreground
	}
	if pressed {
		fill = blend(fill, toggle.theme.Foreground, 0.05)
	}
	if toggle.disabled {
		fill.Alpha *= 0.50
		foreground.Alpha *= 0.50
	}
	if variant == ToggleOutline || toggle.invalid || focused {
		if variant == ToggleOutline {
			toggle.canvas.Elevate(toggle.bounds, radius, ui.ShadowXS) // shadow-xs
		}
		if toggle.disabled {
			border.Alpha *= 0.50
		}
		paintBordered(toggle.canvas, toggle.theme, toggle.bounds, radius, border, fill)
	} else {
		toggle.canvas.Rounded(toggle.bounds, radius, fill)
	}
	toggle.canvas.LabelCenter(label, toggle.bounds, ui.TextStyle{
		Size: toggle.theme.Metrics.FontSizeM, Weight: ui.WeightMedium, Color: foreground,
	})
	if !toggle.id.IsZero() {
		toggle.canvas.HitWithOptions(toggle.id, toggle.bounds, label, ui.HitOptions{
			Role: ui.RoleButton, Disabled: toggle.disabled, Checked: toggle.active,
			Selected: toggle.active,
		})
	}
	return result
}

func (toggle Toggle) controlHeight() float32 {
	return toggleControlHeight(toggle.theme, toggle.size)
}

func toggleControlHeight(theme ui.Theme, size ToggleSize) float32 {
	height := theme.Metrics.ControlHeight
	switch size {
	case ToggleSizeSmall:
		height -= 4
	case ToggleSizeLarge:
		height += 4
	}
	if height < 20 {
		height = 20
	}
	return height
}
