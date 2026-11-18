package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// Key moves a focused slider with Base UI's pattern: arrows move by one Step (or a
// hundredth of the range without one), Shift+arrow and PageUp/PageDown by a large step
// of 10 value units, and Home and End take it to its ends. Give it to Config.OnKey (or
// call it from yours); it ignores every other key and every other focus. The next
// declaration's Changed reports it.
func (s *SliderState) Key(event gfx.KeyEvent) gfx.EventResult {
	if s.ID.IsZero() || event.Target != s.ID || s.off {
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

// settleSlider takes the pointer event this paint delivers, once a frame, and keeps
// the value inside its range. It runs when Changed asks, so the caller sees the new
// value at once, or else when the slider is painted.
func (f *frame) SettleSlider(n *node) {
	d := &widgetsOf(f).sliders[n.Aux]
	if d.settled {
		return
	}
	d.settled = true
	s := d.state
	before := s.Value
	s.Value = s.fit(s.Value)
	d.changed = s.moved
	s.off = n.Flags&flagDisabled != 0
	s.moved = false
	if f.Surface == nil {
		return
	}
	if n.Flags&flagDisabled != 0 {
		s.dragging = false
		return
	}
	event, ok := f.Surface.PointerOn(n.ID)
	if !ok {
		return
	}
	vertical := n.Natural > 0
	pointer := event.X
	if vertical {
		pointer = event.Y
	}
	track := components.BeginSlider(nil, f.Palette, n.ID, s.area, s.Value).Range(s.min, s.max).Step(s.step)
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
		f.Surface.Consume()
	case event.Kind == gfx.PointerMove && s.dragging:
		s.Value = track.ValueAt(pointer - s.grab)
		f.Surface.Consume()
	case event.Kind == gfx.PointerUp && s.dragging:
		s.dragging = false
		f.Surface.Consume()
	}
	if s.Value != before {
		d.changed = true
	}
}

func (f *frame) paintSlider(n *node) {
	f.SettleSlider(n)
	d := &widgetsOf(f).sliders[n.Aux]
	s := d.state
	s.area = n.Rect
	if n.Flags&flagThemed == 0 {
		// A SliderBase paints nothing of its own: it is the slider to the pointer, the
		// keyboard and assistive technology, over the element's whole rectangle.
		label := n.Label
		if label == "" {
			label = "Slider"
		}
		step := s.step
		if step <= 0 {
			step = 1
		}
		f.Surface.HitWithOptions(n.ID, n.Rect, label, gfx.HitOptions{
			Role: gfx.RoleSlider, Value: s.valueText(), Disabled: n.Flags&flagDisabled != 0,
			Range: gfx.ValueRange{Now: float64(s.Value), Min: float64(s.min), Max: float64(s.max), Step: float64(step), Set: true},
		})
		return
	}
	slider := components.BeginSlider(f.Surface, f.Palette, n.ID, n.Rect, s.Value).Range(s.min, s.max).Step(s.step).ValueText(s.valueText())
	if n.Natural > 0 {
		slider = slider.Vertical()
	}
	if n.Flags&flagDisabled != 0 {
		slider = slider.Disabled()
	}
	label := n.Label
	if label == "" {
		label = "Slider"
	}
	slider.Paint(label)
}
