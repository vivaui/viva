// Package window holds the contracts a platform host and the layers above it
// share: the invalidator a background goroutine uses to ask for a repaint, and
// the system appearance. It depends neither on the platform loop nor on pages,
// so both can depend on it.
package window

// Invalidator asks the window to repaint: app.Invalidator implements it, and a
// page or a worker holds the interface instead of the platform loop's type.
type Invalidator interface {
	// Invalidate requests one repaint on the window thread. It is safe to call
	// from any goroutine and never blocks on the application.
	Invalidate()
}
