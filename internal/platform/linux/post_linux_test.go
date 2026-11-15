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
