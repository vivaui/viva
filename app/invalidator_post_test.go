package app

import (
	"sync"
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A background goroutine's result reaches the model through Post: the update
// runs on the window thread, before OnWake and the repaint it asked for, and not
// on the goroutine. A window that has closed takes no more updates.
func TestInvalidatorPostRunsUpdatesOnTheWindowThreadUntilItCloses(t *testing.T) {
	invalidator := new(Invalidator)
	if invalidator.Post(func() { t.Error("an update posted before the window opened ran") }) {
		t.Fatal("Post accepted an update before the window opened")
	}
	status, painted, order := "", "", []string{}
	driver, err := NewDriver(Config{
		Invalidator: invalidator,
		OnWake:      func() { order = append(order, "wake:"+status) },
		Paint:       func(*gfx.Canvas, gfx.Size) { painted = status },
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if !invalidator.Post(func() { status += "x" }) {
				t.Error("a live window refused an update")
			}
		})
	}
	wg.Wait()
	if status != "" {
		t.Fatal("an update ran on the posting goroutine")
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if status != "xxxxxxxx" || painted != status || len(order) != 1 || order[0] != "wake:"+status {
		t.Fatalf("status %q painted %q wakes %v: want every update applied before OnWake and the paint", status, painted, order)
	}

	driver.Close()
	if invalidator.Post(func() { t.Error("an update ran after the window closed") }) {
		t.Fatal("Post accepted an update after the window closed")
	}
}

// A window that cannot keep up refuses updates past the limit instead of
// holding an unbounded backlog.
func TestInvalidatorPostIsBounded(t *testing.T) {
	invalidator := new(Invalidator)
	driver, err := NewDriver(Config{Invalidator: invalidator}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	ran := 0
	for range postLimit {
		if !invalidator.Post(func() { ran++ }) {
			t.Fatal("an update under the limit was refused")
		}
	}
	if invalidator.Post(func() { ran++ }) {
		t.Fatal("an update past the limit was accepted")
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if ran != postLimit || !invalidator.Post(func() {}) {
		t.Fatalf("ran %d of %d, and the drained queue must take updates again", ran, postLimit)
	}
}
