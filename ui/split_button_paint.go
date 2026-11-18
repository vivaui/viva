package ui

import (
	"github.com/vivaui/viva/components"
)

// paintSplitButton paints both halves with the page's variant, size, name and
// state, and leaves their hits to the component.
func (f *frame) paintSplitButton(v ControlView) {
	d := &widgetsOf(f).splitButtons[v.Aux]
	options := components.SplitButtonOptions{
		PrimaryID: d.primary, MenuID: d.menu,
		Label: v.Label, MenuLabel: d.menuLabel, Hint: d.hint, MenuHint: d.menuHint,
		Icon: d.icon, Variant: v.Variant, Size: v.Size,
		TriggerWidth: d.triggerWidth, Open: d.open,
		Disabled: v.Disabled,
	}
	r := v.Rect
	components.BeginSplitButton(f.Surface, f.Palette, Rect{X: r.X, Y: r.Y, Width: r.Width, Height: r.Height}, options).Paint()
}
