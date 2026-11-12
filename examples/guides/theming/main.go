package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type model struct {
	dark  bool
	theme gfx.Theme
}

// applyTheme recomputes the active theme: a value, so changing appearance is
// changing the value the page paints with.
func (m *model) applyTheme() {
	theme := gfx.LightTheme()
	if m.dark {
		theme = gfx.DarkTheme()
	}
	// Tokens are plain fields. Override one to brand the application.
	theme.Primary = gfx.Color{Red: 0.9, Green: 0.45, Blue: 0.1, Alpha: 1}
	theme.PrimaryForeground = gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	m.theme = theme
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		if ui.Button(c, "Toggle theme").Clicked() {
			m.dark = !m.dark
			m.applyTheme()
		}
		ui.Text(c, "Hello")
	})
}

func newConfig(m *model) app.Config {
	m.applyTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &m.theme})
	return page.Config(app.Config{Title: "Theme", Width: 480, Height: 240})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
