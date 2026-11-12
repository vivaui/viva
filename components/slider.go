package components

import "github.com/vivaui/viva/gfx"

// SliderThumbValue is one stable thumb in a range slider. Keep IDs and order
// stable; each enabled thumb becomes its own focusable hit.
type SliderThumbValue struct {
	ID        gfx.ID
	Label     string
	ValueText string
	Value     float32
	Disabled  bool
}

// SliderGeometry describes the common geometry painted for one slider. Thumb
// is the first thumb for single-value convenience; ThumbCount reports all
// thumbs available through ThumbBounds.
type SliderGeometry struct {
	Bounds     gfx.Rect
	Track      gfx.Rect
	Active     gfx.Rect
	Thumb      gfx.Rect
	Value      float32
	ThumbCount int
}

// Slider paints a horizontal or vertical scalar control. Values remain
// application state: the application turns a pointer coordinate into a value
// with ValueAt (the pointer events of the held thumb carry it, with capture while
// it is dragged) and a key into one with SliderKeyDown, and passes the result back.
type Slider struct {
	canvas    *gfx.Canvas
	theme     gfx.Theme
	id        gfx.ID
	bounds    gfx.Rect
	minimum   float32
	maximum   float32
	value     float32
	valueText string
	thumbs    []SliderThumbValue
	step      float32
	vertical  bool
	disabled  bool
}

// BeginSlider starts a single-thumb 0..100 slider declaration.
func BeginSlider(canvas *gfx.Canvas, theme gfx.Theme, id gfx.ID, bounds gfx.Rect, value float32) Slider {
	return Slider{canvas: canvas, theme: theme, id: id, bounds: bounds, maximum: 100, value: value}
}

// BeginRangeSlider starts a multi-thumb 0..100 slider declaration without
// copying thumbs. The caller must keep the slice stable for the paint pass.
func BeginRangeSlider(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, thumbs []SliderThumbValue) Slider {
	return Slider{canvas: canvas, theme: theme, bounds: bounds, maximum: 100, thumbs: thumbs}
}

// Range sets the inclusive scalar range. Invalid or reversed ranges collapse
// to minimum rather than introducing undefined paint geometry.
func (slider Slider) Range(minimum, maximum float32) Slider {
	slider.minimum = minimum
	slider.maximum = maximum
	return slider
}

// Step snaps both painted and coordinate-derived values to a positive step.
func (slider Slider) Step(step float32) Slider {
	if step > 0 {
		slider.step = step
	}
	return slider
}

// ValueText supplies caller-formatted accessibility value text without
// formatting or allocating in the paint path.
func (slider Slider) ValueText(value string) Slider {
	slider.valueText = value
	return slider
}

// Vertical maps minimum to the bottom and maximum to the top.
func (slider Slider) Vertical() Slider {
	slider.vertical = true
	return slider
}

// Disabled renders and routes the slider as inert.
func (slider Slider) Disabled() Slider {
	slider.disabled = true
	return slider
}

// ThumbCount returns the stable number of painted thumbs.
func (slider Slider) ThumbCount() int {
	if len(slider.thumbs) > 0 {
		return len(slider.thumbs)
	}
	return 1
}

// ThumbBounds returns one thumb's geometry without painting.
func (slider Slider) ThumbBounds(index int) gfx.Rect {
	if index < 0 || index >= slider.ThumbCount() {
		return gfx.Rect{}
	}
	value, _, _ := slider.thumb(index)
	return slider.thumbBounds(value)
}

// ValueAt maps a caller-supplied logical coordinate to the slider range. The
// coordinate is x for horizontal and y for vertical sliders.
func (slider Slider) ValueAt(position float32) float32 {
	origin, extent := slider.travel()
	if extent <= 0 || slider.maximum <= slider.minimum {
		return slider.minimum
	}
	fraction := (position - origin) / extent
	if slider.vertical {
		fraction = 1 - fraction
	}
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	return slider.snap(slider.minimum + fraction*(slider.maximum-slider.minimum))
}

// Paint draws the track, selected range, thumbs, and stable hit regions.
func (slider Slider) Paint(label string) SliderGeometry {
	if slider.canvas == nil || (!slider.vertical && slider.bounds.Width <= 0) {
		return SliderGeometry{}
	}
	geometry := slider.geometry()
	trackColor := slider.theme.Muted
	activeColor := slider.theme.Primary
	if slider.disabled {
		trackColor.Alpha *= 0.50
		activeColor.Alpha *= 0.50
	}
	slider.canvas.Rounded(geometry.Track, geometry.Track.Height/2, trackColor)
	activeRadius := geometry.Active.Height / 2
	if slider.vertical {
		activeRadius = geometry.Active.Width / 2
	}
	slider.canvas.Rounded(geometry.Active, activeRadius, activeColor)

	for index := 0; index < slider.ThumbCount(); index++ {
		_, id, thumbDisabled := slider.thumb(index)
		thumb := slider.ThumbBounds(index)
		enabled := !slider.disabled && !thumbDisabled && !id.IsZero()
		hovered := enabled && slider.canvas.Hovered() == id
		focused := enabled && slider.canvas.FocusVisible() == id
		pressed := enabled && slider.canvas.Pressed() == id
		if focused || hovered || pressed {
			ring := slider.theme.Ring
			ring.Alpha *= 0.50
			if pressed {
				ring.Alpha = slider.theme.Ring.Alpha * 0.80 // the held thumb's halo is stronger
			}
			slider.canvas.Rounded(expand(thumb, 4), thumb.Height/2+4, ring)
		}
		thumbBorder := slider.theme.Primary
		thumbFill := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
		if slider.disabled || thumbDisabled {
			thumbBorder.Alpha *= 0.50
			thumbFill.Alpha *= 0.50
		}
		slider.canvas.Elevate(thumb, thumb.Height/2, gfx.ShadowSM) // shadow-sm
		paintBordered(slider.canvas, slider.theme, thumb, thumb.Height/2, thumbBorder, thumbFill)
		if !id.IsZero() {
			hitLabel := label
			if len(slider.thumbs) > 0 && slider.thumbs[index].Label != "" {
				hitLabel = slider.thumbs[index].Label
			}
			hit := expand(thumb, 6)
			if len(slider.thumbs) == 0 {
				hit = geometry.Bounds
			}
			slider.canvas.HitWithOptions(id, hit, hitLabel, gfx.HitOptions{
				Role: gfx.RoleSlider, Value: slider.semanticValue(index),
				Disabled: slider.disabled || thumbDisabled,
				Range:    slider.semanticRange(index),
			})
		}
	}
	return geometry
}

func (slider Slider) geometry() SliderGeometry {
	bounds := slider.resolvedBounds()
	track := slider.trackBounds()
	start, end := slider.activeFractions()
	origin, extent := slider.travel()
	// The range runs between the thumbs' centers, and out to the groove's own end
	// where a thumb rests at the minimum or maximum.
	low, high := origin+extent*start, origin+extent*end
	active := track
	if slider.vertical {
		top, bottom := origin+extent*(1-end), origin+extent*(1-start)
		if end >= 1 {
			top = track.Y
		}
		if start <= 0 {
			bottom = track.Y + track.Height
		}
		active.Y, active.Height = top, bottom-top
	} else {
		if start <= 0 {
			low = track.X
		}
		if end >= 1 {
			high = track.X + track.Width
		}
		active.X, active.Width = low, high-low
	}
	value, _, _ := slider.thumb(0)
	return SliderGeometry{
		Bounds: bounds, Track: track, Active: active,
		Thumb: slider.thumbBounds(value), Value: value, ThumbCount: slider.ThumbCount(),
	}
}

func (slider Slider) resolvedBounds() gfx.Rect {
	bounds := slider.bounds
	if slider.vertical {
		if bounds.Height <= 0 {
			bounds.Height = 160
		}
		if bounds.Width <= 0 {
			bounds.Width = 16
		}
		return bounds
	}
	if bounds.Height <= 0 {
		bounds.Height = 16
	}
	return bounds
}

// sliderThumb and sliderTrack are the thumb's diameter and the track's thickness.
const (
	sliderThumb = float32(16)
	sliderTrack = float32(6)
)

// trackBounds is the groove: the slider's whole length, as Vega's w-full track.
func (slider Slider) trackBounds() gfx.Rect {
	bounds := slider.resolvedBounds()
	if slider.vertical {
		return gfx.Rect{X: bounds.X + (bounds.Width-sliderTrack)/2, Y: bounds.Y, Width: sliderTrack, Height: bounds.Height}
	}
	return gfx.Rect{X: bounds.X, Y: bounds.Y + (bounds.Height-sliderTrack)/2, Width: bounds.Width, Height: sliderTrack}
}

// travel is the stretch of the slider's axis the thumb's center moves over: the
// groove less half a thumb at each end, so the thumb at the minimum or maximum
// sits inside the groove (Base UI's thumbAlignment="edge"). It is x and width
// for a horizontal slider, y and height for a vertical one.
func (slider Slider) travel() (origin, extent float32) {
	track := slider.trackBounds()
	if slider.vertical {
		return track.Y + sliderThumb/2, max(track.Height-sliderThumb, 0)
	}
	return track.X + sliderThumb/2, max(track.Width-sliderThumb, 0)
}

func (slider Slider) thumbBounds(value float32) gfx.Rect {
	track := slider.trackBounds()
	origin, extent := slider.travel()
	fraction := slider.fraction(value)
	if slider.vertical {
		return gfx.Rect{X: track.X + (track.Width-sliderThumb)/2, Y: origin + extent*(1-fraction) - sliderThumb/2, Width: sliderThumb, Height: sliderThumb}
	}
	return gfx.Rect{X: origin + extent*fraction - sliderThumb/2, Y: track.Y + (track.Height-sliderThumb)/2, Width: sliderThumb, Height: sliderThumb}
}

func (slider Slider) activeFractions() (float32, float32) {
	if len(slider.thumbs) == 0 {
		return 0, slider.fraction(slider.snap(slider.value))
	}
	minimum := float32(1)
	maximum := float32(0)
	for index := range slider.thumbs {
		fraction := slider.fraction(slider.snap(slider.thumbs[index].Value))
		if fraction < minimum {
			minimum = fraction
		}
		if fraction > maximum {
			maximum = fraction
		}
	}
	return minimum, maximum
}

func (slider Slider) thumb(index int) (float32, gfx.ID, bool) {
	if len(slider.thumbs) == 0 {
		return slider.snap(slider.value), slider.id, false
	}
	thumb := slider.thumbs[index]
	return slider.snap(thumb.Value), thumb.ID, thumb.Disabled
}

func (slider Slider) semanticValue(index int) string {
	if len(slider.thumbs) == 0 {
		return slider.valueText
	}
	return slider.thumbs[index].ValueText
}

// semanticRange is the number a thumb reports to assistive technology. A
// slider with no step moves by one unit when the keyboard moves it.
func (slider Slider) semanticRange(index int) gfx.ValueRange {
	value, _, _ := slider.thumb(index)
	step := slider.step
	if step <= 0 {
		step = 1
	}
	return gfx.ValueRange{Now: float64(value), Min: float64(slider.minimum), Max: float64(slider.maximum), Step: float64(step), Set: true}
}

func (slider Slider) fraction(value float32) float32 {
	if slider.maximum <= slider.minimum {
		return 0
	}
	fraction := (value - slider.minimum) / (slider.maximum - slider.minimum)
	if fraction < 0 {
		return 0
	}
	if fraction > 1 {
		return 1
	}
	return fraction
}

func (slider Slider) snap(value float32) float32 {
	return snapSliderValue(value, slider.minimum, slider.maximum, slider.step)
}

// snapSliderValue clamps value to minimum..maximum and rounds it to the nearest
// multiple of step from minimum; a zero step leaves it continuous.
func snapSliderValue(value, minimum, maximum, step float32) float32 {
	if maximum <= minimum {
		return minimum
	}
	if value < minimum {
		value = minimum
	}
	if value > maximum {
		value = maximum
	}
	if step > 0 {
		steps := (value - minimum) / step
		index := int(steps + 0.5)
		value = minimum + float32(index)*step
		if value > maximum {
			value = maximum
		}
	}
	return value
}

// SliderKeyDown applies Base UI's slider keyboard pattern to value with Base
// UI's default large step of 10 value units: the arrows move it by one step
// (Right and Up up, Left and Down down; Left and Right swap in RTL), Shift with
// an arrow and PageUp/PageDown by 10 units, Home and End to the minimum and
// maximum. A zero or negative step moves by one unit. The result is clamped and
// snapped like Slider.Step does; the returned bool reports whether key is owned
// by the slider pattern, so the caller passes anything else on.
//
//	next, handled := components.SliderKeyDown(m.volume, 0, 100, 5, components.LTR, event.Key, event.Modifiers)
func SliderKeyDown(value, minimum, maximum, step float32, direction Direction, key gfx.Key, modifiers gfx.Modifiers) (float32, bool) {
	return SliderKeyDownLarge(value, minimum, maximum, step, 10, direction, key, modifiers)
}

// SliderKeyDownLarge is SliderKeyDown with Base UI's configurable largeStep: the
// number of value units Shift+arrow and PageUp/PageDown move, not a number of
// steps. Base UI's default is 10, which SliderKeyDown uses, so a step of 5
// moves 10 units (two steps) rather than 50. A non-positive largeStep also uses
// 10. The result is snapped to step, so a large step that is not a multiple of
// step lands on the nearest one.
func SliderKeyDownLarge(value, minimum, maximum, step, largeStep float32, direction Direction, key gfx.Key, modifiers gfx.Modifiers) (float32, bool) {
	unit := step
	if unit <= 0 {
		unit = 1
	}
	if largeStep <= 0 {
		largeStep = 10
	}
	jump := unit
	if modifiers&gfx.ModifierShift != 0 {
		jump = largeStep
	}
	var next float32
	switch key {
	case gfx.KeyRight, gfx.KeyLeft:
		forward := key == gfx.KeyRight
		if direction.IsRTL() {
			forward = !forward
		}
		next = value - jump
		if forward {
			next = value + jump
		}
	case gfx.KeyUp:
		next = value + jump
	case gfx.KeyDown:
		next = value - jump
	case gfx.KeyPageUp:
		next = value + largeStep
	case gfx.KeyPageDown:
		next = value - largeStep
	case gfx.KeyHome:
		next = minimum
	case gfx.KeyEnd:
		next = maximum
	default:
		return value, false
	}
	return snapSliderValue(next, minimum, maximum, step), true
}

// RangeSliderKeyDown applies Base UI's range-slider keyboard rules to one thumb
// in a sorted values slice and reports whether it changed any value. Arrows move
// by step, Shift+arrow and PageUp/PageDown by largeStep, and Home/End go to the
// ends of the thumb's own range. Thumbs keep minSteps*step apart: moving a thumb
// pushes its neighbours, while Home and End stop at the neighbour as Base UI's
// range thumb does.
func RangeSliderKeyDown(values []float32, index int, minimum, maximum, step, largeStep float32, minSteps int, direction Direction, key gfx.Key, modifiers gfx.Modifiers) bool {
	if index < 0 || index >= len(values) {
		return false
	}
	if minSteps < 0 {
		minSteps = 0
	}
	if largeStep <= 0 {
		largeStep = 10
	}
	gap := step * float32(minSteps)
	if step <= 0 {
		gap = 0
	}
	current := values[index]
	target, handled := SliderKeyDownLarge(current, minimum, maximum, step, largeStep, direction, key, modifiers)
	if !handled {
		return false
	}
	if key == gfx.KeyHome || key == gfx.KeyEnd {
		if index > 0 && target < values[index-1]+gap {
			target = values[index-1] + gap
		}
		if index < len(values)-1 && target > values[index+1]-gap {
			target = values[index+1] - gap
		}
		target = max(min(target, maximum), minimum)
		if target == current {
			return false
		}
		values[index] = target
		return true
	}
	// A push has to leave room for every thumb on the side it moves toward.
	target = min(max(target, minimum+gap*float32(index)), maximum-gap*float32(len(values)-1-index))
	if target > current {
		for at := index + 1; at < len(values); at++ {
			required := target + gap*float32(at-index)
			if values[at] < required {
				values[at] = required
			}
		}
	} else if target < current {
		for at := index - 1; at >= 0; at-- {
			required := target - gap*float32(index-at)
			if values[at] > required {
				values[at] = required
			}
		}
	}
	if target == current && values[index] == current {
		// Neighbours may still have moved, but a key that does not move this
		// thumb is not a change when the slice was already ordered.
		return false
	}
	values[index] = target
	return true
}
