package components

import "github.com/vivaui/viva/ui"

// ButtonGroupOption describes one stable grouped action.
type ButtonGroupOption struct {
	ID       ui.ID
	Label    string
	Variant  ButtonVariant
	Disabled bool
}

// ButtonGroupOrientation selects a row or column of connected actions.
type ButtonGroupOrientation uint8

const (
	ButtonGroupHorizontal ButtonGroupOrientation = iota
	ButtonGroupVertical
)

// ButtonGroup paints related actions as one connected control. Each action
// takes its colors from the same recipe as a Button of its variant, so a group
// can hold any of them.
type ButtonGroup struct {
	canvas      *ui.Canvas
	theme       ui.Theme
	bounds      ui.Rect
	options     []ButtonGroupOption
	orientation ButtonGroupOrientation
	disabled    bool
}

// ButtonGroupResult is what painting a button group reports back.
type ButtonGroupResult struct {
	canvas  *ui.Canvas
	options []ButtonGroupOption
}

// Clicked returns the action whose click this paint is delivering, and false in
// every other paint (see ui.Canvas.Clicked).
func (result ButtonGroupResult) Clicked() (ui.ID, bool) {
	if result.canvas == nil {
		return ui.ID{}, false
	}
	for _, option := range result.options {
		if result.canvas.Clicked(option.ID) {
			return option.ID, true
		}
	}
	return ui.ID{}, false
}

// Vertical stacks connected actions and collapses adjoining horizontal borders.
func (group ButtonGroup) Vertical() ButtonGroup {
	group.orientation = ButtonGroupVertical
	return group
}

// BeginButtonGroup starts a grouped action declaration.
func BeginButtonGroup(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, options []ButtonGroupOption) ButtonGroup {
	return ButtonGroup{canvas: canvas, theme: theme, bounds: bounds, options: options}
}

// Disabled renders every action as inert.
func (group ButtonGroup) Disabled() ButtonGroup {
	group.disabled = true
	return group
}

// Paint draws the actions side by side, rounded only at the group's outer
// corners, with a hairline between neighbours and a shared outline when any of
// them is an outline button.
func (group ButtonGroup) Paint() ButtonGroupResult {
	result := ButtonGroupResult{canvas: group.canvas, options: group.options}
	if group.canvas == nil || len(group.options) == 0 || group.bounds.Width <= 0 {
		return result
	}
	if group.bounds.Height <= 0 {
		group.bounds.Height = group.theme.Metrics.ControlHeight
		if group.orientation == ButtonGroupVertical {
			group.bounds.Height *= float32(len(group.options))
		}
	}
	metrics := group.theme.Metrics
	radius := metrics.RadiusControl
	count := len(group.options)
	vertical := group.orientation == ButtonGroupVertical
	framed := false
	for index, option := range group.options {
		framed = framed || option.Variant == ButtonOutline || option.Variant == ButtonDestructiveOutline
		if !group.disabled && !option.Disabled && !option.ID.IsZero() && group.canvas.FocusVisible() == option.ID {
			ring, _ := buttonRing(&group.theme, option.Variant, false)
			group.canvas.Rounded(expand(group.optionBounds(index, float32(count)), 3), radius+3, ring)
		}
	}
	// The outline goes down first and the actions are painted inside it.
	if framed {
		group.canvas.Elevate(group.bounds, radius, ui.ShadowXS) // the outline buttons' shadow-xs
		paintBordered(group.canvas, group.theme, group.bounds, radius, outlineBorder(&group.theme), ui.Color{})
		radius--
	}
	for index, option := range group.options {
		bounds := group.optionBounds(index, float32(count))
		surface := joinedSurface(bounds, index, count, vertical, framed)
		inert := group.disabled || option.Disabled || option.ID.IsZero()
		style := buttonStyleFor(&group.theme, option.Variant,
			!inert && group.canvas.Hovered() == option.ID, !inert && group.canvas.Pressed() == option.ID)
		fill := style.background
		if !style.filled {
			fill.Alpha = 0
		}
		if group.disabled || option.Disabled {
			fill.Alpha *= 0.50
			style.foreground.Alpha *= 0.50
		}
		paintJoinedSurface(group.canvas, surface, radius, fill, index, count, vertical)
		paintButtonLabel(group.canvas, option.Label, bounds, metrics.FontSizeM, style)
		if !option.ID.IsZero() {
			group.canvas.HitWithOptions(option.ID, bounds, option.Label, ui.HitOptions{
				Role: ui.RoleButton, Disabled: group.disabled || option.Disabled,
			})
		}
		if index > 0 {
			paintJoinedSeparator(group.canvas, surface, group.theme.Input, vertical)
		}
	}
	return result
}

func (group ButtonGroup) optionBounds(index int, count float32) ui.Rect {
	if group.orientation == ButtonGroupVertical {
		top := group.bounds.Y + group.bounds.Height*float32(index)/count
		bottom := group.bounds.Y + group.bounds.Height*float32(index+1)/count
		return ui.Rect{X: group.bounds.X, Y: top, Width: group.bounds.Width, Height: bottom - top}
	}
	left := group.bounds.X + group.bounds.Width*float32(index)/count
	right := group.bounds.X + group.bounds.Width*float32(index+1)/count
	return ui.Rect{X: left, Y: group.bounds.Y, Width: right - left, Height: group.bounds.Height}
}

// paintJoinedSurface fills item index of count in a connected group: rounded on
// the corners that are the group's own, square where it meets a neighbour. There
// is no per-corner rounding, so the end items are drawn as a rounded rectangle
// grown past their square side and clipped back to their bounds.
func paintJoinedSurface(canvas *ui.Canvas, bounds ui.Rect, radius float32, color ui.Color, index, count int, vertical bool) {
	if count == 1 {
		canvas.Rounded(bounds, radius, color)
		return
	}
	if index > 0 && index < count-1 {
		canvas.Fill(bounds, color)
		return
	}
	grown := bounds
	switch {
	case vertical:
		grown.Height += radius
		if index > 0 {
			grown.Y -= radius
		}
	default:
		grown.Width += radius
		if index > 0 {
			grown.X -= radius
		}
	}
	canvas.Clip(bounds)
	canvas.Rounded(grown, radius, color)
	canvas.Unclip()
}

// joinedSurface is the part of item index's bounds the item paints on. A framed
// group's outline takes the pixel along its outer edge, so the items lie inside
// it.
func joinedSurface(bounds ui.Rect, index, count int, vertical, framed bool) ui.Rect {
	if !framed {
		return bounds
	}
	if vertical {
		bounds.X, bounds.Width = bounds.X+1, bounds.Width-2
		if index == 0 {
			bounds.Y, bounds.Height = bounds.Y+1, bounds.Height-1
		}
		if index == count-1 {
			bounds.Height--
		}
		return bounds
	}
	bounds.Y, bounds.Height = bounds.Y+1, bounds.Height-2
	if index == 0 {
		bounds.X, bounds.Width = bounds.X+1, bounds.Width-1
	}
	if index == count-1 {
		bounds.Width--
	}
	return bounds
}

// paintJoinedSeparator draws the hairline on the leading edge of a connected
// group's item.
func paintJoinedSeparator(canvas *ui.Canvas, bounds ui.Rect, color ui.Color, vertical bool) {
	if vertical {
		canvas.Fill(ui.Rect{X: bounds.X, Y: bounds.Y - 0.5, Width: bounds.Width, Height: 1}, color)
		return
	}
	canvas.Fill(ui.Rect{X: bounds.X - 0.5, Y: bounds.Y, Width: 1, Height: bounds.Height}, color)
}
