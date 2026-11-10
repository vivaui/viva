package ui

import "github.com/vivaui/viva/gfx"

// DragState is one drag-and-drop gesture of a page: which element a press takes
// hold of, which declared target the pointer is over, and what the application
// does when it is let go. Keep one in the model, next to the state the callbacks
// read.
//
// Declare a source and its targets every paint, after their Hit:
//
//	ui.Row(c).Hit(ui.ID(c), "Group").DragSource(&m.drag)
//	ui.Row(c).Hit(ui.ID(c), "Folder").DropTarget(&m.drag, m.canDrop, m.dropOn)
//
// The engine owns the press, the distance that turns it into a drag, the target
// under the pointer and the release; the application only says which targets
// take a source and what a drop does. A press that does not move is left to the
// page as a click, and while a drag is over a target Target names it, so the
// page can paint the highlight.
//
// A drag starts only when the press lands on the source itself, not on a control
// inside it; declare the control as a source too if pressing it should drag.
// Sources and targets declared in a Page's view are routed by the page. A
// surface painted through its own ui.View inside the page declares them with
// DragSource and DropTarget and hands the state to the page with Drag, so the
// page routes its pointer before the page's controls. A Frame application with
// no page wires Pointer and Key into the window's handlers instead. Do not
// declare them in a Memo: a copied part would not declare them again. A tree's
// rows have their own before/inside/after drop in TreeState.
//
// A state belongs to one Frame: declare all of its sources and targets there.
// A page's own view is one Frame; a surface painted through ui.View is another.
type DragState struct {
	frame *Frame
	gen   uint32
	epoch uint32 // the page paint this state was registered for

	sources []dragSource
	targets []dragTarget

	pressed   bool
	active    bool
	cancelled bool   // a cancelled press still owns the pointer until it is released
	candidate gfx.ID // the pressed source, while it is not yet a drag
	source    gfx.ID
	target    gfx.ID
	start     gfx.Point
	at        gfx.Point
}

type dragSource struct {
	id   gfx.ID
	node int32
}

type dragTarget struct {
	id     gfx.ID
	node   int32
	accept func(source, target gfx.ID) bool
	drop   func(source, target gfx.ID)
}

// DragSource makes the element a draggable source for state. It needs a Hit
// identity, and the press must land on the element itself.
func (b Box) DragSource(state *DragState) Box {
	n := b.n()
	if n.id.IsZero() {
		panic("ui: DragSource needs a Hit identity")
	}
	if state == nil {
		panic("ui: DragSource needs its DragState")
	}
	p := b.f.page
	if p != nil {
		p.forbidInMemo("DragSource")
	}
	state.begin(b.f)
	state.sources = append(state.sources, dragSource{id: n.id, node: b.i})
	return b
}

// DropTarget makes the element a target for state. accept says whether it takes
// the current source; nil accepts every source. drop runs when the pointer is
// released over a target accept took. Both are asked with the source's and the
// target's identities, so one pair of callbacks can serve every target of the
// state. Keep them in the model: a method value made in each paint allocates.
// The target needs a Hit identity.
func (b Box) DropTarget(state *DragState, accept func(source, target gfx.ID) bool, drop func(source, target gfx.ID)) Box {
	n := b.n()
	if n.id.IsZero() {
		panic("ui: DropTarget needs a Hit identity")
	}
	if state == nil {
		panic("ui: DropTarget needs its DragState")
	}
	if drop == nil {
		panic("ui: DropTarget needs its drop callback")
	}
	p := b.f.page
	if p != nil {
		p.forbidInMemo("DropTarget")
	}
	state.begin(b.f)
	state.targets = append(state.targets, dragTarget{id: n.id, node: b.i, accept: accept, drop: drop})
	return b
}

// Drag hands state to the page that routes its pointer. It is for a surface the
// page does not declare itself: the grid a custom ui.View paints into a slot.
// Call it in the page's view, every paint, before the view that declares the
// state's sources and targets; the page then gives the state the pointer before
// its controls, as it does for a state its own boxes declared. A state declared
// with DragSource in the page's own view needs no Drag.
func Drag(c C, state *DragState) {
	if state == nil {
		panic("ui: Drag needs its DragState")
	}
	p := c.page
	p.forbidInMemo("Drag")
	if state.frame != nil && state.frame.page == p {
		return // the page's own boxes registered it
	}
	if state.epoch == p.epoch {
		return
	}
	state.epoch = p.epoch
	state.sources, state.targets = state.sources[:0], state.targets[:0]
	p.drags = append(p.drags, state)
}

// Dragging reports whether a press on a source has moved far enough to be a
// drag.
func (s *DragState) Dragging() bool { return s.active }

// Source is the identity of the element being dragged, or zero.
func (s *DragState) Source() gfx.ID { return s.source }

// Target is the identity of the target under the pointer that takes the source,
// or zero. It is set while the pointer is over it and until the drop.
func (s *DragState) Target() gfx.ID { return s.target }

// At is where the pointer is while a drag is active, or the zero point. Paint a
// drag ghost, or a line from the source, at it.
func (s *DragState) At() gfx.Point { return s.at }

// Pointer is for a Frame painted through its own ui.View, which no Page routes:
// the host calls it from the window's pointer handler. It reports whether the
// drag handled the event; call it before the host's own pointer handling, and
// give the window's key handler Key too. A state declared in a Page is routed by
// the page, and calling this for it panics.
func (s *DragState) Pointer(event gfx.PointerEvent) (gfx.EventResult, bool) {
	if s == nil || s.frame == nil {
		return gfx.EventResult{}, false
	}
	if s.frame.page != nil {
		panic("ui: a Page routes its DragState; do not call Pointer")
	}
	if s.stale() {
		if s.active {
			s.Cancel()
		} else {
			s.sources, s.targets = s.sources[:0], s.targets[:0]
		}
	}
	if s.pressed {
		result, _ := s.pointer(event)
		return result, result.Handled
	}
	s.press(event)
	return gfx.EventResult{}, false
}

// Key lets go of a Frame's drag when Escape is pressed; the host gives it the
// window's key events with Pointer.
func (s *DragState) Key(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if s == nil || event.Key != gfx.KeyEscape || event.Release || !s.active {
		return gfx.EventResult{}, false
	}
	s.Cancel()
	return gfx.EventResult{Handled: true, Repaint: true}, true
}

// Cancel lets go of a drag without dropping it. The page does this when Escape
// is pressed, or when a paint no longer declares the drag's elements. If the
// press behind the drag is still down, its release is taken too, so cancelling a
// drag does not leave a click behind.
func (s *DragState) Cancel() {
	if s == nil {
		return
	}
	if !s.pressed {
		s.reset()
		return
	}
	s.active, s.cancelled = false, true
	s.candidate, s.source, s.target = gfx.ID{}, gfx.ID{}, gfx.ID{}
	s.at = gfx.Point{}
}

// dragStartThreshold is how far a pressed source moves before it is dragged and
// stops being a click.
const dragStartThreshold = 4

// begin records the frame a paint declares the state in, and adds the state to
// the page that routes its pointer. The first declaration of a paint clears the
// ones of the last.
func (s *DragState) begin(f *Frame) {
	if s.frame == f && s.gen == f.gen {
		return
	}
	s.frame, s.gen = f, f.gen
	s.sources, s.targets = s.sources[:0], s.targets[:0]
	if p := f.page; p != nil && s.epoch != p.epoch {
		s.epoch = p.epoch
		p.drags = append(p.drags, s)
	}
}

// stale reports whether the frame painted again without declaring the state,
// and marks its declarations current so the caller can drop them.
func (s *DragState) stale() bool {
	if s.frame == nil || s.gen == s.frame.gen {
		return false
	}
	s.gen = s.frame.gen
	return true
}

// press takes the press if it landed on one of the state's sources, and reports
// whether it did. The page routes the rest of the gesture to the state.
func (s *DragState) press(event gfx.PointerEvent) bool {
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonPrimary {
		return false
	}
	for i := len(s.sources) - 1; i >= 0; i-- {
		if s.sources[i].id != event.Target {
			continue
		}
		s.pressed, s.candidate = true, event.Target
		s.start, s.at = gfx.Point{X: event.X, Y: event.Y}, gfx.Point{X: event.X, Y: event.Y}
		return true
	}
	return false
}

// pointer follows a claimed press. done reports that the gesture is over and
// the page no longer routes to the state.
func (s *DragState) pointer(event gfx.PointerEvent) (result gfx.EventResult, done bool) {
	if !s.pressed {
		return gfx.EventResult{}, true
	}
	if s.cancelled {
		switch event.Kind {
		case gfx.PointerDown:
			s.reset()
			return gfx.EventResult{}, true
		case gfx.PointerUp, gfx.PointerExit:
			s.reset()
			return gfx.EventResult{Handled: true}, true
		}
		return gfx.EventResult{Handled: true}, false
	}
	point := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerMove:
		if !s.active {
			dx, dy := point.X-s.start.X, point.Y-s.start.Y
			if dx*dx+dy*dy < dragStartThreshold*dragStartThreshold {
				return gfx.EventResult{}, false
			}
			s.active, s.source = true, s.candidate
		}
		s.at = point
		s.target = s.targetAt(point, s.source)
		return gfx.EventResult{Handled: true, Repaint: true}, false
	case gfx.PointerUp:
		if !s.active {
			s.reset()
			return gfx.EventResult{}, true // the press is still a click
		}
		source := s.source
		target := s.targetAt(point, source)
		s.reset()
		if !target.IsZero() {
			if t := s.targetOf(target); t != nil {
				t.drop(source, target)
			}
		}
		return gfx.EventResult{Handled: true, Repaint: true}, true
	case gfx.PointerExit:
		wasDragging := s.active
		s.reset()
		if wasDragging {
			return gfx.EventResult{Handled: true, Repaint: true}, true
		}
		return gfx.EventResult{}, true
	}
	return gfx.EventResult{}, false
}

func (s *DragState) reset() {
	s.pressed, s.active, s.cancelled = false, false, false
	s.candidate, s.source, s.target = gfx.ID{}, gfx.ID{}, gfx.ID{}
	s.start, s.at = gfx.Point{}, gfx.Point{}
}

// targetAt is the target under point that takes source, or zero. Targets are
// tested from the last declared, the one painted on top.
func (s *DragState) targetAt(point gfx.Point, source gfx.ID) gfx.ID {
	for i := len(s.targets) - 1; i >= 0; i-- {
		target := &s.targets[i]
		if !pointInRect(point.X, point.Y, s.hitRect(target.node)) {
			continue
		}
		if target.accept != nil && !target.accept(source, target.id) {
			continue
		}
		return target.id
	}
	return gfx.ID{}
}

func (s *DragState) targetOf(id gfx.ID) *dragTarget {
	for i := len(s.targets) - 1; i >= 0; i-- {
		if s.targets[i].id == id {
			return &s.targets[i]
		}
	}
	return nil
}

// hitRect is the rectangle the element can be pointed at, including HitOutset.
func (s *DragState) hitRect(node int32) Rect {
	n := &s.frame.nodes[node]
	r := n.rect
	if n.ext == 0 {
		return r
	}
	h := s.frame.exts[n.ext].hitOut
	return Rect{X: r.X - h[3], Y: r.Y - h[0], Width: r.Width + h[1] + h[3], Height: r.Height + h[0] + h[2]}
}

func pointInRect(x, y float32, r Rect) bool {
	return x >= r.X && x < r.X+r.Width && y >= r.Y && y < r.Y+r.Height
}
