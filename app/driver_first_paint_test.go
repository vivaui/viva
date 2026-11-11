package app_test

import (
	"context"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// The first paint can start work that lands before NewDriver returns: its wake
// must reach the driver instead of an invalidator nobody is bound to yet.
func TestDriverAppliesAnEffectThatFinishesDuringTheFirstPaint(t *testing.T) {
	var invalidator app.Invalidator
	started := make(chan struct{})
	var got int
	first := true
	effect := ui.NewEffect[int, int](func(context.Context) (int, error) {
		close(started)
		return 7, nil
	})
	page := ui.NewPage(ui.PageConfig{View: func(c ui.C) {
		ui.Effect(c, effect, 1)
		if first {
			first = false
			// The first paint runs inside NewDriver. The run returns as soon as it
			// has started, so this wait puts its wake inside that paint, where the
			// binding used to be missing. The result is taken by the paint the
			// wake asks for.
			<-started
			time.Sleep(50 * time.Millisecond)
			return
		}
		if value, _, ok := effect.Take(); ok {
			got = value
		}
	}})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 480, Invalidator: &invalidator}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if got != 7 {
		t.Fatalf("result after Pump = %d, want 7; the effect's wake was lost", got)
	}
}
