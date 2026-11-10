package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type model struct {
	saved bool
	notes bool
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Children(func() {
		ui.Text(c, "Deploy settings").FontSize(18).Bold()
		ui.Text(c, "Where the build goes when it passes.").Muted()
		ui.Divider(c)
		ui.Row(c).Items(ui.Center).Gap(8).Children(func() {
			ui.Text(c, "Notify the team")
			ui.Spacer(c)
			ui.Switch(c, &m.notes, "Notify the team")
		})
		ui.Spacer(c)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			ui.Button(c, "Cancel").Outline()
			if ui.Button(c, "Save").Clicked() {
				m.saved = true
			}
		})
	})
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	return page.Config(app.Config{Title: "Layout", Width: 420, Height: 220})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(&model{}), &canvas); err != nil {
		log.Fatal(err)
	}
}
