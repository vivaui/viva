package components

import "github.com/vivaui/viva/base"

// CaretBlink is the caret's blink phase, defined in package style so the field
// snapshots can name it without the engine depending on a design system.
type CaretBlink = base.CaretBlink

const (
	CaretBlinkHalf = base.CaretBlinkHalf
	CaretBlinkSpan = base.CaretBlinkSpan
)
