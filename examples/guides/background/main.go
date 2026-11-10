package main

import (
	"log"
	"sync"
	"time"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// slowWork stands in for anything that takes a while: a request, a disk scan.
var slowWork = func() string {
	time.Sleep(time.Second)
	return "done"
}

type model struct {
	mu          sync.Mutex
	pending     string // written by the worker goroutine, under mu
	status      string // read by the page, written only on the window thread
	invalidator *app.Invalidator
}

func (m *model) page(c ui.C) {
	ui.Row(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		if ui.Button(c, "Refresh").Clicked() {
			m.status = "working"
			go func() {
				result := slowWork()
				m.mu.Lock()
				m.pending = result
				m.mu.Unlock()
				m.invalidator.Invalidate() // wakes the window thread once
			}()
		}
		ui.Text(c, m.status)
	})
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	m.invalidator = invalidator
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	config := page.Config(app.Config{Title: "Background", Width: 480, Height: 240, Invalidator: invalidator})

	// OnWake runs on the window thread right before the repaint Invalidate asked
	// for: move what the goroutine published into state that the page reads.
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
