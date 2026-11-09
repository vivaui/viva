package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// rover is a Choice or a Tabs strip declared in the last paint, whose arrow keys
// move between its options.
type rover struct {
	f    *Frame
	node int32
	tabs bool
}

// rove records that node, a Choice or a Tabs, takes arrow keys.
func (p *Page) rove(f *Frame, node Box, tabs bool) {
	p.rovers = append(p.rovers, rover{f: f, node: node.i, tabs: tabs})
	if p.roveRoute == nil {
		p.keyRoute, p.roveRoute = (*Page).routeKey, (*Page).roveKey
	}
}

// roveKey moves between the options of the Choice or Tabs strip the focus is in:
// Left and Right (Up and Down for vertical tabs) to the next enabled option, Home and
// End to the first and last. The focus moves and the option it moves to is chosen,
// as for a radio group and for tabs that activate on focus; the choice is delivered as
// a click on it, so Changed and the bound value follow.
func (p *Page) roveKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	for _, r := range p.rovers {
		n := &r.f.nodes[r.node]
		var next gfx.ID
		var ok bool
		if r.tabs {
			d := &r.f.widgets.tabs[n.aux]
			if _, inside := event.Target.In(d.scope); !inside {
				continue
			}
			options := make([]components.TabOption, len(d.labels))
			for i, label := range d.labels {
				options[i] = components.TabOption{ID: d.scope.At(i), Label: label}
			}
			orientation := components.TabsHorizontal
			if d.vertical {
				orientation = components.TabsVertical
			}
			next, ok = components.TabsKeyDown(event.Target, options, orientation, components.DirectionLTR, event.Key)
		} else {
			options := r.f.segmented[n.aux].options
			if !hasOption(options, event.Target) {
				continue
			}
			next, ok = components.SegmentedControlKeyDown(event.Target, options, components.DirectionLTR, event.Key)
		}
		if !ok {
			return gfx.EventResult{}, false
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: next, FocusVisible: true, Activate: next}, true
	}
	return gfx.EventResult{}, false
}

func hasOption(options []components.SegmentOption, id gfx.ID) bool {
	for i := range options {
		if options[i].ID == id {
			return true
		}
	}
	return false
}
