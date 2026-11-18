package base

import (
	"time"

	"github.com/vivaui/viva/gfx"
)

// scrollbarLinger is how long a scroll area's bars stay after the pointer leaves
// the area or the view stops moving, as the overlay bars of a desktop do; the
// bars then fade out over DefaultMotion. A bar that is dragged stays revealed.
const ScrollbarLinger = time.Second

// slotScrollbarFade is the animated value of an area's bars, among those of the
// area's identity. It is below the state transitions' slots: the bars animate
// their own identity, not a control's.
const SlotScrollbarFade uint8 = 200

// dragScrollbar moves an area by the pointer events this paint delivers to one of its
// bars, as a Slider does its thumb: a press on the thumb holds it where it was taken,
// a press beside it brings the middle of the thumb under the pointer, and the press
// drags it until it is let go. The canvas captures the pointer for the press, so the
// moves come to the bar wherever the pointer is.
func (f *Frame) DragScrollbar(s *ScrollState, bar gfx.ID, geometry ScrollAreaGeometry, axis int) {
	event, ok := f.Surface.PointerOn(bar)
	if !ok {
		return
	}
	thumb := geometry.VerticalThumb
	pointer, from, length := event.Y, thumb.Y, thumb.Height
	if axis == AxisX {
		thumb = geometry.HorizontalThumb
		pointer, from, length = event.X, thumb.X, thumb.Width
	}
	switch {
	case event.Kind == gfx.PointerDown && event.Button <= gfx.PointerButtonPrimary:
		s.grab = length / 2
		if pointer >= from && pointer <= from+length {
			s.grab = pointer - from // taking the thumb does not make it jump
		}
		s.dragging = true
	case event.Kind == gfx.PointerMove && s.dragging:
	case event.Kind == gfx.PointerUp && s.dragging:
		s.dragging = false
		f.Surface.Consume()
		return
	default:
		return
	}
	if axis == AxisY {
		s.ScrollTo(geometry.VerticalOffsetForThumb(pointer - s.grab))
	} else {
		s.ScrollToX(geometry.HorizontalOffsetForThumb(pointer - s.grab))
	}
	f.Surface.Consume()
}
