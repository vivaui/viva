package ui

import "github.com/vivaui/viva/gfx"

// ScrollState is a scroll position: one ScrollArea's, or a Table's. It lives in the view, keyed
// by the area's identity, so the application keeps nothing: declare the area with an
// identity and give the wheel to the view.
//
//	config.OnScroll = m.view.Wheel                       // the area under the pointer takes the wheel
//	f.ScrollArea(listID).Grow(1).Children(func() { ... })
//
// The area reports how tall its content and how tall it itself is each time the page
// is laid out; Wheel moves Offset by a scroll event and keeps it in range. view.ScrollState
// returns the state of an area for the calls that move it by program (ScrollTo,
// ScrollToEnd) or read it (Offset, AtEnd): ask for it when you need it and do not keep
// the pointer, because the state of an area that a frame does not declare is dropped
// (its offset starts again at the top when the area comes back, as for a closed
// dialog or another tab) and its memory is reused for another area.
//
// A log or a chat that grows at the bottom is declared with Follow: the area then stays
// at the end as content is added, until the user scrolls up, and sticks to the end
// again when the user scrolls back down to it.
//
// An area declared Sideways also scrolls left and right: OffsetX, ContentWidth and
// ViewportWidth are its horizontal Offset, Content and Viewport, MaxX and ScrollToX its
// Max and ScrollTo. In any other area OffsetX is 0 and ContentWidth is ViewportWidth.
type ScrollState struct {
	Offset   float32
	Content  float32 // the height of everything inside, as of the last layout
	Viewport float32 // the height of the area itself, less its padding

	OffsetX       float32 // how far a Sideways area is scrolled left
	ContentWidth  float32 // the width of everything inside, as of the last layout
	ViewportWidth float32 // the width of the area itself, less its padding

	id       gfx.ID  // the area's identity, which its scrollbars are named under
	grab     float32 // where along the thumb being dragged the pointer holds it
	follow   bool    // Box.Follow: keep the area at its end while the user has not left it
	detached bool    // follow is set and the user scrolled up
	toEnd    bool    // ScrollToEnd was called and no layout has honored it yet
	sideways bool    // Box.Sideways: the area scrolls left and right too
	bars     bool    // Box.Scrollbars: the area draws its own scrollbars
	dragging bool    // a press on one of the area's scrollbars has the pointer
	barSize  float32 // ScrollbarSize: the bars' thickness; zero is the hover default
}

// scrollStore keeps the ScrollStates of a view, by the identity of their areas. A slot
// not asked for in a frame is dropped when the frame ends and reused by the next new
// area, so the store holds the areas of the page and no more, and a warm page asks for
// no memory.
type scrollStore struct {
	slots map[gfx.ID]*scrollSlot
	free  []*scrollSlot
	epoch uint32 // the frame being declared; a slot is live if it was asked for in it
}

type scrollSlot struct {
	state ScrollState
	seen  uint32
}

func (s *scrollStore) get(id gfx.ID) *ScrollState {
	slot := s.slots[id]
	if slot == nil {
		if last := len(s.free) - 1; last >= 0 {
			slot, s.free = s.free[last], s.free[:last]
		} else {
			slot = new(scrollSlot)
		}
		if s.slots == nil {
			s.slots = make(map[gfx.ID]*scrollSlot)
		}
		s.slots[id] = slot
	}
	slot.seen = s.epoch
	return &slot.state
}

// sweep drops the areas the frame that just ended did not declare.
func (s *scrollStore) sweep() {
	for id, slot := range s.slots {
		if slot.seen != s.epoch {
			delete(s.slots, id)
			slot.state = ScrollState{}
			s.free = append(s.free, slot)
		}
	}
}

// ScrollState returns the state of the ScrollArea with this identity, for ScrollTo,
// ScrollToEnd, Offset and AtEnd. The state is the view's: ask again each time instead
// of keeping the pointer, and ask only for areas the page declares. An area that was
// not declared in a frame loses its state at the end of it.
func (view *view) ScrollState(id gfx.ID) *ScrollState {
	if view.frame == nil {
		view.frame = new(frame) // a state may be set before the first paint
	}
	return view.frame.scrolls.get(id)
}

// Max is how far the area can scroll.
func (s *ScrollState) Max() float32 { return max(s.Content-s.Viewport, 0) }

// MaxX is how far a Sideways area can scroll left; zero for any other area.
func (s *ScrollState) MaxX() float32 { return max(s.ContentWidth-s.ViewportWidth, 0) }

// Dragging reports whether the pointer is dragging one of the area's bars.
func (s *ScrollState) Dragging() bool { return s.dragging }

// AtEnd reports whether the area shows its last row, as of the last layout.
func (s *ScrollState) AtEnd() bool { return s.Offset >= s.Max()-0.5 }

// Pinned reports whether the next layout puts the area at its end, as ScrollToEnd or
// Follow asked, even though Offset has not caught up yet. Read it before the frame is
// laid out; AtEnd answers the same question after it, for a page that decides what to
// draw from where the area will be rather than from where it is.
func (s *ScrollState) Pinned() bool { return s.pinned() }

// ScrollToEnd scrolls to the end of the content in the next layout, even if that grows
// in the same frame (add the message, then call it). With Follow set it also makes the
// area follow the end again.
func (s *ScrollState) ScrollToEnd() { s.toEnd, s.detached = true, false }

// ScrollTo scrolls to offset pixels from the top, kept in range by the next layout.
func (s *ScrollState) ScrollTo(offset float32) {
	s.toEnd = false
	s.Offset = max(offset, 0)
	s.settle()
}

// ScrollToX scrolls a Sideways area to offset pixels from its left edge, kept in range
// by the next layout. It has no effect on any other area.
func (s *ScrollState) ScrollToX(offset float32) { s.OffsetX = max(offset, 0) }

// Follow makes the area keep to its end as content is added, or stop: the
// imperative form of Box.Follow, for a gesture between paints. A List reads
// whether it follows when it is declared, before the Box it returns can say
// Follow(false), so an application that stops following as the user grabs a
// scrollbar thumb must say so on the state: otherwise the rows that stream in
// before the next declaration's Follow(false) reach it are pinned to the end.
func (s *ScrollState) Follow(on bool) {
	s.follow = on
	if !on {
		s.detached = false
	}
}

// settle records whether a following area was left before its end.
func (s *ScrollState) settle() {
	if s.follow {
		s.detached = !s.AtEnd()
	}
}

// pinned says the next layout puts the area at its end.
func (s *ScrollState) pinned() bool { return s.toEnd || s.follow && !s.detached }

// canScroll says whether a wheel that moves the area by dx and dy would move it.
func (s *ScrollState) canScroll(dx, dy float32) bool {
	return dy > 0 && s.Offset < s.Max()-0.01 || dy < 0 && s.Offset > 0.01 ||
		dx > 0 && s.OffsetX < s.MaxX()-0.01 || dx < 0 && s.OffsetX > 0.01
}

// wheelSteps is how far a scroll event moves the area, right and down. A Sideways area
// takes the wheel with Shift held, and no horizontal delta, as a horizontal wheel.
func (s *ScrollState) wheelSteps(event gfx.ScrollEvent) (dx, dy float32) {
	dx, dy = event.DeltaX, event.DeltaY
	if s.sideways && dx == 0 && event.Modifiers&gfx.ModifierShift != 0 {
		dx, dy = dy, 0
	}
	return wheelStep(dx, event.Precise), wheelStep(dy, event.Precise)
}

// wheelStep is how far a wheel delta scrolls: a mouse wheel notch moves at least a
// line, a trackpad exactly what it reports.
func wheelStep(delta float32, precise bool) float32 {
	if !precise {
		if delta > 0 && delta < 24 {
			return 24
		} else if delta < 0 && delta > -24 {
			return -24
		}
	}
	return delta
}

// Wheel applies a scroll event: a positive delta reveals content below, a mouse wheel
// notch moves at least a line and a trackpad exactly what it reports. A Sideways area
// also moves left and right, by the horizontal wheel or by the wheel with Shift held.
// It is the function to give as Config.OnScroll when one area owns the wheel; a page
// with several areas gives view.Wheel instead, which finds the one under the pointer.
func (s *ScrollState) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	dx, dy := s.wheelSteps(event)
	before, beforeX := s.Offset, s.OffsetX
	s.Offset = min(max(before+dy, 0), s.Max())
	s.OffsetX = min(max(beforeX+dx, 0), s.MaxX())
	s.toEnd = false
	s.settle()
	return gfx.EventResult{Handled: true, Repaint: s.Offset != before || s.OffsetX != beforeX}
}

// Wheel gives a scroll event to the ScrollArea of the page last drawn that the pointer
// (event.X, event.Y) is over: the innermost one when areas nest, and the next one out
// when that is already at the end the wheel points to, as a browser does. Areas side
// by side each scroll alone. A sideways wheel goes the same way, to the innermost
// Sideways area that can move along it. A wheel over a ScrollAt area goes to the
// application's own Config.OnScroll instead: ScrollAt declares the area, and the
// event is handed back unhandled rather than bubbled to an outer ScrollArea. It is
// the function to give as Config.OnScroll for a page whose areas are all
// ScrollAreas; handle a Select's open list and a self-scrolling Table first, as
// they own the wheel while they are open or under the pointer. It reports an event
// that no area under the pointer can use as not handled.
func (view *view) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	f := view.frame
	if f == nil {
		return gfx.EventResult{}
	}
	for i := len(f.nodes) - 1; i >= 0; i-- {
		n := &f.nodes[i]
		if n.ext == 0 || f.exts[n.ext].scrollState == nil {
			continue
		}
		if !f.containsPoint(i, event.X, event.Y) {
			continue
		}
		if f.exts[n.ext].scrollManual {
			return gfx.EventResult{}
		}
		s := f.exts[n.ext].scrollState
		if !s.canScroll(s.wheelSteps(event)) {
			continue
		}
		return s.Wheel(event)
	}
	return gfx.EventResult{}
}

// containsPoint says whether a point is inside element i and inside every clip around
// it, which is to say, whether it is where the element can be seen.
func (f *frame) containsPoint(i int, x, y float32) bool {
	for p := int32(i); p >= 0; p = f.nodes[p].parent {
		n := &f.nodes[p]
		if p != int32(i) && n.flags&flagClip == 0 {
			continue
		}
		r := n.rect
		if x < r.X || x >= r.X+r.Width || y < r.Y || y >= r.Y+r.Height {
			return false
		}
	}
	return true
}

// ScrollArea is Scroll whose offset the view keeps under id, with the height of the
// content and of the area up to date after each layout, and the offset clamped to them
// after a window resize or a change of content. The id is the area's identity (a
// tree.ID or one item of a Scope, as for a control); declare one area for each id.
// Sideways makes it scroll left and right too, and Scrollbars draws its scrollbars.
//
//	f.ScrollArea(listID).Grow(1).Gap(4).Children(func() { ... })
//	f.ScrollArea(logID).Grow(1).Follow(true).Children(func() { ... }) // a log that stays at its end
//	f.ScrollArea(codeID).Grow(1).Sideways().Scrollbars().Children(func() { ... }) // both ways, with bars
func (f *frame) ScrollArea(id gfx.ID) Box {
	if id.IsZero() {
		panic("ui: ScrollArea needs an identity, for example f.ScrollArea(listID) with listID = tree.ID(page, \"list\")")
	}
	state := f.scrolls.get(id)
	state.id, state.follow, state.sideways, state.bars, state.barSize = id, false, false, false, 0
	b := f.Scroll(state.Offset)
	f.extra(b.i).scrollState = state
	return b
}

// Follow(true) keeps a ScrollArea at its end as content is added, until the user
// scrolls up, and again once they scroll back down to it: a log, a chat. It starts at
// the end. Follow(false), the default, leaves the offset to the user and to ScrollTo.
func (b Box) Follow(on bool) Box {
	if n := b.n(); n.ext != 0 {
		if s := b.f.exts[n.ext].scrollState; s != nil {
			s.Follow(on)
		}
	}
	return b
}

// Sideways makes a ScrollArea scroll left and right as well as up and down. Its
// children are laid out as wide as they ask to be, the widest of them setting how
// wide the content is, and as wide as that content, or as the area when that is
// wider: a row of tabs keeps the width of its tabs, and the rows of a list are all as
// wide as the longest, so a highlight runs across all of it. A Paragraph does not
// wrap in it, as there is no width to wrap to. Like its height, the area's width is
// what it is given (a Column stretches it, a Row needs Grow), not its content's,
// unless it has a MaxWidth. view.Wheel moves it by the horizontal wheel or by the
// wheel with Shift held; ScrollState().OffsetX says how far it is scrolled. It has no
// effect on a box that is not a ScrollArea.
func (b Box) Sideways() Box {
	if s := b.areaState(); s != nil {
		s.sideways = true
	}
	return b
}

// Scrollbars draws a ScrollArea's scrollbars: thin bars over its right and bottom
// edges, along whichever way its content overflows, which the pointer drags. A press
// on a bar beside its thumb brings the middle of the thumb there and drags it on.
// Without it, scrollbars are the application's, as they were. It has no effect on a
// box that is not a ScrollArea. The bars are revealed while the pointer is over the
// area or a bar is dragged; a view drawn without a Page has no pointer, and keeps
// them revealed.
func (b Box) Scrollbars() Box {
	if s := b.areaState(); s != nil {
		s.bars = true
		b.f.paintBarsFn = (*frame).paintScrollbars
	}
	return b
}

// ScrollbarSize sets the thickness of the scrollbars Scrollbars draws: a vertical
// bar's width or a horizontal bar's height. Zero keeps the thin hover default.
// Like Scrollbars, it has no effect on a box that is not a ScrollArea.
func (b Box) ScrollbarSize(size float32) Box {
	if s := b.areaState(); s != nil {
		s.barSize = size
	}
	return b
}

// areaState is the state of the ScrollArea b is, or nil for any other box.
func (b Box) areaState() *ScrollState {
	if n := b.n(); n.ext != 0 {
		return b.f.exts[n.ext].scrollState
	}
	return nil
}

// ScrollState returns the scroll position of a scroll area: the one ui.Scroll
// declares, or the area of a List or a Tree. Read Offset, Content and Viewport (as of
// the last layout), or move it by program with ScrollTo and ScrollToEnd, for
// instance to bring the selected row into view:
//
//	area := ui.Scroll(c).Grow(1)
//	if m.revealSelection {
//		area.ScrollState().ScrollTo(float32(m.selected) * rowHeight)
//	}
//
// Ask again in each paint instead of keeping the pointer: the state of an area that a
// paint does not declare is dropped, and its offset starts at the top when the area
// comes back, as for a closed dialog or another tab.
func (b Box) ScrollState() *ScrollState {
	n := b.n()
	if n.ext == 0 || b.f.exts[n.ext].scrollState == nil {
		panic("ui: ScrollState is for a scroll area: ui.Scroll, or the area of a List or a Tree")
	}
	return b.f.exts[n.ext].scrollState
}

// reportScrolls tells each ScrollArea's state how tall its content and itself are, and
// clamps the offset to them; the content was laid out at the offset it had, so a
// state that was out of range is corrected for the next frame (the first one still
// paints at the old offset, which Wheel never produces). An area at its end, or
// following it, was laid out there: its offset is the new end. The width of a
// Sideways area's content was reported when it was measured, and the layout already
// kept OffsetX in range of it.
func (f *frame) reportScrolls() {
	for i := range f.nodes {
		n := &f.nodes[i]
		if n.ext == 0 || f.exts[n.ext].scrollState == nil {
			continue
		}
		s := f.exts[n.ext].scrollState
		is, ie := n.inset(axisY)
		s.Content, s.Viewport = n.content, max(n.rect.Height-is-ie, 0)
		if s.pinned() {
			s.Offset = s.Max()
			s.toEnd = false
		} else {
			s.Offset = min(max(s.Offset, 0), s.Max())
		}
		xs, xe := n.inset(axisX)
		s.ViewportWidth = max(n.rect.Width-xs-xe, 0)
		if !s.sideways {
			s.ContentWidth, s.OffsetX = s.ViewportWidth, 0
		}
		s.OffsetX = min(max(s.OffsetX, 0), s.MaxX())
	}
}

// sidewaysState is the state of a scroll area that scrolls left and right too, or nil.
func (f *frame) sidewaysState(n *node) *ScrollState {
	if s := f.exts[n.ext].scrollState; s != nil && s.sideways {
		return s
	}
	return nil
}
