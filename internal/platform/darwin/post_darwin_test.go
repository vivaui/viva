//go:build darwin

package darwin

import "testing"

func TestPostedFunctionsDrainInOrder(t *testing.T) {
	postQueue.Lock()
	postQueue.pending = nil
	postQueue.Unlock()
	var seen []int
	Post(func() { seen = append(seen, 1) })
	Post(func() { seen = append(seen, 2) })
	vivaApplicationDrain()
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}

func TestAFunctionPostedWhileDrainingRunsInTheSameDrain(t *testing.T) {
	postQueue.Lock()
	postQueue.pending = nil
	postQueue.Unlock()
	var seen []int
	Post(func() {
		seen = append(seen, 1)
		Post(func() { seen = append(seen, 2) })
	})
	vivaApplicationDrain()
	if len(seen) != 2 || seen[0] != 1 || seen[1] != 2 {
		t.Fatalf("seen = %v, want [1 2]", seen)
	}
}
