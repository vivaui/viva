package base

// Align places children across a container (Align) or along it (Justify).
// Auto, the zero value, leaves the choice to the default: children stretch across
// a container and start along it.
type Align uint8

const (
	Auto Align = iota
	Start
	Center
	End
	// Baseline lines the children up on their first line of text: in a Row, the
	// labels of a button, a text and a field sit on one line whatever their sizes.
	Baseline
	// Stretch makes a child as wide as the container, for the cross axis, when
	// it has no width of its own. It is the default across.
	Stretch
	// Between puts the free space between the children. It is for Justify.
	Between
)
