package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

func paintTabs(f *Frame, n *Node) {
	t := &base.WidgetsOf(f).GetTabs()[n.Aux]
	// The strip is as tall as the element: an explicit height is the trigger's,
	// and the line variant's measured height keeps the room for its indicator
	// below the triggers.
	height := n.Rect.Height
	if t.GetLine() && !t.GetVertical() {
		height = max(height-TabIndicator, 0)
	}
	bounds := Rect{X: n.Rect.X, Y: n.Rect.Y, Width: n.Rect.Width, Height: height}
	if t.GetVertical() {
		if t.GetLine() {
			bounds.Width = max(bounds.Width-TabIndicator-1, 0)
		}
	}
	var tabs components.Tabs
	if icon := t.GetIcon(); icon != nil {
		widgets := base.WidgetsOf(f)
		labels := t.GetLabels()
		widgets.TabOptions = widgets.TabOptions[:0]
		for index, label := range labels {
			widgets.TabOptions = append(widgets.TabOptions, base.TabOption{
				ID: t.GetScope().At(index), Label: label, Icon: icon(index),
			})
		}
		selected := gfx.ID{}
		if s := t.GetSelected(); s >= 0 && s < len(widgets.TabOptions) {
			selected = widgets.TabOptions[s].ID
		}
		tabs = components.BeginTabs(f.Surface, f.Palette, bounds, widgets.TabOptions, selected)
	} else {
		tabs = components.BeginTabList(f.Surface, f.Palette, bounds, t.GetScope(), t.GetLabels(), t.GetSelected())
	}
	if t.GetLine() {
		tabs = tabs.Line()
	}
	if t.GetVertical() {
		tabs = tabs.Vertical()
	}
	if n.Flags&FlagDisabled != 0 {
		tabs = tabs.Disabled()
	}
	tabs.Paint()
}
