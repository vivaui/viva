package app

import (
	"sync"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/window"
)

// Invalidator lets background goroutines hand results to a window and ask it
// to repaint. Create one, pass it as Config.Invalidator, and call Post with the
// update a goroutine's result makes, or Invalidate when the goroutine published
// under its own synchronization. It owns no goroutine, timer, or channel: an
// application that never calls it pays nothing, and the window thread is woken
// exactly once per call batch.
//
// Calls made before RunCanvas starts or after the window closes do nothing.
// The zero value is ready to use.
type Invalidator struct {
	waker platform.Waker

	mu    sync.Mutex
	posts []func()
	// open is set while the window that wakes through this Invalidator is open:
	// Post queues only then.
	open bool
	// running is the window thread's scratch for the updates it takes from the
	// queue, kept so applying them allocates nothing in the steady state.
	running []func()
}

var _ window.Invalidator = (*Invalidator)(nil)

// Invalidate requests one repaint on the window thread. It is safe to call
// from any goroutine and never blocks on the application.
func (invalidator *Invalidator) Invalidate() {
	invalidator.waker.Wake()
}

// postLimit bounds the updates waiting for one window.
const postLimit = 256

// Post queues update to run on the window thread, then repaints: the way a
// background goroutine hands its result to the model the page reads.
//
//	go func() {
//		hosts, err := load()
//		invalidator.Post(func() { m.hosts, m.err = hosts, err })
//	}()
//
// Updates run in the order they were posted, before OnWake and before the
// repaint they asked for, and only while the window is open: Post returns false
// before the window opens, after it closes (updates still waiting are dropped
// then), and when the window already has 256 updates waiting, so a window that
// cannot keep up holds no unbounded backlog. update runs on the window thread
// and may change anything the page reads; it must not block. Post is safe to
// call from any goroutine.
func (invalidator *Invalidator) Post(update func()) bool {
	if update == nil {
		return false
	}
	invalidator.mu.Lock()
	if !invalidator.open || len(invalidator.posts) >= postLimit {
		invalidator.mu.Unlock()
		return false
	}
	invalidator.posts = append(invalidator.posts, update)
	invalidator.mu.Unlock()
	invalidator.Invalidate()
	return true
}

func (invalidator *Invalidator) isOpen() bool {
	invalidator.mu.Lock()
	defer invalidator.mu.Unlock()
	return invalidator.open
}

// runPosted applies the queued updates on the window thread and reports whether
// the window is still open. An update that closes the window ends the run: the
// ones after it are dropped with the window.
func (invalidator *Invalidator) runPosted() bool {
	invalidator.mu.Lock()
	if !invalidator.open {
		invalidator.mu.Unlock()
		return false
	}
	invalidator.running = append(invalidator.running[:0], invalidator.posts...)
	clear(invalidator.posts)
	invalidator.posts = invalidator.posts[:0]
	invalidator.mu.Unlock()
	for _, update := range invalidator.running {
		if !invalidator.isOpen() {
			break
		}
		update()
	}
	clear(invalidator.running)
	return invalidator.isOpen()
}

// attachPosts makes the window config describes deliver its Invalidator's posts:
// the Invalidator accepts them from now until the window closes, and every wake
// applies them before OnWake. A window that has closed runs no OnWake. Each entry
// point that opens a window calls it once.
func (config *Config) attachPosts() {
	invalidator := config.Invalidator
	if invalidator == nil {
		return
	}
	invalidator.mu.Lock()
	invalidator.open = true
	invalidator.mu.Unlock()
	onWake, onClosed := config.OnWake, config.OnClosed
	config.OnWake = func() {
		if invalidator.runPosted() && onWake != nil {
			onWake()
		}
	}
	config.OnClosed = func() {
		invalidator.closePosts()
		if onClosed != nil {
			onClosed()
		}
	}
}

// closePosts refuses posts from now on and drops the ones waiting: the window
// closed, or never opened.
func (invalidator *Invalidator) closePosts() {
	if invalidator == nil {
		return
	}
	invalidator.mu.Lock()
	invalidator.open = false
	clear(invalidator.posts)
	invalidator.posts = nil
	invalidator.mu.Unlock()
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
