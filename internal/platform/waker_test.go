package platform

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestWakerIsNoOpUntilBoundAndAfterUnbind(t *testing.T) {
	var waker Waker
	waker.Wake()

	var wakes atomic.Int32
	waker.Bind(func() { wakes.Add(1) })
	waker.Wake()
	waker.Wake()
	if wakes.Load() != 2 {
		t.Fatalf("wakes = %d, want 2: with no spacing every wake is posted", wakes.Load())
	}

	waker.Unbind()
	waker.Wake()
	if wakes.Load() != 2 {
		t.Fatalf("wake after Unbind reached the adapter: %d", wakes.Load())
	}
}

func TestWakerUnbindWaitsForInFlightWake(t *testing.T) {
	var waker Waker
	entered, release := make(chan struct{}), make(chan struct{})
	waker.Bind(func() {
		close(entered)
		<-release
	})
	var group sync.WaitGroup
	group.Add(1)
	go func() {
		defer group.Done()
		waker.Wake()
	}()
	<-entered

	var unbindReturned atomic.Bool
	unbound := make(chan struct{})
	go func() {
		waker.Unbind()
		unbindReturned.Store(true)
		close(unbound)
	}()
	runtime.Gosched()
	if unbindReturned.Load() {
		t.Fatal("Unbind returned while the adapter was still posting")
	}
	close(release)
	<-unbound
	group.Wait()
}

// The grid is time, so these tests assert only what holds on any machine: nothing is posted
// before the spacing has passed, and what is held is posted afterwards.

func TestWakesWithinAFrameOfAPaintBecomeOneWakeAtTheNextFrame(t *testing.T) {
	const spacing = 60 * time.Millisecond
	var waker Waker
	posted := make(chan time.Time, 8)
	waker.Bind(func() { posted <- time.Now() })
	waker.SetSpacing(spacing)

	waker.Wake() // no paint yet: at once
	select {
	case <-posted:
	default:
		t.Fatal("a wake before any paint was held")
	}

	waker.Redrawing()
	began := time.Now()
	for range 100 {
		waker.Wake()
	}
	select {
	case at := <-posted:
		t.Fatalf("a wake was posted %v after the paint began, inside the spacing", at.Sub(began))
	default:
	}
	select {
	case at := <-posted:
		if at.Sub(began) < spacing {
			t.Fatalf("posted %v after the paint began, before the spacing %v", at.Sub(began), spacing)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the held wake never came")
	}
	select {
	case <-posted:
		t.Fatal("100 wakes in one frame became more than one")
	case <-time.After(2 * spacing):
	}
}

func TestAWakeAfterAQuietSpellIsPostedAtOnce(t *testing.T) {
	const spacing = 20 * time.Millisecond
	var waker Waker
	var wakes atomic.Int32
	waker.Bind(func() { wakes.Add(1) })
	waker.SetSpacing(spacing)
	waker.Redrawing()
	time.Sleep(2 * spacing)
	waker.Wake()
	if wakes.Load() != 1 {
		t.Fatalf("wakes = %d, want the wake posted at once", wakes.Load())
	}
}

func TestARedrawTakesTheHeldWakeAndNoTimerOutlivesUnbind(t *testing.T) {
	const spacing = 40 * time.Millisecond
	var waker Waker
	var wakes atomic.Int32
	waker.Bind(func() { wakes.Add(1) })
	waker.SetSpacing(spacing)
	waker.Redrawing()
	waker.Wake()      // held
	waker.Redrawing() // another paint reads what it was for
	time.Sleep(3 * spacing)
	if wakes.Load() != 0 {
		t.Fatalf("a wake the next paint made unnecessary was posted: %d", wakes.Load())
	}
	waker.Redrawing()
	waker.Wake() // held again
	waker.Unbind()
	time.Sleep(3 * spacing)
	if wakes.Load() != 0 {
		t.Fatalf("a held wake was posted after Unbind: %d", wakes.Load())
	}
}

// A wake made while another is held or posted is a lock, a clock read and a comparison.
func BenchmarkWakeJoinsAHeldWake(b *testing.B) {
	var waker Waker
	waker.Bind(func() {})
	waker.SetSpacing(time.Hour)
	waker.Redrawing()
	waker.Wake() // held for an hour
	b.ReportAllocs()
	for b.Loop() {
		waker.Wake()
	}
	waker.Unbind()
}
