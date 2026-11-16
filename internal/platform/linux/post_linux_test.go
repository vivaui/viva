//go:build linux

package linux

import "testing"

func TestPostDrainsInOrder(t *testing.T) {
	application := &linuxApplication{}
	var seen []int
	application.Post(func() { seen = append(seen, 1) })
	application.Post(func() { seen = append(seen, 2) })
	application.drainPosts()
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}

func TestAWindowsDrainPicksUpWorkPostedWhileItRuns(t *testing.T) {
	application := &linuxApplication{}
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

func TestAWakeWithNoWindowDoesNotSwallowTheNextPost(t *testing.T) {
	application := &linuxApplication{}
	application.Post(func() {})
	if application.postWake.Load() {
		t.Fatal("a wake stayed pending although no window could carry it")
	}
	application.liveMu.Lock()
	application.live = append(application.live, &window{})
	application.liveMu.Unlock()
	seen := 0
	application.Post(func() { seen++ })
	if !application.postWake.Load() {
		t.Fatal("the post after a window opened did not ask for a drain")
	}
	application.drainPosts()
	if seen != 1 {
		t.Fatalf("seen = %d, want 1", seen)
	}
}

func TestAWindowCarriesWorkPostedBeforeIt(t *testing.T) {
	application := &linuxApplication{}
	seen := 0
	application.Post(func() { seen++ })
	application.liveMu.Lock()
	application.live = append(application.live, &window{})
	application.liveMu.Unlock()
	application.wakePosts()
	if !application.postWake.Load() {
		t.Fatal("opening a window did not wake work posted before it")
	}
	application.drainPosts()
	if seen != 1 {
		t.Fatalf("seen = %d, want 1", seen)
	}
}

func TestAClosedWindowHandsItsWakeToTheRemainingWindow(t *testing.T) {
	application := &linuxApplication{}
	first, second := &window{}, &window{}
	application.liveMu.Lock()
	application.live = append(application.live, first)
	application.liveMu.Unlock()
	seen := 0
	application.Post(func() { seen++ })
	application.liveMu.Lock()
	application.live = append(application.live, second)
	application.liveMu.Unlock()
	application.wakePosts()
	application.removeLive(first)
	if !application.postWake.Load() {
		t.Fatal("the wake went with the closed window; the remaining window was not woken")
	}
	application.removeLive(second)
	if application.postWake.Load() {
		t.Fatal("the wake stayed pending after its last carrier closed")
	}
	application.liveMu.Lock()
	application.live = append(application.live, &window{})
	application.liveMu.Unlock()
	application.wakePosts()
	application.drainPosts()
	if seen != 1 {
		t.Fatalf("seen = %d, want 1", seen)
	}
}

func TestAClosedLastWindowLeavesNoStaleWake(t *testing.T) {
	application := &linuxApplication{}
	last := &window{}
	application.liveMu.Lock()
	application.live = append(application.live, last)
	application.liveMu.Unlock()
	seen := 0
	application.Post(func() { seen++ })
	application.removeLive(last)
	if application.postWake.Load() {
		t.Fatal("a wake stayed pending after the last window closed")
	}
	application.liveMu.Lock()
	application.live = append(application.live, &window{})
	application.liveMu.Unlock()
	application.wakePosts()
	if !application.postWake.Load() {
		t.Fatal("opening a window after the last one closed did not wake the queued work")
	}
	application.drainPosts()
	if seen != 1 {
		t.Fatalf("seen = %d, want 1", seen)
	}
}
