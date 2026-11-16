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
