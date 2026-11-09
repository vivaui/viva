package ui

import (
	"strconv"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// sliderLength and sliderThickness are what a Slider is when it is given neither: the
// thumb is sliderThickness across, the track is as long as the container is wide (or
// sliderLength in a Row).
const (
	sliderLength    = 160
	sliderThickness = 16
	sliderDefaultTo = 100
)

// SliderState is the value of one Slider, kept in the application's state. The value
// is Value, in the range the Slider is declared with (0 to 100 unless Range says
// otherwise) and snapped to its Step; set it before the first frame, and a drag, a click
// on the track, an arrow key or a change of range keeps it inside. The rest is
// bookkeeping.
type SliderState struct {
	Value float32

	id       gfx.ID
	min, max float32 // the range as of the last declaration, for Key
	step     float32
	area     Rect // where the slider was in the last layout, for the pointer
	dragging bool
	grab     float32 // how far from the thumb's middle a drag took hold of it
	off      bool    // the slider was declared Disabled
	moved    bool    // Key changed the value and no declaration has reported it yet
	text     string  // Value as accessibility text, formatted when it changes
	textOf   float32
}

// Fraction is how far along its range the value is, 0 to 1: where a slider the caller draws
// puts its thumb.
func (s *SliderState) Fraction() float32 {
	if s.max <= s.min {
		return 0
	}
	return min(max((s.Value-s.min)/(s.max-s.min), 0), 1)
}

// Dragging reports whether the thumb is being dragged.
func (s *SliderState) Dragging() bool { return s.dragging }

// Key moves a focused slider with Base UI's pattern: arrows move by one Step (or a
// hundredth of the range without one), Shift+arrow and PageUp/PageDown by a large step
// of 10 value units, and Home and End take it to its ends. Give it to Config.OnKey (or
// call it from yours); it ignores every other key and every other focus. The next
// declaration's Changed reports it.
func (s *SliderState) Key(event gfx.KeyEvent) gfx.EventResult {
	if s.id.IsZero() || event.Target != s.id || s.off {
		return gfx.EventResult{}
	}
	unit := s.step
	if unit <= 0 {
		unit = (s.max - s.min) / 100
	}
	value, handled := components.SliderKeyDownLarge(s.Value, s.min, s.max, unit, 10, components.LTR, event.Key, event.Modifiers)
	if !handled {
		return gfx.EventResult{}
	}
	if value = s.fit(value); value != s.Value {
		s.Value, s.moved = value, true
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// fit keeps a value inside the range and on a multiple of the step from its start.
func (s *SliderState) fit(value float32) float32 {
	if s.max <= s.min {
		return s.min
	}
	value = min(max(value, s.min), s.max)
	if s.step > 0 {
		value = min(s.min+float32(int((value-s.min)/s.step+0.5))*s.step, s.max)
	}
	return value
}

// valueText is Value as the text a screen reader announces, formatted only when the
// value has changed.
func (s *SliderState) valueText() string {
	if s.text == "" || s.textOf != s.Value {
		s.text, s.textOf = strconv.FormatFloat(float64(s.Value), 'f', -1, 32), s.Value
	}
	return s.text
}

type sliderDecl struct {
	state   *SliderState
	settled bool // the pointer and the range have been dealt with in this frame
	changed bool // the user changed the value in this frame
}

// Slider adds a control that picks a number: a track with a filled part and a thumb,
// 16 across, as wide as its container in a Column and 160 wide in a Row unless given a
// Width (Vertical makes it a tall one). The value is state.Value, 0 to 100 unless
// Range says otherwise, and Step snaps it. The pointer is captured while the thumb is
// dragged, a click on the track takes the thumb there, and with the keyboard
// SliderState.Key moves it. Label names it for a screen reader, which also hears the
// value. Disabled(true) makes it inert and dimmed. Changed reports a change the user
// made, once:
//
//	if f.Slider(volumeID, &m.volume).Range(0, 1).Step(0.05).Label("Volume").Changed() {
//		m.player.SetVolume(m.volume.Value)
//	}
func (f *Frame) Slider(id gfx.ID, state *SliderState) Box {
	b := f.SliderBase(id, state)
	f.nodes[b.i].flags |= flagThemed
	return b
}

// SliderBase adds a slider without a look: the value, the pointer capture, the click on
// the track, the keyboard (SliderState.Key), the range and the step, and the slider it is
// to a screen reader, with no track or thumb painted. They are the caller's, drawn with
// Draw; State.Fraction says how far along the range the value is, and the thumb is
// sliderThickness (16) across for the pointer to take it where it is drawn. Range, Step,
// Label, Vertical, Disabled and Changed work as on Slider.
//
//	f.SliderBase(id, &m.volume).Height(16).Draw(func(d *ui.DrawContext) {
//		x := d.Rect.X + 8 + m.volume.Fraction()*(d.Rect.Width-16)
//		d.Canvas.Rounded(d.Rect, 3, trackColor)
//		d.Canvas.Rounded(Rect{X: x - 8, Y: d.Rect.Y, Width: 16, Height: 16}, 8, thumbColor)
//	})
func (f *Frame) SliderBase(id gfx.ID, state *SliderState) Box {
	b := f.box(kindSlider)
	n := &f.nodes[b.i]
	n.id = id
	n.aux = int32(len(f.widgets.sliders))
	f.widgets.sliders = append(f.widgets.sliders, sliderDecl{state: state})
	state.id, state.min, state.max, state.step = id, 0, sliderDefaultTo, 0
	return b
}

// Range sets the least and the greatest value of a Slider.
func (b Box) Range(least, greatest float32) Box {
	if n := b.n(); n.kind == kindSlider {
		s := b.f.widgets.sliders[n.aux].state
		s.min, s.max = least, max(greatest, least)
	}
	return b
}

// Step makes a Slider take only multiples of step above its least value.
func (b Box) Step(step float32) Box {
	if n := b.n(); n.kind == kindSlider {
		b.f.widgets.sliders[n.aux].state.step = max(step, 0)
	}
	return b
}

// Changed reports whether the user changed the Slider's value in this paint, by
// dragging, clicking its track or a key; call it last in the chain, after Range, Step
// and Disabled. On a control bound to a value (as a Screen's are: a Checkbox, Switch,
// Choice, Tabs, Select or Field) it reports that this paint wrote a new value into it,
// or for a Field that its text was edited since it was last declared.
func (b Box) Changed() bool {
	n := b.n()
	if n.ext != 0 && b.f.exts[n.ext].bind != 0 {
		return b.f.settleFn(b.f, b.i).changed
	}
	if n.kind != kindSlider {
		return false
	}
	b.f.settleSlider(n)
	return b.f.widgets.sliders[n.aux].changed
}

func (f *Frame) measureSlider(n *node) {
	if n.natural > 0 {
		n.pref = [2]float32{sliderThickness, sliderLength}
	} else {
		n.pref = [2]float32{sliderLength, sliderThickness}
	}
}

// settleSlider takes the pointer event this paint delivers, once a frame, and keeps
// the value inside its range. It runs when Changed asks, so the caller sees the new
// value at once, or else when the slider is painted.
func (f *Frame) settleSlider(n *node) {
	d := &f.widgets.sliders[n.aux]
	if d.settled {
		return
	}
	d.settled = true
	s := d.state
	before := s.Value
	s.Value = s.fit(s.Value)
	d.changed = s.moved
	s.off = n.flags&flagDisabled != 0
	s.moved = false
	if f.canvas == nil {
		return
	}
	if n.flags&flagDisabled != 0 {
		s.dragging = false
		return
	}
	event, ok := f.canvas.PointerOn(n.id)
	if !ok {
		return
	}
	vertical := n.natural > 0
	pointer := event.X
	if vertical {
		pointer = event.Y
	}
	track := components.BeginSlider(nil, f.theme, n.id, s.area, s.Value).Range(s.min, s.max).Step(s.step)
	if vertical {
		track = track.Vertical()
	}
	switch {
	case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
		thumb := track.ThumbBounds(0)
		middle, half := thumb.X+thumb.Width/2, thumb.Width/2
		if vertical {
			middle, half = thumb.Y+thumb.Height/2, thumb.Height/2
		}
		s.grab = 0
		if pointer >= middle-half && pointer <= middle+half {
			s.grab = pointer - middle // taking the thumb does not make it jump
		}
		s.dragging = true
		s.Value = track.ValueAt(pointer - s.grab)
		f.canvas.Consume()
	case event.Kind == gfx.PointerMove && s.dragging:
		s.Value = track.ValueAt(pointer - s.grab)
		f.canvas.Consume()
	case event.Kind == gfx.PointerUp && s.dragging:
		s.dragging = false
		f.canvas.Consume()
	}
	if s.Value != before {
		d.changed = true
	}
}

func (f *Frame) paintSlider(n *node) {
	f.settleSlider(n)
	d := &f.widgets.sliders[n.aux]
	s := d.state
	s.area = n.rect
	if n.flags&flagThemed == 0 {
		// A SliderBase paints nothing of its own: it is the slider to the pointer, the
		// keyboard and assistive technology, over the element's whole rectangle.
		label := n.label
		if label == "" {
			label = "Slider"
		}
		step := s.step
		if step <= 0 {
			step = 1
		}
		f.canvas.HitWithOptions(n.id, n.rect, label, gfx.HitOptions{
			Role: gfx.RoleSlider, Value: s.valueText(), Disabled: n.flags&flagDisabled != 0,
			Range: gfx.ValueRange{Now: float64(s.Value), Min: float64(s.min), Max: float64(s.max), Step: float64(step), Set: true},
		})
		return
	}
	slider := components.BeginSlider(f.canvas, f.theme, n.id, n.rect, s.Value).Range(s.min, s.max).Step(s.step).ValueText(s.valueText())
	if n.natural > 0 {
		slider = slider.Vertical()
	}
	if n.flags&flagDisabled != 0 {
		slider = slider.Disabled()
	}
	label := n.label
	if label == "" {
		label = "Slider"
	}
	slider.Paint(label)
}
