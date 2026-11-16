//go:build darwin

package darwin

import (
	"testing"
	"time"
)

func postedDrainResult(t *testing.T, run func() []int) []int {
	t.Helper()
	result := make(chan []int, 1)
	// TestMain serves the native queue. Keep the manual drain on that same
	// thread, so a scheduled native drain cannot consume these callbacks first.
	Post(func() { result <- run() })
	select {
	case seen := <-result:
		return seen
	case <-time.After(5 * time.Second):
		t.Fatal("posted main-thread drain did not complete")
		return nil
	}
}

func TestPostedFunctionsDrainInOrder(t *testing.T) {
	seen := postedDrainResult(t, func() []int {
		var seen []int
		Post(func() { seen = append(seen, 1) })
		Post(func() { seen = append(seen, 2) })
		vivaApplicationDrain()
		return seen
	})
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}

func TestAFunctionPostedWhileDrainingRunsInTheSameDrain(t *testing.T) {
	seen := postedDrainResult(t, func() []int {
		var seen []int
		Post(func() {
			seen = append(seen, 1)
			Post(func() { seen = append(seen, 2) })
		})
		vivaApplicationDrain()
		return seen
	})
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}
