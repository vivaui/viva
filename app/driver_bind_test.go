package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The first paint runs the application's code, which may wake the driver. The
// binding has to be in place before that paint, or the wake is dropped.
func TestDriverBindsItsInvalidatorBeforeTheFirstPaint(t *testing.T) {
	var invalidator Invalidator
	paints := 0
	config := Config{Title: "first paint wake", Width: 100, Height: 60, Invalidator: &invalidator}
	config.Paint = func(*gfx.Canvas, gfx.Size) {
		paints++
		if paints == 1 {
			invalidator.Invalidate()
		}
	}
	driver, err := NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if paints != 1 {
		t.Fatalf("NewDriver painted %d times, want 1", paints)
	}
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if paints != 2 {
		t.Fatalf("a wake made during the first paint was applied %d times, want once", paints-1)
	}
}
