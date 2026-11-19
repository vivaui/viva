package ui

import "github.com/vivaui/viva/base"

// Drag is base.Drag; the ui facade keeps the old call sites working.
func Drag(c C, state *DragState) { base.Drag(c, state) }
