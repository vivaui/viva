package style

import "github.com/vivaui/viva/gfx"

// The leading icon of a button keeps one square, one gap and one vertical inset in every
// size, so measuring and painting agree and a narrow control shrinks the icon instead of
// drawing it past its own edge.
const (
	ButtonIconSize  = 16
	ButtonIconGap   = 8
	ButtonIconInset = 8
)

// ButtonIconExtent is the side of the icon square in a button height tall.
func ButtonIconExtent(height float32) float32 {
	return max(0, min(float32(ButtonIconSize), height-ButtonIconInset))
}

// ButtonGeometry is what a size gives a button at a theme's metrics: its height, the
// padding either side of its content, the size of its label, and whether it holds an
// icon alone (then it is as wide as it is tall). A caller that asks for a Height or a
// FontSize of its own replaces those two; the rest stays the size's.
func ButtonGeometry(m gfx.Metrics, size Size) (height, padding, fontSize float32, iconOnly bool) {
	height = m.ControlHeight
	padding = 10
	fontSize = m.FontSizeM
	switch size {
	case SizeXSmall:
		height, padding, fontSize = m.ControlHeight-8, 8, m.FontSizeS
	case SizeSmall:
		height = m.ControlHeight - 4
	case SizeLarge:
		height = m.ControlHeight + 4
	case SizeIcon:
		iconOnly = true
	case SizeIconXSmall:
		height, iconOnly = m.ControlHeight-8, true
		fontSize = m.FontSizeS
	case SizeIconSmall:
		height, iconOnly = m.ControlHeight-4, true
	case SizeIconLarge:
		height, iconOnly = m.ControlHeight+4, true
	}
	if height < 20 {
		height = 20
	}
	return height, padding, fontSize, iconOnly
}
