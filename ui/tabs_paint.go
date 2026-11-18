package ui

import (
	"github.com/vivaui/viva/components"
)

func (f *frame) paintTabs(n *node) {
	t := &widgetsOf(f).tabs[n.Aux]
	// The strip is as tall as the element: an explicit height is the trigger's,
	// and the line variant's measured height keeps the room for its indicator
	// below the triggers.
	height := n.Rect.Height
	if t.line && !t.vertical {
		height = max(height-tabIndicator, 0)
	}
	bounds := Rect{X: n.Rect.X, Y: n.Rect.Y, Width: n.Rect.Width, Height: height}
	if t.vertical {
		if t.line {
			bounds.Width = max(bounds.Width-tabIndicator-1, 0)
		}
	}
	tabs := components.BeginTabList(f.Surface, f.Palette, bounds, t.scope, t.labels, t.selected)
	if t.line {
		tabs = tabs.Line()
	}
	if t.vertical {
		tabs = tabs.Vertical()
	}
	if n.Flags&flagDisabled != 0 {
		tabs = tabs.Disabled()
	}
	tabs.Paint()
}
