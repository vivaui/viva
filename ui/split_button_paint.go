package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// paintSplitButton paints both halves with the page's variant, size, name and
// state, and leaves their hits to the component.
func paintSplitButton(f *Frame, v ControlView) {
	d := &base.WidgetsOf(f).GetSplitButtons()[v.Aux]
	options := components.SplitButtonOptions{
		PrimaryID: d.GetPrimary(), MenuID: d.GetMenu(),
		Label: v.Label, MenuLabel: d.GetMenuLabel(), Hint: d.GetHint(), MenuHint: d.GetMenuHint(),
		Icon: d.GetIcon(), Variant: v.Variant, Size: v.Size,
		TriggerWidth: d.GetTriggerWidth(), Open: d.GetOpen(),
		Disabled: v.Disabled,
	}
	r := v.Rect
	components.BeginSplitButton(f.Surface, f.Palette, Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}, options).Paint()
}
