//go:build darwin || windows || linux

package app

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// goroutinesSettledAt returns the goroutine count once it has stopped moving, so
// a goroutine that is only just finishing does not count against a test.
func goroutinesSettledAt() int {
	count := runtime.NumGoroutine()
	for range 50 {
		runtime.Gosched()
		time.Sleep(2 * time.Millisecond)
		next := runtime.NumGoroutine()
		if next == count {
			return count
		}
		count = next
	}
	return count
}

// The runtime's idle contract: no hidden workers, timers or polling
// by default. The driver runs the same painter as a window, so a driver that
// takes input, animates, settles and closes must add no goroutine at any point,
// ask for no further frame once settled, and paint nothing more however long it
// is left alone.
func TestDriverAddsNoGoroutinesAndIdlesWhenSettled(t *testing.T) {
	var invalidator Invalidator
	paints, wakes := 0, 0
	checked := false
	config := Config{Title: "idle", Width: 320, Height: 120, Invalidator: &invalidator}
	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		paints++
		theme := gfx.LightTheme()
		components.BeginButton(canvas, theme, gfx.Item(1), gfx.Rect{X: 10, Y: 10, Width: 90, Height: 32}).Default("Save")
		components.BeginSwitch(canvas, theme, gfx.Item(2), gfx.Rect{X: 10, Y: 60, Width: 44, Height: 24}, checked).Paint()
	}
	onClick(&config, func(id gfx.ID) {
		if id == gfx.Item(2) {
			checked = !checked
		}
	})
	config.OnWake = func() { wakes++ }

	before := goroutinesSettledAt()
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	for _, step := range []func() error{
		func() error { return driver.Move(20, 20) },
		func() error { return driver.Click(20, 20) },
		func() error {
			if err := driver.Click(20, 70); err != nil {
				return err
			}
			// The switch's transition is what there is to settle: without it
			// the test would pass for a window that never animated.
			if !driver.animating {
				t.Fatal("toggling the switch started no transition, so this test would prove nothing")
			}
			return nil
		},
		func() error { return driver.Resize(400, 200) },
		func() error { return driver.Leave() },
	} {
		if err := step(); err != nil {
			t.Fatal(err)
		}
		if now := runtime.NumGoroutine(); now > before {
			t.Fatalf("input started %d goroutines", now-before)
		}
	}

	// A background goroutine asks for a repaint, as the application's own
	// workers do; the request is one wake, applied by Pump.
	var worker sync.WaitGroup
	worker.Add(1)
	go func() {
		defer worker.Done()
		invalidator.Invalidate()
	}()
	worker.Wait()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if wakes != 1 {
		t.Fatalf("one Invalidate ran OnWake %d times", wakes)
	}

	// Let every transition finish.
	for range 200 {
		if !driver.animating {
			break
		}
		if err := driver.Advance(20 * time.Millisecond); err != nil {
			t.Fatal(err)
		}
	}
	if driver.animating {
		t.Fatal("transitions never settled")
	}

	settled := paints
	for range 5 {
		if err := driver.Advance(time.Minute); err != nil {
			t.Fatal(err)
		}
		if err := driver.Pump(); err != nil {
			t.Fatal(err)
		}
	}
	if paints != settled || driver.animating {
		t.Fatalf("a settled window painted %d more times (animating=%v)", paints-settled, driver.animating)
	}

	driver.Close()
	if after := goroutinesSettledAt(); after > before {
		t.Fatalf("the driver left %d goroutines behind", after-before)
	}
}
