//go:build windows

package windows

import "testing"

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
