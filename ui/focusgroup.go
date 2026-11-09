package ui

import (
	"github.com/vivaui/viva/gfx"
)

// FocusAxis is the arrow keys a FocusGroup moves the focus with.
type FocusAxis uint8

const (
	FocusHorizontal FocusAxis = iota + 1 // Left and Right
	FocusVertical                        // Up and Down
	FocusBoth                            // all four
)

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
func (b Box) FocusGroup(axis FocusAxis) Box {
	b.n()
	p := b.f.page
	if p == nil {
		panic("ui: FocusGroup is for a container of a page")
	}
	if axis != FocusHorizontal && axis != FocusVertical && axis != FocusBoth {
		panic("ui: FocusGroup needs FocusHorizontal, FocusVertical or FocusBoth")
	}
	b.f.extra(b.i).focusGroup = axis
	b.f.groupFn = (*Frame).endFocusGroup
	if p.groupRoute == nil {
		p.keyRoute, p.groupRoute = (*Page).routeKey, (*Page).groupKey
	}
	return b
}

// focusGroup is a FocusGroup painted in the last paint: its axis, and where its
// members are in Page.groupIDs.
type focusGroup struct {
	axis       FocusAxis
	start, end int32
}

// groupStop is the Tab stop of a group: the member it was on last.
type groupStop struct {
	id    gfx.ID
	epoch uint32
}

// endFocusGroup finishes painting container i, a FocusGroup, whose hits start at from:
// its members become one stop of Tab, and are kept for the arrow keys.
func (f *Frame) endFocusGroup(i int32, axis FocusAxis, from gfx.Mark) {
	p, canvas := f.page, f.canvas
	if canvas == nil || int(i) >= len(p.nodes) {
		return
	}
	key := p.nodeID(i)
	start := int32(len(p.groupIDs))
	stop, ids := canvas.OneTabStop(from, p.groupStops[key].id, p.groupIDs)
	p.groupIDs = ids
	if stop.IsZero() {
		return
	}
	if p.groupStops == nil {
		p.groupStops = map[gfx.ID]groupStop{}
	}
	p.groupStops[key] = groupStop{id: stop, epoch: p.epoch}
	p.groups = append(p.groups, focusGroup{axis: axis, start: start, end: int32(len(ids))})
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
	n := &p.nodes[i]
	return p.id(place{node: n.parentNode, scope: n.parentScope, name: n.name})
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
