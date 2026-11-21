package style

import "github.com/vivaui/viva/gfx"

// ToggleGeometry is what a toggle's size gives it at a theme's metrics: its
// height, the padding either side of its content and the size of its label.
// The width is at least the height (shadcn's min-w), which the layout applies
// as the element's MinS.
func ToggleGeometry(m gfx.Metrics, size Size) (height, padding, fontSize float32) {
	height, padding, fontSize = m.ControlHeight, 10, m.FontSizeM
	switch size {
	case SizeSmall:
		height = m.ControlHeight - 4
	case SizeLarge:
		height = m.ControlHeight + 4
	}
	if height < 20 {
		height = 20
	}
	return height, padding, fontSize
}
