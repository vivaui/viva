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
	xs, xe := n.Inset(axisX)
	ys, ye := n.Inset(axisY)
	content := gfx.Size{Width: s.ContentWidth + xs + xe, Height: s.Content + ys + ye}
	parts := f.Surface.Tree().Sub(s.ID.Scope(), s.ID.Name())
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
	theme, revealed := f.Palette, float32(1)
	if f.page != nil {
		now := f.Surface.Now()
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
		case s.barsAlways || s.dragging || f.page.PointerOver(n.Rect):
			target = 1
		case s.barLit:
			if remaining := scrollbarLinger - (now - s.barSince); remaining > 0 {
				target = 1
				f.Surface.RequestFrameIn(remaining)
			} else {
				s.barLit = false
			}
		}
		if target > 0 {
			// The bars appear at once; only their leaving is animated.
			revealed = f.Surface.Animate(vertical, slotScrollbarFade, target, gfx.Motion{})
		} else {
			revealed = f.Surface.Animate(vertical, slotScrollbarFade, target, gfx.DefaultMotion())
		}
		theme.Border.Alpha *= revealed
		theme.MutedForeground.Alpha *= revealed
	}
	area := components.BeginScrollArea(f.Surface, theme, n.Rect, content).
		Offset(s.OffsetX, s.Offset).
		ScrollbarSize(size).
		HoverScrollbars(revealed > 0.001).
		VerticalScrollbar(vertical, verticalLabel, "").
		HorizontalScrollbar(horizontal, horizontalLabel, "")
	geometry := area.Geometry()
	if geometry.MaxOffsetY > 0 {
		f.DragScrollbar(s, vertical, geometry, axisY)
	}
	if geometry.MaxOffsetX > 0 {
		f.DragScrollbar(s, horizontal, geometry, axisX)
	}
	area.EndContent()
}
