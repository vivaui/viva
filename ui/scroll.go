package ui

import "github.com/vivaui/viva/gfx"

// ScrollState is the scroll position of one ScrollArea. It lives in the View, keyed
// by the area's identity, so the application keeps nothing: declare the area with an
// identity and give the wheel to the View.
//
//	config.OnScroll = m.view.Wheel                       // the area under the pointer takes the wheel
//	f.ScrollArea(listID).Grow(1).Children(func() { ... })
//
// The area reports how tall its content and how tall it itself is each time the page
// is laid out; Wheel moves Offset by a scroll event and keeps it in range. View.ScrollState
// returns the state of an area for the calls that move it by program (ScrollTo,
// ScrollToEnd) or read it (Offset, AtEnd): ask for it when you need it and do not keep
// the pointer, because the state of an area that a frame does not declare is dropped
// (its offset starts again at the top when the area comes back, as for a closed
// dialog or another tab) and its memory is reused for another area.
//
// A log or a chat that grows at the bottom is declared with Follow: the area then stays
// at the end as content is added, until the user scrolls up, and sticks to the end
// again when the user scrolls back down to it.
type ScrollState struct {
	Offset   float32
	Content  float32 // the height of everything inside, as of the last layout
	Viewport float32 // the height of the area itself, less its padding

	follow   bool // Box.Follow: keep the area at its end while the user has not left it
	detached bool // follow is set and the user scrolled up
	toEnd    bool // ScrollToEnd was called and no layout has honored it yet
}

// scrollStore keeps the ScrollStates of a View, by the identity of their areas. A slot
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
// ScrollToEnd, Offset and AtEnd. The state is the View's: ask again each time instead
// of keeping the pointer, and ask only for areas the page declares. An area that was
// not declared in a frame loses its state at the end of it.
func (view *View) ScrollState(id gfx.ID) *ScrollState { return view.frame.scrolls.get(id) }

// Max is how far the area can scroll.
func (s *ScrollState) Max() float32 { return max(s.Content-s.Viewport, 0) }

// AtEnd reports whether the area shows its last row, as of the last layout.
func (s *ScrollState) AtEnd() bool { return s.Offset >= s.Max()-0.5 }

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

// settle records whether a following area was left before its end.
func (s *ScrollState) settle() {
	if s.follow {
		s.detached = !s.AtEnd()
	}
}

// pinned says the next layout puts the area at its end.
func (s *ScrollState) pinned() bool { return s.toEnd || s.follow && !s.detached }

// canScroll says whether a wheel with this delta would move the area.
func (s *ScrollState) canScroll(delta float32) bool {
	return delta > 0 && s.Offset < s.Max()-0.01 || delta < 0 && s.Offset > 0.01
}

// Wheel applies a scroll event: a positive delta reveals content below, a mouse wheel
// notch moves at least a line and a trackpad exactly what it reports. It is the
// function to give as Config.OnScroll when one area owns the wheel; a page with
// several areas gives View.Wheel instead, which finds the one under the pointer.
func (s *ScrollState) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	step := event.DeltaY
	if !event.Precise {
		if step > 0 && step < 24 {
			step = 24
		} else if step < 0 && step > -24 {
			step = -24
		}
	}
	before := s.Offset
	s.Offset = min(max(before+step, 0), s.Max())
	s.toEnd = false
	s.settle()
	return gfx.EventResult{Handled: true, Repaint: s.Offset != before}
}

// Wheel gives a scroll event to the ScrollArea of the page last drawn that the pointer
// (event.X, event.Y) is over: the innermost one when areas nest, and the next one out
// when that is already at the end the wheel points to, as a browser does. Areas side
// by side each scroll alone. It is the function to give as Config.OnScroll for a page
// whose areas are all ScrollAreas; handle a Select's open list and a self-scrolling
// Table first, as they own the wheel while they are open or under the pointer. It
// reports an event that no area under the pointer can use as not handled.
func (view *View) Wheel(event gfx.ScrollEvent) gfx.EventResult {
	f := &view.frame
	for i := len(f.nodes) - 1; i >= 0; i-- {
		n := &f.nodes[i]
		if n.ext == 0 || f.exts[n.ext].scrollState == nil {
			continue
		}
		s := f.exts[n.ext].scrollState
		if !s.canScroll(event.DeltaY) || !f.containsPoint(i, event.X, event.Y) {
			continue
		}
		return s.Wheel(event)
	}
	return gfx.EventResult{}
}

// containsPoint says whether a point is inside element i and inside every clip around
// it, which is to say, whether it is where the element can be seen.
func (f *Frame) containsPoint(i int, x, y float32) bool {
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

// ScrollArea is Scroll whose offset the View keeps under id, with the height of the
// content and of the area up to date after each layout, and the offset clamped to them
// after a window resize or a change of content. The id is the area's identity (a
// tree.ID or one item of a Scope, as for a control); declare one area for each id.
//
//	f.ScrollArea(listID).Grow(1).Gap(4).Children(func() { ... })
//	f.ScrollArea(logID).Grow(1).Follow(true).Children(func() { ... }) // a log that stays at its end
func (f *Frame) ScrollArea(id gfx.ID) Box {
	if id.IsZero() {
		panic("ui: ScrollArea needs an identity, for example f.ScrollArea(listID) with listID = tree.ID(page, \"list\")")
	}
	state := f.scrolls.get(id)
	state.follow = false
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
			s.follow = on
			if !on {
				s.detached = false
			}
		}
	}
	return b
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
// following it, was laid out there: its offset is the new end.
func (f *Frame) reportScrolls() {
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
	}
}
