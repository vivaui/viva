package style

import "github.com/vivaui/viva/gfx"

// BadgeGeometry is what a theme's metrics give a badge: the height of its chip,
// the padding either side of its content, the radius that makes it a pill (half
// the height) and the size of its label. Vega scales the chip at 20/32 of the
// control height and pads it by 9/32 either side.
func BadgeGeometry(m gfx.Metrics) (height, padding, radius, fontSize float32) {
	scale := m.ControlHeight / 32
	return 20 * scale, 9 * scale, 10 * scale, m.FontSizeS
}
