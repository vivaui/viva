package main

import (
	"log"
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
	status      string // read by the page, written only on the window thread
	invalidator *app.Invalidator
}

func (m *model) page(c ui.C) {
	ui.Row(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		if ui.Button(c, "Refresh").Clicked() {
			m.status = "working"
			go func() {
				result := slowWork()
				// Post runs the update on the window thread, then repaints: the
				// goroutine never touches what the page reads.
				m.invalidator.Post(func() { m.status = result })
			}()
		}
		ui.Text(c, m.status)
	})
}

func newConfig(m *model, invalidator *app.Invalidator) app.Config {
	theme := gfx.LightTheme()
	m.invalidator = invalidator
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	return ui.Configure(page, app.Config{Title: "Background", Width: 480, Height: 240, Invalidator: invalidator})
}

func main() {
	var invalidator app.Invalidator
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{status: "idle"}, &invalidator), &canvas); err != nil {
		log.Fatal(err)
	}
}
