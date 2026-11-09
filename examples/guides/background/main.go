package main

import (
	"log"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers: the application declares the
// tree once and every control it paints is a name inside a feature of it.
var (
	tree = gfx.Tree{}
	page = tree.Feature("background")

	refresh = tree.ID(page, "refresh")
)

// slowWork stands in for anything that takes a while: a request, a disk scan.
var slowWork = func() string {
	time.Sleep(time.Second)
	return "done"
}

type model struct {
	mu      sync.Mutex
	pending string // written by the worker goroutine, under mu
	status  string // read by Paint, written only on the window thread
	view    ui.View
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Background", Width: 480, Height: 240, Invalidator: invalidator}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Row().Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
				if f.Button(refresh, "Refresh").Clicked() {
					m.status = "working"
					go func() {
						result := slowWork()
						m.mu.Lock()
						m.pending = result
						m.mu.Unlock()
						invalidator.Invalidate() // wakes the window thread once
					}()
				}
				f.Text(m.status)
			})
		})
	}

	// OnWake runs on the window thread right before the repaint Invalidate asked
	// for: move what the goroutine published into state that Paint reads.
	config.OnWake = func() {
		m.mu.Lock()
		m.status, m.pending = m.pending, ""
		m.mu.Unlock()
	}
	return config
}

func main() {
	var invalidator app.Invalidator
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{status: "idle"}, &invalidator), &canvas); err != nil {
		log.Fatal(err)
	}
}
