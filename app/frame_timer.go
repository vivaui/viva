package app

import (
	"sync/atomic"
	"time"
)

// frameInterval is the interval the frame timer starts with, for a workspace
// whose adapter never reports how it presents. An adapter that does calls
// platform.Config.SetFrameInterval: zero means presenting a frame blocks until
// the display takes it, and the timer then asks for the next frame as soon as
// the last one is presented; a positive value is the display's own interval.
const frameInterval = time.Second / 120

// minFrameInterval is the shortest interval the timer asks for when the
// presentation paces frames. It is a safety net for a frame that presents
// nothing (so nothing throttles the loop) and stays below every display's
// refresh period.
const minFrameInterval = time.Second / 240

// frameTimer coalesces next-frame requests into at most one pending timer.
// The painter calls request from the window thread after a paint that left
// transitions in flight; a settled canvas never calls it, so an idle
// application owns no timer, goroutine, or wake-up. When the timer fires it
// posts one redraw event through wake and the cycle repeats from the repaint.
type frameTimer struct {
	wake func()
	// interval is the display period, zero when presentation blocks. It is
	// written by the adapter on the window thread and read by request there.
	interval time.Duration
	// last is when the frame now being painted began. Requests are measured
	// from it, not from the end of the paint, so a slow frame does not push
	// every later frame behind.
	last time.Time
	// now is time.Now, and a test's own clock.
	now     func() time.Time
	timer   *time.Timer
	due     time.Time // when the armed timer fires; written by the window thread only
	armed   atomic.Bool
	stopped atomic.Bool
}

func newFrameTimer(wake func()) *frameTimer {
	return &frameTimer{wake: wake, interval: frameInterval, now: time.Now}
}

// redrawing records the start of the frame the window is painting now. It must
// run on the window thread, before the paint calls request.
func (frames *frameTimer) redrawing() {
	frames.last = frames.now()
}

// setInterval tells the timer how the window is paced: zero when a presented
// frame blocks until the display takes it, else the display interval. It must
// run on the window thread.
func (frames *frameTimer) setInterval(interval time.Duration) {
	frames.interval = max(interval, 0)
}

// request arms the timer for a wake-up after delay, never sooner than one
// display frame, unless a wake-up is already pending. A pending wake-up that
// is later than the one asked for is brought forward, so an animation that
// starts while a slow tick is waiting is not held back by it. It must be
// called from the window thread, which is the timer's only writer.
func (frames *frameTimer) request(delay time.Duration) {
	if frames.stopped.Load() {
		return
	}
	now := frames.now()
	base := frames.last
	if base.IsZero() {
		base = now
	}
	step := max(delay, frames.interval)
	if frames.interval <= 0 {
		step = max(step, minFrameInterval)
	}
	due := base.Add(step)
	wait := due.Sub(now)
	if wait < 0 {
		wait = 0
	}
	if !frames.armed.CompareAndSwap(false, true) {
		// Stop reports whether the timer was still waiting; if it already
		// fired the wake-up is on its way and the repaint it causes asks again.
		if due.Before(frames.due) && frames.timer.Stop() {
			frames.due = due
			frames.timer.Reset(wait)
		}
		return
	}
	frames.due = due
	if frames.timer == nil {
		frames.timer = time.AfterFunc(wait, frames.fire)
		return
	}
	frames.timer.Reset(wait)
}

func (frames *frameTimer) fire() {
	frames.armed.Store(false)
	if !frames.stopped.Load() {
		frames.wake()
	}
}

// stop cancels any pending wake-up. Later requests are ignored.
func (frames *frameTimer) stop() {
	frames.stopped.Store(true)
	if frames.timer != nil {
		frames.timer.Stop()
	}
}

// monotonicClock returns frame times measured from first use. time.Since
// reads the monotonic clock, so wall-clock adjustments never bend a
// transition.
func monotonicClock() func() time.Duration {
	epoch := time.Now()
	return func() time.Duration { return time.Since(epoch) }
}
