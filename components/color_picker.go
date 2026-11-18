package components

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

func pickerUnit(v float32) float32 {
	if !(v >= 0) {
		return 0
	}
	return min(v, 1)
}

// ParseColorHex accepts #RGB/#RRGGBB and, when alpha is enabled, their alpha
// forms. Product-specific formats and validation stay with the caller.

// ColorPickerOptions is a snapshot, not a binding or a hidden editor. The
// application stages Value and Hex while open, commits on Apply, and restores
// its original value on Cancel/Dismiss. Native text input resolves IDs.Hex to
// the application's editor just as it does for Input.
type ColorPickerOptions struct {
	IDs                                ColorPickerIDs
	Labels                             ColorPickerLabels
	Value                              ColorPickerValue
	Hex                                InputSnapshot
	Error                              string
	SaturationText, HueText, AlphaText string
	Open, Empty, Disabled, ShowAlpha   bool
	// RoundTrigger renders a circular swatch. TriggerIcon can identify a custom
	// color action independently of its selected color.
	RoundTrigger bool
	TriggerIcon  *gfx.Icon
	Viewport     gfx.Rect
	Layer        uint16
	Scope        gfx.ID
}

type ColorPicker struct {
	canvas                 *gfx.Canvas
	theme                  gfx.Theme
	bounds                 gfx.Rect
	options                ColorPickerOptions
	triggerOnly, panelOnly bool
}

func BeginColorPicker(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, options ColorPickerOptions) ColorPicker {
	return ColorPicker{canvas: canvas, theme: theme, bounds: bounds, options: options}
}

// PaintTrigger can be used inside a clipped form; paint its panel afterwards
// with PaintPanel so the popup belongs to the window rather than that clip.
func (picker ColorPicker) PaintTrigger() { picker.triggerOnly = true; picker.Paint() }

// PaintPanel paints only the open popup, retaining the same anchor and IDs.
func (picker ColorPicker) PaintPanel() ColorPickerLayout {
	picker.panelOnly = true
	return picker.Paint()
}

func (picker ColorPicker) Paint() ColorPickerLayout {
	c, theme, o := picker.canvas, picker.theme, picker.options
	l := ColorPickerLayout{IDs: o.IDs, Disabled: o.Disabled, ShowAlpha: o.ShowAlpha}
	if c == nil || picker.bounds.Width <= 0 || picker.bounds.Height <= 0 {
		return l
	}
	labels := o.Labels
	if labels.Color == "" {
		labels.Color = "Choose color"
	}
	if labels.Saturation == "" {
		labels.Saturation = "Saturation and brightness"
	}
	if labels.Hue == "" {
		labels.Hue = "Hue"
	}
	if labels.Alpha == "" {
		labels.Alpha = "Opacity"
	}
	if labels.Hex == "" {
		labels.Hex = "Hex color"
	}
	if labels.Clear == "" {
		labels.Clear = "Clear"
	}
	if labels.Cancel == "" {
		labels.Cancel = "Cancel"
	}
	if labels.Apply == "" {
		labels.Apply = "OK"
	}
	v := o.Value.Normalized()
	if !o.ShowAlpha {
		v.Alpha = 1
	}
	if !picker.panelOnly {
		radius := theme.Metrics.RadiusControl
		if o.RoundTrigger {
			radius = min(picker.bounds.Width, picker.bounds.Height) / 2
			border := theme.Input
			if !o.Empty || o.Open {
				border = theme.Foreground
			}
			if c.Hovered() == o.IDs.Trigger || c.FocusVisible() == o.IDs.Trigger {
				border = theme.Ring
			}
			if o.Disabled {
				border.Alpha *= .4
			}
			c.Rounded(picker.bounds, radius, border)
			inner := gfx.Rect{X: picker.bounds.X + 2, Y: picker.bounds.Y + 2, Width: max(0, picker.bounds.Width-4), Height: max(0, picker.bounds.Height-4)}
			c.Rounded(inner, min(inner.Width, inner.Height)/2, theme.Background)
		} else {
			paintBordered(c, theme, picker.bounds, radius, theme.Border, theme.Background)
		}
		preview := gfx.Rect{X: picker.bounds.X + 5, Y: picker.bounds.Y + 5, Width: max(picker.bounds.Width-10, 0), Height: max(picker.bounds.Height-10, 0)}
		if o.RoundTrigger {
			c.Rounded(preview, min(preview.Width, preview.Height)/2, theme.Muted)
		} else {
			pickerChecker(c, theme, preview)
		}
		if o.Empty {
			if o.TriggerIcon == nil {
				c.LabelCenter("×", preview, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground})
			}
		} else {
			if o.RoundTrigger {
				c.Rounded(preview, min(preview.Width, preview.Height)/2, v.Color())
			} else {
				c.Fill(preview, v.Color())
			}
		}
		if o.TriggerIcon != nil {
			iconColor := theme.Foreground
			if !o.Empty {
				if color := v.Color(); .2126*color.Red+.7152*color.Green+.0722*color.Blue > .5 {
					iconColor = gfx.Color{Alpha: 1}
				} else {
					iconColor = gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
				}
			}
			c.DrawIcon(*o.TriggerIcon, gfx.Rect{X: picker.bounds.X + (picker.bounds.Width-16)/2, Y: picker.bounds.Y + (picker.bounds.Height-16)/2, Width: 16, Height: 16}, iconColor)
		}
		if o.Disabled {
			color := theme.Background
			color.Alpha = .55
			if o.RoundTrigger {
				c.Rounded(picker.bounds, radius, color)
			} else {
				c.Fill(picker.bounds, color)
			}
		}
		if c.FocusVisible() == o.IDs.Trigger && !o.Disabled {
			if o.RoundTrigger {
				c.RoundedBorder(picker.bounds, radius, 2, theme.Ring, gfx.Color{})
			} else {
				pickerFocusRing(c, theme, picker.bounds)
			}
		}
		c.HitWithOptions(o.IDs.Trigger, picker.bounds, labels.Color, gfx.HitOptions{Role: gfx.RoleButton, Hint: labels.Color, Disabled: o.Disabled, Expanded: o.Open})
	}
	if picker.triggerOnly || !o.Open || o.Disabled || o.Viewport.Width <= 0 || o.Viewport.Height <= 0 {
		return l
	}
	gap, control := theme.Metrics.GapItem, theme.Metrics.ControlHeight
	errorHeight := float32(0)
	if o.Error != "" {
		errorHeight = theme.Metrics.FontSizeS + 6 + gap
	}
	alphaHeight := float32(0)
	if o.ShowAlpha {
		alphaHeight = 16 + gap
	}
	fixed := float32(16) + 3*gap + 2*control + alphaHeight + errorHeight
	overhead := 2*theme.Metrics.PadSurface + popoverHeaderHeight(theme.Metrics, 0) + popoverBodyGap(theme.Metrics) + 16
	planeHeight := min(float32(160), max(0, o.Viewport.Height-fixed-overhead))
	popover := BeginPopover(c, theme, o.Viewport, picker.bounds, PopoverOptions{
		Layer: o.Layer, Scope: o.Scope, DismissID: o.IDs.Dismiss, AnchorDismiss: true, ConsumeOutsidePress: true,
		Align: PopoverAlignEnd, SideOffset: 6, ContentSize: gfx.Size{Width: 280}, BodyHeight: planeHeight + fixed,
		InitialFocusID: o.IDs.Saturation,
	})
	body := popover.Begin(labels.Color, "").Body
	defer popover.End()
	if !o.Scope.IsZero() {
		c.TrapFocus(o.Scope)
	}
	l.Content = body
	c.Clip(body)
	defer c.Unclip()
	l.Saturation = gfx.Rect{X: body.X, Y: body.Y, Width: body.Width, Height: planeHeight}
	hue := ColorPickerValue{Hue: v.Hue, Saturation: 1, Brightness: 1, Alpha: 1}.Color()
	c.FillLinearGradient(l.Saturation, gfx.LinearGradient{Start: gfx.Point{X: body.X, Y: body.Y}, End: gfx.Point{X: body.X + body.Width, Y: body.Y}, From: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, To: hue})
	c.FillLinearGradient(l.Saturation, gfx.LinearGradient{Start: gfx.Point{X: body.X, Y: body.Y}, End: gfx.Point{X: body.X, Y: body.Y + planeHeight}, From: gfx.Color{}, To: gfx.Color{Alpha: 1}})
	pickerHandle(c, theme, body.X+v.Saturation*body.Width, body.Y+(1-v.Brightness)*planeHeight)
	c.HitWithOptions(o.IDs.Saturation, l.Saturation, labels.Saturation, gfx.HitOptions{Role: gfx.RoleSlider, Value: o.SaturationText, Range: gfx.ValueRange{Min: 0, Max: 100, Now: float64(v.Saturation * 100), Step: 1, Set: true}})
	if c.FocusVisible() == o.IDs.Saturation {
		pickerFocusRing(c, theme, l.Saturation)
	}
	l.Hue = gfx.Rect{X: body.X, Y: body.Y + planeHeight + gap, Width: body.Width, Height: 16}
	for i := range 6 {
		left, right := body.X+float32(i)*body.Width/6, body.X+float32(i+1)*body.Width/6
		from := ColorPickerValue{Hue: float32(i) * 60, Saturation: 1, Brightness: 1, Alpha: 1}.Color()
		to := ColorPickerValue{Hue: float32(i+1) * 60, Saturation: 1, Brightness: 1, Alpha: 1}.Color()
		c.FillLinearGradient(gfx.Rect{X: left, Y: l.Hue.Y, Width: right - left, Height: 16}, gfx.LinearGradient{Start: gfx.Point{X: left, Y: l.Hue.Y}, End: gfx.Point{X: right, Y: l.Hue.Y}, From: from, To: to})
	}
	pickerBarHandle(c, theme, l.Hue, v.Hue/360, c.FocusVisible() == o.IDs.Hue)
	c.HitWithOptions(o.IDs.Hue, l.Hue, labels.Hue, gfx.HitOptions{Role: gfx.RoleSlider, Value: o.HueText, Range: gfx.ValueRange{Min: 0, Max: 360, Now: float64(v.Hue), Step: 1, Set: true}})
	y := l.Hue.Y + l.Hue.Height + gap
	if o.ShowAlpha {
		l.Alpha = gfx.Rect{X: body.X, Y: y, Width: body.Width, Height: 16}
		pickerChecker(c, theme, l.Alpha)
		from, to := v.Color(), v.Color()
		from.Alpha, to.Alpha = 0, 1
		c.FillLinearGradient(l.Alpha, gfx.LinearGradient{Start: gfx.Point{X: body.X, Y: y}, End: gfx.Point{X: body.X + body.Width, Y: y}, From: from, To: to})
		pickerBarHandle(c, theme, l.Alpha, v.Alpha, c.FocusVisible() == o.IDs.Alpha)
		c.HitWithOptions(o.IDs.Alpha, l.Alpha, labels.Alpha, gfx.HitOptions{Role: gfx.RoleSlider, Value: o.AlphaText, Range: gfx.ValueRange{Min: 0, Max: 100, Now: float64(v.Alpha * 100), Step: 1, Set: true}})
		y += alphaHeight
	}
	l.Hex = gfx.Rect{X: body.X, Y: y, Width: body.Width, Height: control}
	input := o.Hex
	input.ID, input.Label = o.IDs.Hex, labels.Hex
	PaintInput(c, theme, l.Hex, input)
	y += control + gap
	if o.Error != "" {
		c.Label(o.Error, gfx.Rect{X: body.X, Y: y, Width: body.Width, Height: errorHeight - gap}, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.Danger})
		y += errorHeight
	}
	width := max(0, (body.Width-2*gap)/3)
	BeginButton(c, theme, o.IDs.Clear, gfx.Rect{X: body.X, Y: y, Width: width, Height: control}).Ghost(labels.Clear)
	BeginButton(c, theme, o.IDs.Cancel, gfx.Rect{X: body.X + width + gap, Y: y, Width: width, Height: control}).Outline(labels.Cancel)
	apply := BeginButton(c, theme, o.IDs.Apply, gfx.Rect{X: body.X + 2*(width+gap), Y: y, Width: width, Height: control})
	if input.Invalid {
		apply = apply.Disabled()
	}
	apply.Default(labels.Apply)
	return l
}

func pickerChecker(c *gfx.Canvas, theme gfx.Theme, b gfx.Rect) {
	c.Fill(b, theme.Background)
	for y := b.Y; y < b.Y+b.Height; y += 8 {
		for x := b.X; x < b.X+b.Width; x += 8 {
			if (int((x-b.X)/8)+int((y-b.Y)/8))%2 == 0 {
				c.Fill(gfx.Rect{X: x, Y: y, Width: min(8, b.X+b.Width-x), Height: min(8, b.Y+b.Height-y)}, theme.Muted)
			}
		}
	}
}

func pickerHandle(c *gfx.Canvas, theme gfx.Theme, x, y float32) {
	c.Rounded(gfx.Rect{X: x - 6, Y: y - 6, Width: 12, Height: 12}, 6, theme.Foreground)
	c.Rounded(gfx.Rect{X: x - 4, Y: y - 4, Width: 8, Height: 8}, 4, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
}

func pickerBarHandle(c *gfx.Canvas, theme gfx.Theme, b gfx.Rect, value float32, focused bool) {
	r := gfx.Rect{X: b.X + pickerUnit(value)*b.Width - 3, Y: b.Y, Width: 6, Height: b.Height}
	if focused {
		pickerFocusRing(c, theme, expand(r, 2))
	}
	paintBordered(c, theme, r, 2, theme.Foreground, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
}

func pickerFocusRing(c *gfx.Canvas, theme gfx.Theme, b gfx.Rect) {
	const width = float32(2)
	c.Fill(gfx.Rect{X: b.X, Y: b.Y, Width: b.Width, Height: min(width, b.Height)}, theme.Ring)
	c.Fill(gfx.Rect{X: b.X, Y: b.Y + max(0, b.Height-width), Width: b.Width, Height: min(width, b.Height)}, theme.Ring)
	c.Fill(gfx.Rect{X: b.X, Y: b.Y, Width: min(width, b.Width), Height: b.Height}, theme.Ring)
	c.Fill(gfx.Rect{X: b.X + max(0, b.Width-width), Y: b.Y, Width: min(width, b.Width), Height: b.Height}, theme.Ring)
}

// Aliases into the style vocabulary.
type ColorPickerValue = base.ColorPickerValue
type ColorPickerIDs = base.ColorPickerIDs
type ColorPickerLabels = base.ColorPickerLabels
type ColorPickerLayout = base.ColorPickerLayout

// ParseColorHex parses #RGB, #RGBA, #RRGGBB and #RRGGBBAA; it lives in style.
func ParseColorHex(value string, alpha bool) (gfx.Color, bool) {
	return base.ParseColorHex(value, alpha)
}
