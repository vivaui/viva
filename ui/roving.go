package ui

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// rover is a Choice or a Tabs strip declared in the last paint, whose arrow keys
// move between its options.
type rover struct {
	f    *frame
	node int32
	tabs bool
}

// rove records that node, a Choice or a Tabs, takes arrow keys.
func (p *Page) rove(f *frame, node Box, tabs bool) {
	p.rovers = append(p.rovers, rover{f: f, node: node.i, tabs: tabs})
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
