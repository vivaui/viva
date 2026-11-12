//go:build darwin

package darwin

import "testing"

// A page appending one instance per frame must not multiply the buffer it
// holds: the capacity grows only when the count no longer fits, and doubles,
// so it stays within twice the scene however many appends arrive. Growing by a
// fraction of the capacity on every append compounded it instead, and the
// request passed the largest buffer the device makes after a few dozen
// appends; the window then closed with "failed to rebuild rectangle instance
// buffer".
func TestRectangleBufferCapacityFollowsTheSceneNotTheAppends(t *testing.T) {
	capacity, count, grown := uintptr(1000), uintptr(1000), 0
	for range 1000 {
		count++
		if next := rectangleCapacity(capacity, count); next != capacity {
			capacity = next
			grown++
		}
	}
	if capacity < count {
		t.Fatalf("capacity %d cannot hold %d instances", capacity, count)
	}
	if capacity > 2*count {
		t.Fatalf("capacity %d is more than twice the %d instances", capacity, count)
	}
	// Doubling from 1000 covers 2000 instances in one growth.
	if grown > 4 {
		t.Fatalf("1000 single-instance appends reallocated %d times", grown)
	}
}

// Headroom that already holds the count is kept, not remade.
func TestRectangleBufferCapacityKeepsItsHeadroom(t *testing.T) {
	if got := rectangleCapacity(4096, 1000); got != 4096 {
		t.Fatalf("capacity of a count that fits = %d, want 4096", got)
	}
	if got := rectangleCapacity(0, 10); got < 10 {
		t.Fatalf("capacity for 10 instances from nothing = %d", got)
	}
	if got := rectangleCapacity(4, 100); got < 100 {
		t.Fatalf("capacity for 100 instances from 4 = %d", got)
	}
}
