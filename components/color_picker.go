package components

import (
	"strings"

	"github.com/vivaui/viva/gfx"
)

// ColorPickerValue is caller-owned HSV editing state. Keeping HSV rather than
// rebuilding it from RGB on every paint preserves the hue at gray and black.
type ColorPickerValue struct {
	Hue, Saturation, Brightness, Alpha float32
}

func pickerUnit(v float32) float32 {
	if !(v >= 0) {
		return 0
	}
	return min(v, 1)
}

func (v ColorPickerValue) normalized() ColorPickerValue {
	if !(v.Hue >= 0) {
		v.Hue = 0
	}
	v.Hue = min(v.Hue, 360)
	v.Saturation, v.Brightness, v.Alpha = pickerUnit(v.Saturation), pickerUnit(v.Brightness), pickerUnit(v.Alpha)
	return v
}

// Color converts the selected HSV value to straight RGBA without allocating.
func (v ColorPickerValue) Color() gfx.Color {
	v = v.normalized()
	h := v.Hue / 60
	sector := int(h)
	f := h - float32(sector)
	p, q, t := v.Brightness*(1-v.Saturation), v.Brightness*(1-v.Saturation*f), v.Brightness*(1-v.Saturation*(1-f))
	r, g, b := v.Brightness, t, p
	switch sector % 6 {
	case 1:
		r, g, b = q, v.Brightness, p
	case 2:
		r, g, b = p, v.Brightness, t
	case 3:
		r, g, b = p, q, v.Brightness
	case 4:
		r, g, b = t, p, v.Brightness
	case 5:
		r, g, b = v.Brightness, p, q
	}
	return gfx.Color{Red: r, Green: g, Blue: b, Alpha: v.Alpha}
}

// WithColor loads an RGB edit, retaining hue for achromatic colors and the
// previous saturation at black so a subsequent brightness change is useful.
func (v ColorPickerValue) WithColor(c gfx.Color) ColorPickerValue {
	v = v.normalized()
	r, g, b := pickerUnit(c.Red), pickerUnit(c.Green), pickerUnit(c.Blue)
	hi, lo := max(r, g, b), min(r, g, b)
	d := hi - lo
	v.Brightness, v.Alpha = hi, pickerUnit(c.Alpha)
	if hi > 0 {
		v.Saturation = d / hi
	}
	if d > 0 {
		switch hi {
		case r:
			v.Hue = 60 * (g - b) / d
		case g:
			v.Hue = 60 * (2 + (b-r)/d)
		default:
			v.Hue = 60 * (4 + (r-g)/d)
		}
		if v.Hue < 0 {
			v.Hue += 360
		}
	}
	return v
}

// AppendHex appends canonical #RRGGBB (or #RRGGBBAA) to caller-owned storage.
// Format only when a value changes; painting consumes the caller's input text.
func (v ColorPickerValue) AppendHex(dst []byte, alpha bool) []byte {
	c := v.Color()
	dst = append(dst, '#')
	channels := [4]float32{c.Red, c.Green, c.Blue, c.Alpha}
	n := 3
	if alpha {
		n = 4
	}
	const digits = "0123456789ABCDEF"
	for _, channel := range channels[:n] {
		b := byte(channel*255 + .5)
		dst = append(dst, digits[b>>4], digits[b&15])
	}
	return dst
}

// ParseColorHex accepts #RGB/#RRGGBB and, when alpha is enabled, their alpha
// forms. Product-specific formats and validation stay with the caller.
func ParseColorHex(value string, alpha bool) (gfx.Color, bool) {
	value = strings.TrimSpace(value)
	if len(value) == 0 || value[0] != '#' {
		return gfx.Color{}, false
	}
	value = value[1:]
	short := len(value) == 3 || alpha && len(value) == 4
	if !short && len(value) != 6 && !(alpha && len(value) == 8) {
		return gfx.Color{}, false
	}
	channels := [4]byte{0, 0, 0, 255}
	count := len(value) / 2
	if short {
		count = len(value)
	}
	for i := 0; i < count; i++ {
		at := i * 2
		if short {
			at = i
		}
		a, ok := pickerHexDigit(value[at])
		if !ok {
			return gfx.Color{}, false
		}
		b := a
		if !short {
			b, ok = pickerHexDigit(value[i*2+1])
			if !ok {
				return gfx.Color{}, false
			}
		}
		channels[i] = a*16 + b
	}
	return gfx.Color{Red: float32(channels[0]) / 255, Green: float32(channels[1]) / 255, Blue: float32(channels[2]) / 255, Alpha: float32(channels[3]) / 255}, true
}

func pickerHexDigit(b byte) (byte, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}

// ColorPickerIDs remain stable across opening, resizing and value changes.
type ColorPickerIDs struct {
	Trigger, Dismiss, Saturation, Hue, Alpha, Hex, Clear, Cancel, Apply gfx.ID
}

type ColorPickerLabels struct {
	Color, Saturation, Hue, Alpha, Hex, Clear, Cancel, Apply string
}

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

// ColorPickerLayout is also the input geometry. Pointer and Key update a copy
// of caller-owned HSV state, using logical pixels at every display scale.
type ColorPickerLayout struct {
	Content, Saturation, Hue, Alpha, Hex gfx.Rect
	IDs                                  ColorPickerIDs
	Disabled, ShowAlpha                  bool
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
	v := o.Value.normalized()
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

// Pointer handles primary press and captured dragging, including outside the
// popup. Other buttons and hover do not change the draft or steal text editing.
func (l ColorPickerLayout) Pointer(event gfx.PointerEvent, v ColorPickerValue) (ColorPickerValue, gfx.EventResult) {
	if l.Disabled || l.Content.Width <= 0 || event.Kind != gfx.PointerDown && !(event.Kind == gfx.PointerMove && event.Captured) {
		return v, gfx.EventResult{}
	}
	if event.Kind == gfx.PointerDown && event.Button != gfx.PointerButtonPrimary && event.Button != gfx.PointerButtonNone {
		return v, gfx.EventResult{}
	}
	old := v
	switch event.Target {
	case l.IDs.Saturation:
		if l.Saturation.Width <= 0 || l.Saturation.Height <= 0 {
			return v, gfx.EventResult{}
		}
		v.Saturation = pickerUnit((event.X - l.Saturation.X) / l.Saturation.Width)
		v.Brightness = 1 - pickerUnit((event.Y-l.Saturation.Y)/l.Saturation.Height)
	case l.IDs.Hue:
		if l.Hue.Width <= 0 {
			return v, gfx.EventResult{}
		}
		v.Hue = 360 * pickerUnit((event.X-l.Hue.X)/l.Hue.Width)
	case l.IDs.Alpha:
		if !l.ShowAlpha || l.Alpha.Width <= 0 {
			return v, gfx.EventResult{}
		}
		v.Alpha = pickerUnit((event.X - l.Alpha.X) / l.Alpha.Width)
	default:
		return v, gfx.EventResult{}
	}
	return v, gfx.EventResult{Handled: true, Repaint: v != old, Focus: event.Target}
}

// Key provides two-axis arrows on the color plane and scalar slider keys on
// hue/alpha. Shift uses larger steps. Text-field and action keys stay native.
func (l ColorPickerLayout) Key(event gfx.KeyEvent, v ColorPickerValue) (ColorPickerValue, gfx.EventResult) {
	if l.Disabled || l.Content.Width <= 0 || event.Release || event.Modifiers & ^gfx.ModifierShift != 0 {
		return v, gfx.EventResult{}
	}
	old := v
	step := float32(.01)
	if event.Modifiers&gfx.ModifierShift != 0 {
		step = .1
	}
	switch event.Target {
	case l.IDs.Saturation:
		switch event.Key {
		case gfx.KeyLeft:
			v.Saturation -= step
		case gfx.KeyRight:
			v.Saturation += step
		case gfx.KeyUp:
			v.Brightness += step
		case gfx.KeyDown:
			v.Brightness -= step
		case gfx.KeyHome:
			v.Saturation = 0
		case gfx.KeyEnd:
			v.Saturation = 1
		default:
			return v, gfx.EventResult{}
		}
	case l.IDs.Hue, l.IDs.Alpha:
		value, maximum, unit, large := v.Hue, float32(360), float32(1), float32(10)
		if event.Target == l.IDs.Alpha {
			if !l.ShowAlpha {
				return v, gfx.EventResult{}
			}
			value, maximum, unit, large = v.Alpha, 1, .01, .1
		}
		next, ok := SliderKeyDownLarge(value, 0, maximum, unit, large, DirectionLTR, event.Key, event.Modifiers)
		if !ok {
			return v, gfx.EventResult{}
		}
		if event.Target == l.IDs.Hue {
			v.Hue = next
		} else {
			v.Alpha = next
		}
	default:
		return v, gfx.EventResult{}
	}
	v = v.normalized()
	return v, gfx.EventResult{Handled: true, Repaint: v != old, Focus: event.Target, FocusVisible: true}
}
