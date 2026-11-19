package base

import (
	"github.com/vivaui/viva/gfx"
)

// FocusOrder chooses the member a key moves to in a FocusGroup: current is the
// member's index in declaration order, count how many the group has. It returns
// false for a key it does not handle, and the group then uses its axis. Keep the
// function in the model; it runs on every paint and must not allocate.
//
// A grid uses it for two dimensions:
//
//	func (m *grid) order(current int, key gfx.Key, count int) (int, bool) {
//		columns := m.columns
//		switch key {
//		case gfx.KeyLeft:
//			if current%columns == 0 { return current, true }
//			return current - 1, true
//		case gfx.KeyRight:
//			if current%columns == columns-1 || current+1 >= count { return current, true }
//			return current + 1, true
//		case gfx.KeyUp:
//			return max(current-columns, 0), true
//		case gfx.KeyDown:
//			return min(current+columns, count-1), true
//		}
//		return 0, false
//	}
//
// FocusGroup makes the focusable controls inside the container one stop of Tab, as
// the buttons of a toolbar or the items of a menu bar are natively: Tab reaches the one
// that had the focus last (else the first) and goes on out of the group, and the
// arrow keys of axis move the focus to the next and the previous, wrapping, Home and
// End to the first and last. A text field, a combobox, a slider or a terminal inside
// keeps its own Tab stop and the arrows it takes, and a Control keeps the keys it
// handles.
//
//	ui.Row(c).Gap(4).FocusGroup(ui.FocusHorizontal).Children(func() {
//		for _, tool := range tools {
//			if ui.Button(c.Key(tool.Name), tool.Name).Clicked() { m.tool = tool }
//		}
//	})
//
// The arrows move the focus, not the choice. A component that chooses as the
// focus moves reads Box.State().Focused while it declares each member, as a
// roving strip does; it needs no Page-internal rover.
func (b Box) FocusGroup(axis FocusAxis) Box {
	b.N()
	p := b.f.page
	if p == nil {
		panic("ui: FocusGroup is for a container of a page")
	}
	if axis != FocusHorizontal && axis != FocusVertical && axis != FocusBoth {
		panic("ui: FocusGroup needs FocusHorizontal, FocusVertical or FocusBoth")
	}
	b.f.Extra(b.i).focusGroup = axis
	b.f.groupFn = (*Frame).endFocusGroup
	if p.groupRoute == nil {
		p.keyRoute, p.groupRoute = (*Page).RouteKey, (*Page).groupKey
	}
	return b
}

// FocusOrder gives a FocusGroup its own choice of the member each key moves to:
// a grid moves by row and column instead of by declaration order. Call it after
// FocusGroup. The function is kept as it is: keep it in the model, not in the
// paint, and make it handle the keys it wants; any other key falls back to the
// group's axis. Not inside a Memo.
func (b Box) FocusOrder(order FocusOrder) Box {
	if order == nil {
		panic("ui: FocusOrder needs a function")
	}
	if b.f.Extra(b.i).focusGroup == 0 {
		panic("ui: FocusOrder is for a FocusGroup: call FocusGroup first")
	}
	b.f.Extra(b.i).focusOrder = order
	return b
}

// focusGroup is a FocusGroup painted in the last paint: its axis, its own order
// when it has one, and where its members are in Page.groupIDs.
type focusGroup struct {
	axis       FocusAxis
	order      FocusOrder
	start, end int32
}

// groupStop is the Tab stop of a group: the member it was on last.
type groupStop struct {
	ID    gfx.ID
	epoch uint32
}

// endFocusGroup finishes painting container i, a FocusGroup, whose hits start at from:
// its members become one stop of Tab, and are kept for the arrow keys.
func (f *Frame) endFocusGroup(i int32, axis FocusAxis, from gfx.Mark) {
	p, canvas := f.page, f.Surface
	if canvas == nil || int(i) >= len(p.Nodes) {
		return
	}
	key := p.nodeID(i)
	start := int32(len(p.groupIDs))
	stop, ids := canvas.OneTabStop(from, p.groupStops[key].ID, p.groupIDs)
	p.groupIDs = ids
	if stop.IsZero() {
		return
	}
	if p.groupStops == nil {
		p.groupStops = map[gfx.ID]groupStop{}
	}
	p.groupStops[key] = groupStop{ID: stop, epoch: p.epoch}
	order := f.Exts[f.Nodes[i].Ext].focusOrder
	p.groups = append(p.groups, focusGroup{axis: axis, order: order, start: start, end: int32(len(ids))})
}

// sweepGroups forgets the Tab stop of the groups the paint did not declare.
func (p *Page) sweepGroups() {
	for key, stop := range p.groupStops {
		if stop.epoch != p.epoch {
			delete(p.groupStops, key)
		}
	}
}

// nodeID is the identity of container i, named by its place.
func (p *Page) nodeID(i int32) gfx.ID {
	n := &p.Nodes[i]
	return p.ID(Place{Node: n.parentNode, scope: n.parentScope, Name: n.Name})
}

// groupKey moves the focus between the members of the FocusGroup it is in.
func (p *Page) groupKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	for _, g := range p.groups {
		members := p.groupIDs[g.start:g.end]
		at := -1
		for i, id := range members {
			if id == event.Target {
				at = i
			}
		}
		if at < 0 {
			continue
		}
		if g.order != nil {
			if next, ok := g.order(at, event.Key, len(members)); ok {
				if next < 0 || next >= len(members) {
					panic("ui: FocusOrder returned an index outside its group")
				}
				return gfx.EventResult{Handled: true, Repaint: true, Focus: members[next], FocusVisible: true}, true
			}
		}
		horizontal, vertical := g.axis != FocusVertical, g.axis != FocusHorizontal
		next := at
		switch event.Key {
		case gfx.KeyLeft:
			if !horizontal {
				return gfx.EventResult{}, false
			}
			next = (at + len(members) - 1) % len(members)
		case gfx.KeyRight:
			if !horizontal {
				return gfx.EventResult{}, false
			}
			next = (at + 1) % len(members)
		case gfx.KeyUp:
			if !vertical {
				return gfx.EventResult{}, false
			}
			next = (at + len(members) - 1) % len(members)
		case gfx.KeyDown:
			if !vertical {
				return gfx.EventResult{}, false
			}
			next = (at + 1) % len(members)
		case gfx.KeyHome:
			next = 0
		case gfx.KeyEnd:
			next = len(members) - 1
		default:
			return gfx.EventResult{}, false
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: members[next], FocusVisible: true}, true
	}
	return gfx.EventResult{}, false
}
