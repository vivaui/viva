package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

func measureSelect(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetSelects()[n.Aux]
	widest := d.GetState().WidestOption(f.Surface, d.GetLabels(), f.Palette.Metrics.FontSizeM)
	height := f.Palette.Metrics.ControlHeight
	switch {
	case n.ButtonSize == components.ButtonSizeXSmall:
		height -= 8
	case n.Flags&FlagSmall != 0:
		height -= 4
	}
	n.Pref = [2]float32{widest + f.Palette.Metrics.PadControlX + 36, height}
}

func paintSelect(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetSelects()[n.Aux]
	count := len(d.GetLabels())
	// A Select is as tall as it measures unless the application gave it a height,
	// which then sizes the trigger itself.
	height := n.Pref[AxisY]
	if n.Size[AxisY].Kind != DimAuto {
		height = n.Rect.Height
	}
	trigger := Rect{X: n.Rect.X, Y: n.Rect.Y, Width: n.Rect.Width, Height: height}
	options := components.SelectOptions{
		TriggerID: d.TriggerID(count), Label: n.Label, Placeholder: n.Text, Hint: d.GetHint(),
		Disabled: n.Flags&FlagDisabled != 0, Invalid: n.Flags&FlagInvalid != 0,
		Layer: d.GetLayer(), Scope: d.GetScope().At(count + 2), DismissID: d.GetScope().At(count + 1), Viewport: f.GetBounds(),
	}
	switch {
	case n.ButtonSize == components.ButtonSizeXSmall:
		options.Size = components.SelectSizeXSmall
	case n.Flags&FlagSmall != 0:
		options.Size = components.SelectSizeSmall
	}
	options.Unpainted = n.Flags&FlagThemed == 0
	if d.GetSelected() >= 0 && d.GetSelected() < count {
		options.Value = d.GetLabels()[d.GetSelected()]
		if d.GetSelected() < len(d.GetIcons()) {
			options.Icon = d.GetIcons()[d.GetSelected()]
		}
	}
	if options.Label == "" {
		options.Label = n.Text
	}
	open := d.GetPopup() >= 0
	if open {
		list := f.Nodes[d.GetPopup()].Rect
		list.Width = max(list.Width, trigger.Width)
		options.Open, options.Content = true, list
	}
	control := components.BeginSelect(f.Surface, f.Palette, trigger, options)
	if !open {
		return
	}
	// The list is as wide as its trigger and as the component's minimum at least. The element
	// that places it says so too, so what a caller draws on it (SelectBase) is the panel the
	// options are laid in.
	if panel := control.ContentBounds(); panel.Width > 0 {
		f.Nodes[d.GetPopup()].Rect = panel
	}
	first, visible := d.GetState().First, d.Visible
	if visible < count {
		control.AddScrollUp(d.GetScope().At(count+3), first == 0)
	}
	// The rows' rectangles are kept for the frame, for a list the caller paints (Part).
	d.SetRects(int32(len(base.WidgetsOf(f).GetRows())))
	for range count {
		base.WidgetsOf(f).SetRows(append(base.WidgetsOf(f).GetRows(), Rect{}))
	}
	for index := first; index < first+visible && index < count; index++ {
		item := components.SelectItem{ID: d.GetScope().At(index), Label: d.GetLabels()[index], Selected: index == d.GetSelected(), Disabled: base.OptionDisabled(d.GetDisabled(), index)}
		if index < len(d.GetIcons()) {
			item.Icon = d.GetIcons()[index]
		}
		base.WidgetsOf(f).GetRows()[int(d.GetRects())+index] = control.AddItem(item)
	}
	if visible < count {
		control.AddScrollDown(d.GetScope().At(count+4), first+visible >= count)
	}
}
