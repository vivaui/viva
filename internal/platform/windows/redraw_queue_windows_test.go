package windows

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestRedrawQueueCoalescesConcurrentWakesAndKeepsChangesDuringPaint(t *testing.T) {
	var queue redrawQueue
	var posts atomic.Int32
	post := func() bool { posts.Add(1); return true }
	var jobs sync.WaitGroup
	for range 32 {
		jobs.Go(func() {
			for range 1000 {
				queue.post(post)
			}
		})
	}
	jobs.Wait()
	if posts.Load() != 1 {
		t.Fatalf("queued %d redraws for one burst", posts.Load())
	}
	queue.begin()
	// The event loop has begun OnRedraw; a concurrent update needs another frame.
	queue.post(post)
	queue.post(post)
	if posts.Load() != 2 {
		t.Fatalf("lost or duplicated a wake during paint: %d", posts.Load())
	}
	queue.begin()
	if posts.Load() != 2 {
		t.Fatal("idle queue posted a redraw")
	}
}

func TestRedrawQueueRetriesAfterNativePostFails(t *testing.T) {
	var queue redrawQueue
	queue.post(func() bool { return false })
	posted := false
	queue.post(func() bool { posted = true; return true })
	if !posted {
		t.Fatal("failed native post suppressed the next wake")
	}
}

func TestRedrawQueueRetriesConcurrentWakeAfterNativePostFails(t *testing.T) {
	var queue redrawQueue
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	attempts := 0
	go func() {
		queue.post(func() bool {
			attempts++
			if attempts == 1 {
				close(entered)
				<-release
				return false
			}
			return true
		})
		close(done)
	}()
	<-entered
	queue.post(func() bool {
		t.Fatal("a concurrent wake should join the in-flight native post")
		return true
	})
	close(release)
	<-done
	if attempts != 2 {
		t.Fatalf("native post attempts=%d, want 2", attempts)
	}
}

func TestNativeRedrawMessageKeepsRequestsCoalescedUntilPaint(t *testing.T) {
	// Exercise the message dispatch without creating an OS window or renderer.
	state := &windowState{}
	handle := ^uintptr(0)
	windowStates.Store(handle, state)
	previous := active.Load()
	defer func() { windowStates.Delete(handle); active.Store(previous) }()
	posts := 0
	post := func() bool { posts++; return true }
	state.redraws.post(post)
	state.config.OnRedraw = func() (platform.Frame, error) {
		t.Fatal("posted redraw must yield to hardware input before painting")
		return platform.Frame{}, nil
	}
	windowProc(handle, wmRedraw, 0, 0)
	state.redraws.post(post)
	if posts != 1 || !state.redrawRequested {
		t.Fatalf("posts=%d requested=%v", posts, state.redrawRequested)
	}
}

func BenchmarkRedrawQueue(b *testing.B) {
	var queue redrawQueue
	post := func() bool { return true }
	b.ReportAllocs()
	for b.Loop() {
		queue.post(post)
		queue.post(post)
		queue.begin()
	}
}

func BenchmarkWindowWakeBurst(b *testing.B) {
	for _, coalesce := range []bool{false, true} {
		name := "before-every-request"
		if coalesce {
			name = "after-coalesced"
		}
		b.Run(name, func(b *testing.B) {
			var queue redrawQueue
			var waker platform.Waker
			var posts atomic.Int64
			post := func() bool { posts.Add(1); return true }
			if coalesce {
				waker.Bind(func() { queue.post(post) })
			} else {
				waker.Bind(func() { post() })
			}
			defer waker.Unbind()
			b.ReportAllocs()
			for b.Loop() {
				for range 1000 {
					waker.Wake()
				}
				queue.begin()
			}
			b.ReportMetric(float64(posts.Load())/float64(b.N), "posts/burst")
		})
	}
}
