package base

import "testing"

// Numbers one paint releases come back after the quarantine in the order of the
// numbers, whatever order the sweep found them in, so the names a later paint
// hands out do not depend on map iteration.
func TestSlotPoolGivesReleasedNumbersBackInOrder(t *testing.T) {
	var pool slotPool
	for range 6 {
		pool.take(1)
	}
	for _, slot := range []uint32{4, 1, 5, 0} {
		pool.give(slot, 2)
	}
	if got := pool.take(3); got != 6 {
		t.Fatalf("a number still in quarantine was handed out: %d", got)
	}
	for _, want := range []uint32{0, 1, 4, 5, 7} {
		if got := pool.take(2 + slotQuarantine); got != want {
			t.Fatalf("take = %d, want %d", got, want)
		}
	}
}

// Releasing and claiming a whole page of keys at once stays linear: the queue
// pops its front without moving the rest.
func BenchmarkSlotPoolTurnover(b *testing.B) {
	const rows = 10000
	var pool slotPool
	epoch := uint32(0)
	for range rows {
		pool.take(epoch)
	}
	b.ReportAllocs()
	for b.Loop() {
		epoch++
		for slot := range uint32(rows) {
			pool.give(slot, epoch)
		}
		epoch += slotQuarantine
		for range rows {
			pool.take(epoch)
		}
	}
}
