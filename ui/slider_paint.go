package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func paintSlider(f *Frame, n *Node) {
	f.SettleSlider(n)
	d := &base.WidgetsOf(f).GetSliders()[n.Aux]
	s := d.GetState()
	s.SetArea(n.Rect)
	if n.Flags&FlagThemed == 0 {
		// A SliderBase paints nothing of its own: it is the slider to the pointer, the
		// keyboard and assistive technology, over the element's whole rectangle.
		label := n.Label
		if label == "" {
			label = "Slider"
		}
		step := s.GetStep()
		if step <= 0 {
			step = 1
		}
		f.Surface.HitWithOptions(n.ID, n.Rect, label, gfx.HitOptions{
			Role: gfx.RoleSlider, Value: s.ValueText(), Disabled: n.Flags&FlagDisabled != 0,
			Range: gfx.ValueRange{Now: float64(s.Value), Min: float64(s.GetMin()), Max: float64(s.GetMax()), Step: float64(step), Set: true},
		})
		return
	}
	slider := components.BeginSlider(f.Surface, f.Palette, n.ID, n.Rect, s.Value).Range(s.GetMin(), s.GetMax()).Step(s.GetStep()).ValueText(s.ValueText())
	if n.Natural > 0 {
		slider = slider.Vertical()
	}
	if n.Flags&FlagDisabled != 0 {
		slider = slider.Disabled()
	}
	label := n.Label
	if label == "" {
		label = "Slider"
	}
	slider.Paint(label)
}

// sliderPointer moves the slider under the pointer.
func sliderPointer(f *Frame, n *Node, s *SliderState) {
	event, _ := f.Surface.PointerOn(n.ID)
	vertical := n.Natural > 0
	pointer := event.X
	if vertical {
		pointer = event.Y
	}
	track := components.BeginSlider(nil, f.Palette, n.ID, s.GetArea(), s.Value).Range(s.GetMin(), s.GetMax()).Step(s.GetStep())
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
		s.SetGrab(0)
		if pointer >= middle-half && pointer <= middle+half {
			s.SetGrab(pointer - middle) // taking the thumb does not make it jump
		}
		s.SetDraggingState(true)
		s.Value = track.ValueAt(pointer - s.GetGrab())
		f.Surface.Consume()
	case event.Kind == gfx.PointerMove && s.GetDragging():
		s.Value = track.ValueAt(pointer - s.GetGrab())
		f.Surface.Consume()
	case event.Kind == gfx.PointerUp && s.GetDragging():
		s.SetDraggingState(false)
		f.Surface.Consume()
	}

}
