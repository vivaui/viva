package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// selectSize maps the element's size calls to the component size whose
// geometry the trigger takes.
func selectSize(n *Node) style.Size {
	switch {
	case n.ButtonSize == components.ButtonSizeXSmall:
		return style.SizeXSmall
	case n.Flags&FlagSmall != 0:
		return style.SizeSmall
	}
	return style.SizeDefault
}

// frameSelectLook is the look a Select trigger is dressed with: the page's
// Styles.Select when it set one, else the theme's.
func frameSelectLook(f *Frame, n *Node, Palette *Theme, state gfx.State, placeholder bool) (style.TextEntryLook, gfx.Color, gfx.Color) {
	if f.GetStyles() != nil && f.GetStyles().Select != nil {
		return f.GetStyles().Select(*Palette, state, placeholder)
	}
	look := components.SelectRecipe(*Palette, state, placeholder)
	return look.Surface, look.Foreground, look.Chevron
}

func measureSelect(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetSelects()[n.Aux]
	height, textSize := style.SelectGeometry(f.Palette.Metrics, selectSize(n))
	if n.FontSize > 0 {
		textSize = n.FontSize
	}
	weight := gfx.WeightRegular
	if n.WeightSet || n.Weight != 0 {
		weight = n.Weight
	}
	var widest float32
	if weight == gfx.WeightRegular && textSize == f.Palette.Metrics.FontSizeM {
		widest = d.GetState().WidestOption(f.Surface, d.GetLabels(), textSize)
	} else {
		for _, label := range d.GetLabels() {
			widest = max(widest, f.Surface.TextWidthWeight(label, textSize, weight))
		}
	}
	if len(d.GetIcons()) > 0 {
		widest += components.SelectIconSpan
	}
	if d := n.Size[AxisY]; d.Kind == DimPx {
		height = d.Value
	}
	n.Pref = [2]float32{widest + style.SelectDefaultWidth(f.Palette.Metrics), height}
}

func paintSelect(f *Frame, n *Node) {
	d := &base.WidgetsOf(f).GetSelects()[n.Aux]
	count := len(d.GetLabels())
	// A Select is as tall as it measures unless the application gave it a height,
	// which then sizes the trigger itself. A themed trigger's surface is the
	// element's rectangle, so its hit is too.
	height := n.Pref[AxisY]
	if n.Size[AxisY].Kind != DimAuto || n.Flags&FlagThemed != 0 {
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
	if n.Flags&FlagThemed != 0 {
		foreground, chevron := f.GetSelectColors()
		options.UnpaintedSurface = true
		options.UnpaintedValue = n.Flags&FlagChildren != 0
		options.TriggerContent = f.ContentRect(n)
		options.Foreground, options.HasForeground = foreground, true
		if n.Flags&FlagHasColor != 0 {
			options.Foreground = n.Color
		}
		if n.WeightSet || n.Weight != 0 {
			options.Weight = n.Weight
		}
		options.Chevron, options.HasChevron = chevron, true
		if n.FontSize > 0 {
			options.TextSize = n.FontSize
		}
		options.Disabled = f.Disabled(n)
	}
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
