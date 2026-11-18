package ui

import (
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// scrollbarLinger is how long a scroll area's bars stay after the pointer leaves
// the area or the view stops moving, as the overlay bars of a desktop do; the
// bars then fade out over DefaultMotion. A bar that is dragged stays revealed.
const scrollbarLinger = time.Second

// slotScrollbarFade is the animated value of an area's bars, among those of the
// area's identity. It is below the state transitions' slots: the bars animate
// their own identity, not a control's.
const slotScrollbarFade uint8 = 200

// paintScrollbars draws the bars of a ScrollArea declared with Scrollbars over its
// edges, as the thin overlay bars of components.ScrollArea, and moves the area by a
// drag on them. The bars measure the area with its padding: the content and the
// viewport each with the area's insets, so the thumb travels the area's Max and MaxX.
func (f *frame) paintScrollbars(n *node, s *ScrollState) {
	xs, xe := n.inset(axisX)
	ys, ye := n.inset(axisY)
	content := gfx.Size{Width: s.ContentWidth + xs + xe, Height: s.Content + ys + ye}
	parts := f.canvas.Tree().Sub(s.id.Scope(), s.id.Name())
	vertical, horizontal := parts.At(0), parts.At(1)
	if !s.verticalID.IsZero() {
		vertical = s.verticalID
	}
	if !s.horizontalID.IsZero() {
		horizontal = s.horizontalID
	}
	verticalLabel, horizontalLabel := "Vertical scrollbar", "Horizontal scrollbar"
	if s.verticalLabel != "" {
		verticalLabel = s.verticalLabel
	}
	if s.horizontalLabel != "" {
		horizontalLabel = s.horizontalLabel
	}
	size := s.barSize
	if size <= 0 && f.page != nil {
		size = f.page.config.ScrollbarSize
	}
	// A scroll area without a Page has no pointer to rest on it: a view's bars
	// stay revealed, as they did before pages tracked the pointer. With one, the
	// bars are revealed while the pointer is over the area or a bar is dragged,
	// and stay revealed for a moment after the pointer leaves or the view moves,
	// fading out as they go.
	theme, revealed := f.theme, float32(1)
	if f.page != nil {
		now := f.canvas.Now()
		if !s.barSeen {
			// An area first painted with an offset already set is not moving:
			// only a change seen across two paints lights the bars.
			s.barSeen = true
			s.barOffset, s.barOffsetX = s.Offset, s.OffsetX
		}
		if s.Offset != s.barOffset || s.OffsetX != s.barOffsetX {
			s.barOffset, s.barOffsetX, s.barSince, s.barLit = s.Offset, s.OffsetX, now, true
		}
		target := float32(0)
		switch {
		case s.barsAlways || s.dragging || f.page.pointerOver(n.rect):
			target = 1
		case s.barLit:
			if remaining := scrollbarLinger - (now - s.barSince); remaining > 0 {
				target = 1
				f.canvas.RequestFrameIn(remaining)
			} else {
				s.barLit = false
			}
		}
		if target > 0 {
			// The bars appear at once; only their leaving is animated.
			revealed = f.canvas.Animate(vertical, slotScrollbarFade, target, gfx.Motion{})
		} else {
			revealed = f.canvas.Animate(vertical, slotScrollbarFade, target, gfx.DefaultMotion())
		}
		theme.Border.Alpha *= revealed
		theme.MutedForeground.Alpha *= revealed
	}
	area := components.BeginScrollArea(f.canvas, theme, n.rect, content).
		Offset(s.OffsetX, s.Offset).
		ScrollbarSize(size).
		HoverScrollbars(revealed > 0.001).
		VerticalScrollbar(vertical, verticalLabel, "").
		HorizontalScrollbar(horizontal, horizontalLabel, "")
	geometry := area.Geometry()
	if geometry.MaxOffsetY > 0 {
		f.dragScrollbar(s, vertical, geometry, axisY)
	}
	if geometry.MaxOffsetX > 0 {
		f.dragScrollbar(s, horizontal, geometry, axisX)
	}
	area.EndContent()
}

// dragScrollbar moves an area by the pointer events this paint delivers to one of its
// bars, as a Slider does its thumb: a press on the thumb holds it where it was taken,
// a press beside it brings the middle of the thumb under the pointer, and the press
// drags it until it is let go. The canvas captures the pointer for the press, so the
// moves come to the bar wherever the pointer is.
func (f *frame) dragScrollbar(s *ScrollState, bar gfx.ID, geometry components.ScrollAreaGeometry, axis int) {
	event, ok := f.canvas.PointerOn(bar)
	if !ok {
		return
	}
	thumb := geometry.VerticalThumb
	pointer, from, length := event.Y, thumb.Y, thumb.Height
	if axis == axisX {
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
		f.canvas.Consume()
		return
	default:
		return
	}
	if axis == axisY {
		s.ScrollTo(geometry.VerticalOffsetForThumb(pointer - s.grab))
	} else {
		s.ScrollToX(geometry.HorizontalOffsetForThumb(pointer - s.grab))
	}
	f.canvas.Consume()
}
