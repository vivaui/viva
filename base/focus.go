package base

import "github.com/vivaui/viva/gfx"

type FocusOrder func(current int, key gfx.Key, count int) (next int, ok bool)

// FocusAxis is the arrow keys a FocusGroup moves the focus with.
type FocusAxis uint8

const (
	FocusHorizontal FocusAxis = iota + 1 // Left and Right
	FocusVertical                        // Up and Down
	FocusBoth                            // all four
)
