package app

import "github.com/vivaui/viva/internal/platform"

// PressKind is what a press at a window point does in a custom-titlebar window:
// the application takes it, a control takes it, the platform moves the window
// at once, or the application takes it and the platform moves the window if the
// pointer drags. The platform asks the same question of the painted page when a
// button goes down (see gfx.Canvas.MoveRegion).
type PressKind = platform.PressKind

const (
	// PressClient is a press the application takes and that moves nothing.
	PressClient = platform.PressClient
	// PressControl is a press on a control.
	PressControl = platform.PressControl
	// PressMove is a press on a move region: the platform moves the window.
	PressMove = platform.PressMove
	// PressMoveOnDrag is a press on a move region under an overlay's backdrop:
	// the application takes it to dismiss the overlay, and the window moves once
	// the pointer drags.
	PressMoveOnDrag = platform.PressMoveOnDrag
)
