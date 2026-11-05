package app

import "github.com/vivaui/viva/internal/platform"

// Invalidator lets background goroutines ask the window to repaint. Create one,
// pass it as Config.Invalidator, and call Invalidate from anywhere after the
// goroutine has published its result. It owns no goroutine, timer, or channel:
// an application that never calls Invalidate pays nothing, and the window
// thread is woken exactly once per call batch.
//
// Calls made before RunCanvas starts or after the window closes do nothing.
// The zero value is ready to use.
type Invalidator struct {
	waker platform.Waker
}

// Invalidate requests one repaint on the window thread. It is safe to call
// from any goroutine and never blocks on the application.
func (invalidator *Invalidator) Invalidate() {
	invalidator.waker.Wake()
}

// wakeHandler runs the application's OnWake hook, then repaints, so results
// that background work published are applied before the next paint reads them.
//
// The waker is told the redraw has begun before OnWake reads anything, which is what puts
// the next wake on the frame grid. A window the gate says nobody can see runs OnWake and
// skips the repaint.
func wakeHandler(waker *platform.Waker, gate *visibilityGate, onWake func(), repaint platform.RedrawHandler) platform.RedrawHandler {
	return func() (platform.Frame, error) {
		waker.Redrawing()
		if onWake != nil {
			onWake()
		}
		if gate != nil && gate.skipPaint() {
			return platform.Frame{}, nil
		}
		return repaint()
	}
}
