package components

import "github.com/vivaui/viva/style"

// CaretBlink is the caret's blink phase, defined in package style so the field
// snapshots can name it without the engine depending on a design system.
type CaretBlink = style.CaretBlink

const (
	CaretBlinkHalf = style.CaretBlinkHalf
	CaretBlinkSpan = style.CaretBlinkSpan
)
