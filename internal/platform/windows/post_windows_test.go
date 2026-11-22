//go:build windows

package windows

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
	"unsafe"
)

var peekWakeMessage = user.NewProc("PeekMessageW")

func TestDrainPostsRunsWorkInOrder(t *testing.T) {
	application := &windowsApplication{}
	var seen []int
	application.Post(func() { seen = append(seen, 1) })
	application.Post(func() { seen = append(seen, 2) })
	application.drainPosts()
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}

func TestADrainPicksUpWorkPostedWhileItRuns(t *testing.T) {
	application := &windowsApplication{}
	var seen []int
	application.Post(func() {
		seen = append(seen, 1)
		application.Post(func() { seen = append(seen, 2) })
	})
	application.drainPosts()
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}

// modalPump retrieves messages the way a modal Win32 loop (a native menu, a
// dialog) does: a window message reaches its window procedure, a thread
// message is dispatched to nothing and disappears.
func modalPump(d time.Duration, done func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if done() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		var event message
		if received, _, _ := peekWakeMessage.Call(uintptr(unsafe.Pointer(&event)), 0, 0, 0, 1); received != 0 {
			if event.Window != 0 {
				dispatchMessage.Call(uintptr(unsafe.Pointer(&event)))
			}
			continue
		}
		time.Sleep(time.Millisecond)
	}
}

// Posted work must run while the loop is inside a modal message loop, and a
// later Post must still find a wake after one could not be delivered.
func TestPostedWorkRunsUnderAModalLoopAndRecoversALostWake(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	application := &windowsApplication{}

	// No wake window yet: the first wake cannot reach a loop (thread 0). The
	// work stays queued and the next Post must post a fresh wake rather than
	// leave it behind a latch.
	var first, second atomic.Bool
	application.Post(func() { first.Store(true) })
	if err := application.openWakeWindow(); err != nil {
		t.Fatalf("open wake window: %v", err)
	}
	defer application.closeWakeWindow()
	application.Post(func() { second.Store(true) })

	if !modalPump(2*time.Second, func() bool { return first.Load() && second.Load() }) {
		t.Fatalf("posted work did not run under a modal loop: first=%v second=%v", first.Load(), second.Load())
	}
}
