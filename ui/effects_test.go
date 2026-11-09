package ui_test

import (
	"context"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// waitEffect draws until done holds, or fails the test: the work itself runs in
// another goroutine, so the paint that takes its result comes later.
func waitEffect(t *testing.T, draw func(), done func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !done() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the effect")
		}
		draw()
		runtime.Gosched()
	}
}

func TestEffectRunsOncePerKeyAndTakesTheResult(t *testing.T) {
	var runs atomic.Int32
	state := ui.NewEffect[int, string](func(context.Context) (int, error) {
		runs.Add(1)
		return 7, nil
	})
	key := "a"
	var result int
	var took bool
	view := func(c ui.C) {
		ui.Effect(c, state, key)
		if value, err, ok := state.Take(); ok {
			result, took = value, true
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}
	}
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	waitEffect(t, func() { drawPage(page, canvas, view) }, func() bool { return took })
	if result != 7 {
		t.Fatalf("result = %d, want 7", result)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
	if _, _, ok := state.Take(); ok {
		t.Fatal("Take returned the result twice")
	}
	for i := 0; i < 5; i++ {
		drawPage(page, canvas, view)
	}
	if got := runs.Load(); got != 1 {
		t.Fatalf("the same key started %d runs, want 1", got)
	}
}

func TestEffectRestartsWhenTheKeyChanges(t *testing.T) {
	var starts, stopped atomic.Int32
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		starts.Add(1)
		<-ctx.Done()
		stopped.Add(1)
		return 0, ctx.Err()
	})
	key := 1
	view := func(c ui.C) { ui.Effect(c, state, key) }
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	waitEffect(t, draw, func() bool { return starts.Load() == 1 })

	key = 2
	draw()
	waitEffect(t, draw, func() bool { return starts.Load() == 2 && stopped.Load() == 1 })
}

func TestEffectCancelsWhenItDisappearsAndStartsAgainWhenBack(t *testing.T) {
	var starts, stopped atomic.Int32
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		starts.Add(1)
		<-ctx.Done()
		stopped.Add(1)
		return 0, ctx.Err()
	})
	shown := true
	view := func(c ui.C) {
		if shown {
			ui.Effect(c, state, 1)
		}
	}
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	waitEffect(t, draw, func() bool { return starts.Load() == 1 })

	shown = false
	draw()
	waitEffect(t, draw, func() bool { return stopped.Load() == 1 })
	if got := starts.Load(); got != 1 {
		t.Fatalf("a hidden effect started %d runs, want 1", got)
	}

	shown = true
	draw()
	waitEffect(t, draw, func() bool { return starts.Load() == 2 })
}

func TestEffectCancelsWhenThePageCloses(t *testing.T) {
	var starts, stopped atomic.Int32
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		starts.Add(1)
		<-ctx.Done()
		stopped.Add(1)
		return 0, ctx.Err()
	})
	view := func(c ui.C) { ui.Effect(c, state, 1) }
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }

	var baseClosed bool
	config := page.Config(app.Config{OnClosed: func() { baseClosed = true }})
	if config.Invalidator == nil {
		t.Fatal("Page.Config did not give the effects an Invalidator")
	}
	draw()
	waitEffect(t, draw, func() bool { return starts.Load() == 1 })

	config.OnClosed()
	waitEffect(t, draw, func() bool { return stopped.Load() == 1 })
	if !baseClosed {
		t.Fatal("the page's OnClosed did not run the application's")
	}
	draw()
	if got := starts.Load(); got != 1 {
		t.Fatalf("a closed page started %d runs, want 1", got)
	}
}

func TestEffectSteadyPaintAllocatesNothing(t *testing.T) {
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	view := func(c ui.C) {
		ui.Effect(c, state, 1)
		ui.Text(c, "steady")
	}
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	defer page.Config(app.Config{}).OnClosed()
	draw := func() { drawPage(page, canvas, view) }
	for range 3 {
		draw()
	}
	if allocs := testing.AllocsPerRun(30, draw); allocs != 0 {
		t.Fatalf("a steady page with one effect allocates %v times", allocs)
	}
}

func TestPageCloseEndsItsEffects(t *testing.T) {
	var starts, stopped atomic.Int32
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		starts.Add(1)
		<-ctx.Done()
		stopped.Add(1)
		return 0, ctx.Err()
	})
	view := func(c ui.C) { ui.Effect(c, state, 1) }
	page, canvas := ui.NewPage(ui.PageConfig{View: view, Invalidator: new(app.Invalidator)}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	draw()
	waitEffect(t, draw, func() bool { return starts.Load() == 1 })

	page.Close()
	page.Close()
	waitEffect(t, draw, func() bool { return stopped.Load() == 1 })
	draw()
	if got := starts.Load(); got != 1 {
		t.Fatalf("a closed page started %d runs, want 1", got)
	}
}

func BenchmarkEffectSteadyPaint(b *testing.B) {
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	view := func(c ui.C) {
		ui.Effect(c, state, 1)
		ui.Text(c, "steady")
	}
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	draw()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw()
	}
	b.StopTimer()
	page.Close()
}

func BenchmarkEffectRestart(b *testing.B) {
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	key := 0
	view := func(c ui.C) { ui.Effect(c, state, key) }
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	draw()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		key++
		draw()
	}
	b.StopTimer()
	page.Close()
}

func TestEffectInAMemoPanics(t *testing.T) {
	state := ui.NewEffect[int, int](func(context.Context) (int, error) { return 0, nil })
	page, canvas := ui.NewPage(ui.PageConfig{}), benchCanvas()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("Effect in a Memo did not panic")
		}
		if !strings.Contains(recovered.(string), "Memo") {
			t.Fatalf("the panic does not explain the Memo: %v", recovered)
		}
	}()
	drawPage(page, canvas, func(c ui.C) {
		ui.Memo(c, 1, func() { ui.Effect(c, state, 1) })
	})
}

func TestEffectCannotBeDeclaredTwiceWithDifferentKeys(t *testing.T) {
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	view := func(c ui.C) {
		ui.Effect(c, state, 1)
		ui.Effect(c, state, 2)
	}
	page, canvas := ui.NewPage(ui.PageConfig{View: view}), benchCanvas()
	defer func() {
		page.Config(app.Config{}).OnClosed()
		recovered := recover()
		if recovered == nil || !strings.Contains(recovered.(string), "twice") {
			t.Fatalf("the second declaration did not panic: %v", recovered)
		}
	}()
	drawPage(page, canvas, view)
}

func TestEffectStateBelongsToOnePage(t *testing.T) {
	state := ui.NewEffect[int, int](func(ctx context.Context) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	})
	firstView := func(c ui.C) { ui.Effect(c, state, 1) }
	first, canvas := ui.NewPage(ui.PageConfig{View: firstView}), benchCanvas()
	drawPage(first, canvas, firstView)
	second := ui.NewPage(ui.PageConfig{View: firstView})
	defer func() {
		first.Config(app.Config{}).OnClosed()
		second.Config(app.Config{}).OnClosed()
		recovered := recover()
		if recovered == nil || !strings.Contains(recovered.(string), "one page") {
			t.Fatalf("the second page did not panic: %v", recovered)
		}
	}()
	drawPage(second, canvas, firstView)
}

func TestPageConfigInvalidatorCanBeGivenToAnEmbeddedPage(t *testing.T) {
	view := func(ui.C) {}
	ours, base := new(app.Invalidator), new(app.Invalidator)
	page := ui.NewPage(ui.PageConfig{View: view, Invalidator: ours})
	if config := page.Config(app.Config{Invalidator: base}); config.Invalidator != ours {
		t.Fatal("Page.Config did not use the page's Invalidator")
	}
}

func TestEffectNeedsAPageAndItsState(t *testing.T) {
	state := ui.NewEffect[int, int](func(context.Context) (int, error) { return 0, nil })
	var noState *ui.EffectState[int, int]
	page, canvas := ui.NewPage(ui.PageConfig{}), benchCanvas()
	for name, fn := range map[string]func(){
		"no page":  func() { ui.Effect(nil, state, 1) },
		"no state": func() { drawPage(page, canvas, func(c ui.C) { ui.Effect(c, noState, 1) }) },
		"nil run":  func() { ui.NewEffect[int, int](nil) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("did not panic")
				}
			}()
			fn()
		})
	}
}
