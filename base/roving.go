package base

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// rover is a Choice or a Tabs strip declared in the last paint, whose arrow keys
// move between its options.
type rover struct {
	f    *Frame
	Node int32
	tabs bool
}

// rove records that node, a Choice or a Tabs, takes arrow keys.
func (p *Page) rove(f *Frame, Node Box, tabs bool) {
	p.rovers = append(p.rovers, rover{f: f, Node: Node.i, tabs: tabs})
	if p.roveRoute == nil {
		p.keyRoute, p.roveRoute = (*Page).RouteKey, (*Page).RoveKey
	}
}

// slides records that state, a Slider declared in this paint, takes the keys that
// move its thumb while it has the focus.
func (p *Page) slides(state *SliderState) {
	p.sliders = append(p.sliders, state)
	if p.sliderRoute == nil {
		p.keyRoute, p.sliderRoute = (*Page).RouteKey, (*Page).SliderKey
	}
}

// sliderKey gives a key to the Slider that has the focus: arrows by a step, Shift and
// Page Up and Down by ten, Home and End to the ends of the range.
func (p *Page) SliderKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release {
		return gfx.EventResult{}, false
	}
	for _, state := range p.sliders {
		if result := state.Key(event); result.Handled {
			return result, true
		}
	}
	return gfx.EventResult{}, false
}

func hasOption(options []style.SegmentOption, ID gfx.ID) bool {
	for i := range options {
		if options[i].ID == ID {
			return true
		}
	}
	return false
}

// roveKey moves between the options of the Choice or Tabs strip the focus is in:
// Left and Right (Up and Down for vertical tabs) to the next enabled option, Home and
// End to the first and last. The focus moves and the option it moves to is chosen,
// as for a radio group and for tabs that activate on focus; the choice is delivered as
// a click on it, so Changed and the bound value follow.
func (p *Page) RoveKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	for _, r := range p.rovers {
		n := &r.f.Nodes[r.Node]
		var next gfx.ID
		var ok bool
		if r.tabs {
			d := &WidgetsOf(r.f).tabs[n.Aux]
			if _, inside := event.Target.In(d.scope); !inside {
				continue
			}
			options := make([]TabOption, len(d.labels))
			for i, label := range d.labels {
				options[i] = TabOption{ID: d.scope.At(i), Label: label}
			}
			orientation := TabsHorizontal
			if d.vertical {
				orientation = TabsVertical
			}
			next, ok = TabsKeyDown(event.Target, options, orientation, WritingLTR, event.Key)
		} else {
			options := WidgetsOf(r.f).segmented[n.Aux].options
			if !hasOption(options, event.Target) {
				continue
			}
			next, ok = SegmentedControlKeyDown(event.Target, options, WritingLTR, event.Key)
		}
		if !ok {
			return gfx.EventResult{}, false
		}
		return gfx.EventResult{Handled: true, Repaint: true, Focus: next, FocusVisible: true, Activate: next}, true
	}
	return gfx.EventResult{}, false
}
