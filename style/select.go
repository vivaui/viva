package style

import "github.com/vivaui/viva/gfx"

// The select trigger's parts: the arrow's square, the gap before it, and the
// slack the value keeps before that gap. They are Vega's px-2, gap-1.5 and the
// 16px chevron.
const (
	SelectChevronSize = 16
	SelectChevronGap  = 6
	SelectValueSlack  = 6
)

// SelectGeometry is what a theme's metrics give a select trigger at a size: its
// height and the size of its value. Only the extra-small step changes the type.
func SelectGeometry(m gfx.Metrics, size Size) (height, textSize float32) {
	switch size {
	case SizeSmall:
		return m.ControlHeight - 4, m.FontSizeM
	case SizeXSmall:
		return m.ControlHeight - 8, m.FontSizeS
	}
	return m.ControlHeight, m.FontSizeM
}

// SelectContentInsets is the room the trigger leaves inside its edge for the
// value: the text inset on the left, and on the right the inset less its border
// plus the gap, the chevron and the slack.
func SelectContentInsets(m gfx.Metrics) (left, right float32) {
	left = m.TextInsetX()
	right = left - 2 + SelectValueSlack + SelectChevronSize
	return left, right
}

// SelectDefaultWidth is what a trigger with the widest value needs beyond that
// value: its content insets and the slack before the chevron.
func SelectDefaultWidth(m gfx.Metrics) float32 {
	left, right := SelectContentInsets(m)
	return left + right + SelectValueSlack
}
