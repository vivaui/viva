package app

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestInvalidatorIsInertUntilBoundAndAfterUnbind(t *testing.T) {
	var invalidator Invalidator
	invalidator.Invalidate() // zero value, nothing bound: must not panic or block

	var wakes atomic.Int32
	invalidator.waker.Bind(func() { wakes.Add(1) })
	invalidator.Invalidate()
	invalidator.Invalidate()
	if wakes.Load() != 2 {
		t.Fatalf("wakes = %d, want one per call", wakes.Load())
	}

	invalidator.waker.Unbind()
	invalidator.Invalidate()
	if wakes.Load() != 2 {
		t.Fatalf("a wake reached the adapter after the window closed: %d", wakes.Load())
	}
}

func TestInvalidatorAcceptsConcurrentCallers(t *testing.T) {
	var invalidator Invalidator
	var wakes atomic.Int32
	invalidator.waker.Bind(func() { wakes.Add(1) })

	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			for range 100 {
				invalidator.Invalidate()
			}
		}()
	}
	group.Wait()
	if wakes.Load() != 800 {
		t.Fatalf("wakes = %d, want 800", wakes.Load())
	}
}

func TestWakeHandlerAppliesBackgroundResultsBeforeRepainting(t *testing.T) {
	var order []string
	repaint := func() (platform.Frame, error) {
		order = append(order, "repaint")
		return platform.Frame{}, nil
	}
	handler := wakeHandler(&platform.Waker{}, nil, func() { order = append(order, "onwake") }, repaint)
	if _, err := handler(); err != nil {
		t.Fatal(err)
	}
	if len(order) != 2 || order[0] != "onwake" || order[1] != "repaint" {
		t.Fatalf("order = %v, want OnWake before the repaint", order)
	}

	order = order[:0]
	if _, err := wakeHandler(&platform.Waker{}, nil, nil, repaint)(); err != nil {
		t.Fatal(err)
	}
	if len(order) != 1 || order[0] != "repaint" {
		t.Fatalf("without OnWake, order = %v", order)
	}
}

func TestAWindowNobodyCanSeeRunsOnWakeButDoesNotPaintUntilItShows(t *testing.T) {
	var waker platform.Waker
	var posted int
	waker.Bind(func() { posted++ })
	gate := &visibilityGate{waker: &waker}
	var wakes, paints int
	handler := wakeHandler(&waker, gate, func() { wakes++ }, func() (platform.Frame, error) {
		paints++
		return platform.Frame{}, nil
	})

	gate.changed(false)
	for range 3 {
		if _, err := handler(); err != nil {
			t.Fatal(err)
		}
	}
	if wakes != 3 || paints != 0 {
		t.Fatalf("hidden: %d wakes and %d paints, want every wake run and none painted", wakes, paints)
	}

	gate.changed(true)
	if posted != 1 {
		t.Fatalf("showing a window that missed frames posted %d wakes, want 1", posted)
	}
	if _, err := handler(); err != nil {
		t.Fatal(err)
	}
	if paints != 1 {
		t.Fatalf("the missed frame was painted %d times, want once", paints)
	}

	gate.changed(false)
	gate.changed(true)
	if posted != 1 {
		t.Fatalf("showing a window that missed nothing posted a wake (%d)", posted)
	}
}
