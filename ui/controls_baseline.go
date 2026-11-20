package ui

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// controlBaseline is the label's baseline of a button, for a row that lines
// elements up by it. The engine asks the hook; the control layer answers with
// the label size the button's size gives, which is the size both its
// measurement and its painting use.
var controlBaseline = func(f *Frame, n *Node) float32 {
	theme := f.Palette
	if n.HasOwnTheme() {
		theme = f.OwnTheme(n)
	}
	_, _, fontSize, _ := style.ButtonGeometry(theme.Metrics, n.ButtonSize)
	if n.FontSize > 0 {
		fontSize = n.FontSize
	}
	return f.Surface.LabelBaseline(
		gfx.Rect{Height: n.Pref[AxisY]},
		gfx.TextStyle{Size: fontSize},
	)
}
