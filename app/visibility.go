package app

import "github.com/vivaui/viva/internal/platform"

// visibilityGate keeps a window nobody can see from painting. The platform says when the window
// is hidden, minimized, covered or on a desktop that is not showing; while it is, a wake still
// runs OnWake, which the application may use to refresh its status icon, but the paint and the
// present are skipped and remembered, and the first wake after the window shows again paints the
// frame that was missed. It is touched only on the window thread.
type visibilityGate struct {
	hidden bool
	// missed is set when a wake's paint was skipped.
	missed bool
	waker  *platform.Waker
}

// changed is OnVisibility.
func (gate *visibilityGate) changed(visible bool) {
	gate.hidden = !visible
	if visible && gate.missed {
		gate.waker.Wake()
	}
}

// skipPaint reports whether this wake's paint should be skipped, remembering that it was.
func (gate *visibilityGate) skipPaint() bool {
	if gate.hidden {
		gate.missed = true
		return true
	}
	gate.missed = false
	return false
}

// handler is the gate as the platform's OnVisibility, or nil for a window without wakes, which
// has nothing to hold back.
func (gate *visibilityGate) handler() platform.VisibilityHandler {
	if gate == nil {
		return nil
	}
	return gate.changed
}
