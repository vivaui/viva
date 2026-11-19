package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// controlBaseline is the label's baseline of a button, for a row that lines
// elements up by it. The engine asks the hook; the control layer answers.
var controlBaseline = func(f *Frame, n *Node) float32 {
	theme := f.Palette
	if n.HasOwnTheme() {
		theme = f.OwnTheme(n)
	}
	button := components.BeginButton(f.Surface, theme, gfx.ID{}, Rect{}).Size(n.ButtonSize)
	return f.Surface.LabelBaseline(
		gfx.Rect{Height: n.Pref[AxisY]},
		gfx.TextStyle{Size: button.LabelFontSize()},
	)
}
