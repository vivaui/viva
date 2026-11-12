package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

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
	// A scroll area without a Page has no pointer to rest on it: a view's bars
	// stay revealed, as they did before pages tracked the pointer.
	hinted := f.page == nil || f.page.pointerOver(n.rect)
	area := components.BeginScrollArea(f.canvas, f.theme, n.rect, content).
		Offset(s.OffsetX, s.Offset).
		ScrollbarSize(s.barSize).
		HoverScrollbars(hinted || s.dragging).
		VerticalScrollbar(vertical, "Vertical scrollbar", "").
		HorizontalScrollbar(horizontal, "Horizontal scrollbar", "")
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
