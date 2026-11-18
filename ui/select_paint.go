package ui

import (
	"github.com/vivaui/viva/components"
)

func (f *frame) measureSelect(n *node) {
	d := &widgetsOf(f).selects[n.Aux]
	widest := d.state.widestOption(f.Surface, d.labels, f.Palette.Metrics.FontSizeM)
	height := f.Palette.Metrics.ControlHeight
	switch {
	case n.ButtonSize == components.ButtonSizeXSmall:
		height -= 8
	case n.Flags&flagSmall != 0:
		height -= 4
	}
	n.Pref = [2]float32{widest + f.Palette.Metrics.PadControlX + 36, height}
}

func (f *frame) paintSelect(n *node) {
	d := &widgetsOf(f).selects[n.Aux]
	count := len(d.labels)
	// A Select is as tall as it measures unless the application gave it a height,
	// which then sizes the trigger itself.
	height := n.Pref[axisY]
	if n.Size[axisY].Kind != dimAuto {
		height = n.Rect.Height
	}
	trigger := Rect{X: n.Rect.X, Y: n.Rect.Y, Width: n.Rect.Width, Height: height}
	options := components.SelectOptions{
		TriggerID: d.triggerID(count), Label: n.Label, Placeholder: n.Text, Hint: d.hint,
		Disabled: n.Flags&flagDisabled != 0,
		Layer:    d.layer, Scope: d.scope.At(count + 2), DismissID: d.scope.At(count + 1), Viewport: f.bounds,
	}
	switch {
	case n.ButtonSize == components.ButtonSizeXSmall:
		options.Size = components.SelectSizeXSmall
	case n.Flags&flagSmall != 0:
		options.Size = components.SelectSizeSmall
	}
	options.Unpainted = n.Flags&flagThemed == 0
	if d.selected >= 0 && d.selected < count {
		options.Value = d.labels[d.selected]
		if d.selected < len(d.icons) {
			options.Icon = d.icons[d.selected]
		}
	}
	if options.Label == "" {
		options.Label = n.Text
	}
	open := d.popup >= 0
	if open {
		list := f.Nodes[d.popup].Rect
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
		f.Nodes[d.popup].Rect = panel
	}
	first, visible := d.state.First, d.Visible
	if visible < count {
		control.AddScrollUp(d.scope.At(count+3), first == 0)
	}
	// The rows' rectangles are kept for the frame, for a list the caller paints (Part).
	d.rects = int32(len(widgetsOf(f).rows))
	for range count {
		widgetsOf(f).rows = append(widgetsOf(f).rows, Rect{})
	}
	for index := first; index < first+visible && index < count; index++ {
		item := components.SelectItem{ID: d.scope.At(index), Label: d.labels[index], Selected: index == d.selected, Disabled: optionDisabled(d.disabled, index)}
		if index < len(d.icons) {
			item.Icon = d.icons[index]
		}
		widgetsOf(f).rows[int(d.rects)+index] = control.AddItem(item)
	}
	if visible < count {
		control.AddScrollDown(d.scope.At(count+4), first+visible >= count)
	}
}
