package components

import (
	"github.com/vivaui/viva/ui"
)

// SegmentOption is one stable choice in a SegmentedControl. ID is returned by
// the normal canvas pointer and keyboard activation path; callers should keep
// both ID and order stable between paints.
type SegmentOption struct {
	ID       ui.ID
	Label    string
	Disabled bool
}

// SegmentedControl paints a compact, neutral choice group. It retains no
// state: selection belongs to the application and every paint reads transient
// pointer and focus state from the canvas.
type SegmentedControl struct {
	canvas   *ui.Canvas
	theme    ui.Theme
	bounds   ui.Rect
	options  []SegmentOption
	selected ui.ID
	hovered  ui.ID
	pressed  ui.ID
	focused  ui.ID
	disabled bool
	// tabStop is the one segment Tab reaches; arrows move among the others.
	tabStop ui.ID
}

// BeginSegmentedControl starts a segmented choice declaration. Each enabled,
// non-zero option ID becomes an independently focusable hit. The runtime's
// existing hit routing therefore activates the focused option with Enter or
// Space.
func BeginSegmentedControl(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, options []SegmentOption, selected ui.ID) SegmentedControl {
	control := SegmentedControl{
		canvas:   canvas,
		theme:    theme,
		bounds:   bounds,
		options:  options,
		selected: selected,
		hovered:  canvas.Hovered(),
		pressed:  canvas.Pressed(),
		focused:  canvas.FocusVisible(),
	}
	control.tabStop = control.rovingTabStop()
	return control
}

// rovingTabStop keeps the Tab stop on the focused segment, else on the
// selection, else on the first enabled one.
func (control SegmentedControl) rovingTabStop() ui.ID {
	enabled := func(option SegmentOption) bool {
		return !control.disabled && !option.Disabled && !option.ID.IsZero()
	}
	for _, option := range control.options {
		if option.ID == control.focused && enabled(option) {
			return control.focused
		}
	}
	for _, option := range control.options {
		if option.ID == control.selected && enabled(option) {
			return control.selected
		}
	}
	for _, option := range control.options {
		if enabled(option) {
			return option.ID
		}
	}
	return ui.ID{}
}

// SegmentedControlWidth is the narrowest width at which every option's label is
// whole with the theme's control padding on both sides of it. The options are of
// equal width, so it is the widest label's, times their number: a control given
// less than this cuts its longest label.
func SegmentedControlWidth(canvas *ui.Canvas, theme ui.Theme, options []SegmentOption) float32 {
	var widest float32
	for _, option := range options {
		widest = max(widest, canvas.TextWidthWeight(option.Label, theme.Metrics.FontSizeM, ui.WeightMedium))
	}
	return float32(len(options)) * (widest + 2*theme.Metrics.PadControlX)
}

// SegmentedControlResult is what painting a segmented control reports back.
type SegmentedControlResult struct {
	canvas  *ui.Canvas
	options []SegmentOption
}

// Clicked returns the option whose click this paint is delivering, and false in
// every other paint (see ui.Canvas.Clicked), so the code that declares the
// control is the code that changes the selection:
//
//	if id, ok := components.BeginSegmentedControl(canvas, theme, bounds, options, selected).Paint().Clicked(); ok {
//		selected = id
//	}
func (result SegmentedControlResult) Clicked() (ui.ID, bool) {
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

// Disabled renders the complete control as inert.
func (control SegmentedControl) Disabled() SegmentedControl {
	control.disabled = true
	return control
}

// SegmentedControlKeyDown applies horizontal roving focus to current. Left and
// Right wrap through enabled non-zero options and reverse in RTL; Home and End
// choose the first and last enabled options. Vertical arrows are not owned by
// a horizontal segmented control.
func SegmentedControlKeyDown(current ui.ID, options []SegmentOption, direction Direction, key ui.Key) (ui.ID, bool) {
	step := 0
	switch key {
	case ui.KeyLeft:
		step = -1
		if direction.IsRTL() {
			step = 1
		}
	case ui.KeyRight:
		step = 1
		if direction.IsRTL() {
			step = -1
		}
	case ui.KeyHome:
		return firstEnabledSegment(current, options, 1), true
	case ui.KeyEnd:
		return firstEnabledSegment(current, options, -1), true
	default:
		return current, false
	}
	return nextEnabledSegment(current, options, step), true
}

func firstEnabledSegment(current ui.ID, options []SegmentOption, step int) ui.ID {
	for index := 0; index < len(options); index++ {
		candidate := index
		if step < 0 {
			candidate = len(options) - 1 - index
		}
		option := options[candidate]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

func nextEnabledSegment(current ui.ID, options []SegmentOption, step int) ui.ID {
	if len(options) == 0 {
		return current
	}
	currentIndex := -1
	for index := range options {
		if options[index].ID == current {
			currentIndex = index
			break
		}
	}
	if currentIndex < 0 {
		return firstEnabledSegment(current, options, step)
	}
	for offset := 1; offset <= len(options); offset++ {
		index := (currentIndex + step*offset) % len(options)
		if index < 0 {
			index += len(options)
		}
		option := options[index]
		if !option.ID.IsZero() && !option.Disabled {
			return option.ID
		}
	}
	return current
}

// Paint draws the track, one stable surface command per option, separators,
// labels, and hit regions. Stable option order keeps retained scene identities
// unchanged while hover, press, and selection only update command values. Each
// option's hit is a radio (checked when selected), disabled ones included.
func (control SegmentedControl) Paint() SegmentedControlResult {
	result := SegmentedControlResult{canvas: control.canvas, options: control.options}
	if len(control.options) == 0 || control.bounds.Width <= 0 {
		return result
	}

	metrics := control.theme.Metrics
	if control.bounds.Height <= 0 {
		control.bounds.Height = metrics.ControlHeight
	}
	if control.bounds.Height <= 0 {
		return result
	}
	radius := metrics.RadiusControl

	trackFill := control.theme.Muted
	if control.disabled {
		trackFill = control.theme.CardElevated
	}
	paintBordered(control.canvas, control.theme, control.bounds, radius, control.theme.Border, trackFill)

	count := float32(len(control.options))
	innerRadius := radius - 2
	if innerRadius < 2 {
		innerRadius = 2
	}

	// Every option emits exactly one surface rectangle. Resting choices use the
	// track color, so interaction changes colors without shifting scene nodes.
	// The selected choice is a raised pill. On a light theme the card is
	// lighter than the track, which does that; on a dark theme the card is
	// darker than the track, so using it would sink the pill into the track
	// with barely any contrast. There the pill is the track lifted toward the
	// foreground instead.
	selectedFill := control.theme.Card
	if segmentedControlLuminance(selectedFill) < segmentedControlLuminance(trackFill)+0.02 {
		selectedFill = segmentedControlBlend(trackFill, control.theme.Foreground, 0.14)
	}
	for index, option := range control.options {
		bounds := control.segmentBounds(index, count)
		surface := segmentedControlInset(bounds, 2)
		enabled := !control.disabled && !option.Disabled
		hovered := enabled && !option.ID.IsZero() && control.hovered == option.ID
		pressed := enabled && !option.ID.IsZero() && control.pressed == option.ID
		selected := !option.ID.IsZero() && option.ID == control.selected

		fill := trackFill
		if control.theme.Background.Alpha < 1 {
			// The track is already painted; resting segments must not tint it twice.
			fill.Alpha = 0
		}
		switch {
		case selected && pressed:
			fill = segmentedControlBlend(selectedFill, control.theme.Foreground, 0.08)
		case selected && hovered:
			fill = segmentedControlBlend(selectedFill, control.theme.Foreground, 0.04)
		case selected:
			fill = selectedFill
		case pressed:
			fill = control.theme.Border
		case hovered:
			fill = control.theme.Hover
		}
		if !enabled && selected {
			fill = control.theme.Muted
		}
		if enabled && !option.ID.IsZero() && control.focused == option.ID {
			// The segment that holds the keyboard, not the whole control.
			if control.theme.Background.Alpha < 1 {
				control.canvas.RoundedOutline(segmentedControlExpand(surface, 3), innerRadius+3, 2, scaledAlpha(control.theme.Ring, 0.50))
			} else {
				control.canvas.Rounded(segmentedControlExpand(surface, 3), innerRadius+3, scaledAlpha(control.theme.Ring, 0.50))
			}
			paintBordered(control.canvas, control.theme, surface, innerRadius, control.theme.Ring, fill)
			continue
		}
		control.canvas.Rounded(surface, innerRadius, fill)
	}

	// Separators are always present in the command stream. Boundaries touching
	// the selected pill become transparent, preserving both the visual capsule
	// and retained command identity.
	separatorColor := control.theme.Border
	separatorTop := control.bounds.Y + control.bounds.Height*0.28
	separatorHeight := control.bounds.Height * 0.44
	for index := 1; index < len(control.options); index++ {
		color := separatorColor
		if control.options[index-1].ID == control.selected || control.options[index].ID == control.selected {
			color.Alpha = 0
		}
		x := control.bounds.X + control.bounds.Width*float32(index)/count
		control.canvas.Fill(ui.Rect{X: x - 0.5, Y: separatorTop, Width: 1, Height: separatorHeight}, color)
	}

	for index, option := range control.options {
		bounds := control.segmentBounds(index, count)
		enabled := !control.disabled && !option.Disabled
		active := !option.ID.IsZero() && option.ID == control.selected
		hovered := enabled && !option.ID.IsZero() && control.hovered == option.ID

		foreground := control.theme.MutedForeground
		if active || hovered {
			foreground = control.theme.Foreground
		}
		if !enabled {
			foreground = control.theme.MutedForeground
		}

		// Every option shares one weight: a heavier selected label is wider at the
		// same size, so it would look enlarged and shift off center.
		control.canvas.LabelCenter(option.Label, bounds, ui.TextStyle{
			Size: metrics.FontSizeM, Weight: ui.WeightMedium, Color: foreground,
		})
		if !option.ID.IsZero() {
			control.canvas.HitWithOptions(option.ID, bounds, option.Label, ui.HitOptions{
				Role: ui.RoleRadio, Value: option.Label, Disabled: !enabled,
				Checked: active, Selected: active,
				SkipTab: option.ID != control.tabStop,
			})
		}
	}
	return result
}

func (control SegmentedControl) segmentBounds(index int, count float32) ui.Rect {
	left := control.bounds.X + control.bounds.Width*float32(index)/count
	right := control.bounds.X + control.bounds.Width*float32(index+1)/count
	return ui.Rect{X: left, Y: control.bounds.Y, Width: right - left, Height: control.bounds.Height}
}

func segmentedControlInset(bounds ui.Rect, amount float32) ui.Rect {
	width := bounds.Width - amount*2
	height := bounds.Height - amount*2
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return ui.Rect{X: bounds.X + amount, Y: bounds.Y + amount, Width: width, Height: height}
}

func segmentedControlExpand(bounds ui.Rect, amount float32) ui.Rect {
	return ui.Rect{
		X: bounds.X - amount, Y: bounds.Y - amount,
		Width: bounds.Width + amount*2, Height: bounds.Height + amount*2,
	}
}

// segmentedControlLuminance is the relative brightness of a color, 0 to 1.
func segmentedControlLuminance(color ui.Color) float32 {
	return 0.2126*color.Red + 0.7152*color.Green + 0.0722*color.Blue
}

func segmentedControlBlend(color, overlay ui.Color, amount float32) ui.Color {
	return ui.Color{
		Red:   color.Red + (overlay.Red-color.Red)*amount,
		Green: color.Green + (overlay.Green-color.Green)*amount,
		Blue:  color.Blue + (overlay.Blue-color.Blue)*amount,
		Alpha: color.Alpha,
	}
}
