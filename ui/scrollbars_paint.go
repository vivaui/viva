package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintScrollbars draws the bars of a ScrollArea declared with Scrollbars over its
// edges, as the thin overlay bars of components.ScrollArea, and moves the area by a
// drag on them. The bars measure the area with its padding: the content and the
// viewport each with the area's insets, so the thumb travels the area's Max and MaxX.
func paintScrollbars(f *Frame, n *Node, s *ScrollState) {
	xs, xe := n.Inset(AxisX)
	ys, ye := n.Inset(AxisY)
	content := gfx.Size{Width: s.ContentWidth + xs + xe, Height: s.Content + ys + ye}
	parts := f.Surface.Tree().Sub(s.ID.Scope(), s.ID.Name())
	vertical, horizontal := parts.At(0), parts.At(1)
	if !s.GetVerticalID().IsZero() {
		vertical = s.GetVerticalID()
	}
	if !s.GetHorizontalID().IsZero() {
		horizontal = s.GetHorizontalID()
	}
	verticalLabel, horizontalLabel := "Vertical scrollbar", "Horizontal scrollbar"
	if s.GetVerticalLabel() != "" {
		verticalLabel = s.GetVerticalLabel()
	}
	if s.GetHorizontalLabel() != "" {
		horizontalLabel = s.GetHorizontalLabel()
	}
	size := s.GetBarSize()
	if size <= 0 && f.GetPage() != nil {
		size = f.GetPage().GetConfig().ScrollbarSize
	}
	// A scroll area without a Page has no pointer to rest on it: a view's bars
	// stay revealed, as they did before pages tracked the pointer. With one, the
	// bars are revealed while the pointer is over the area or a bar is dragged,
	// and stay revealed for a moment after the pointer leaves or the view moves,
	// fading out as they go.
	theme, revealed := f.Palette, float32(1)
	if f.GetPage() != nil {
		now := f.Surface.Now()
		if !s.GetBarSeen() {
			// An area first painted with an offset already set is not moving:
			// only a change seen across two paints lights the bars.
			s.SetBarSeenTime(true)
			s.SetBarOffset(s.Offset)
			s.SetBarOffsetXValue(s.OffsetX)
		}
		if s.Offset != s.GetBarOffset() || s.OffsetX != s.GetBarOffsetX() {
			s.SetBarOffset(s.Offset)
			s.SetBarOffsetXValue(s.OffsetX)
			s.SetBarSinceTime(now)
			s.SetBarLit(true)
		}
		target := float32(0)
		switch {
		case s.GetBarsAlways() || s.GetDragging() || f.GetPage().PointerOver(n.Rect):
			target = 1
		case s.GetBarLit():
			if remaining := ScrollbarLinger - (now - s.GetBarSince()); remaining > 0 {
				target = 1
				f.Surface.RequestFrameIn(remaining)
			} else {
				s.SetBarLit(false)
			}
		}
		if target > 0 {
			// The bars appear at once; only their leaving is animated.
			revealed = f.Surface.Animate(vertical, SlotScrollbarFade, target, gfx.Motion{})
		} else {
			revealed = f.Surface.Animate(vertical, SlotScrollbarFade, target, gfx.DefaultMotion())
		}
		theme.Border.Alpha *= revealed
		theme.MutedForeground.Alpha *= revealed
	}
	area := components.BeginScrollArea(f.Surface, theme, n.Rect, content).
		Offset(s.OffsetX, s.Offset).
		ScrollbarSize(size).
		HoverScrollbars(revealed > 0.001).
		VerticalScrollbar(vertical, verticalLabel, s.GetVerticalValue()).
		HorizontalScrollbar(horizontal, horizontalLabel, s.GetHorizontalValue())
	geometry := area.Geometry()
	if geometry.MaxOffsetY > 0 {
		f.DragScrollbar(s, vertical, geometry, AxisY)
	}
	if geometry.MaxOffsetX > 0 {
		f.DragScrollbar(s, horizontal, geometry, AxisX)
	}
	area.EndContent()
}
