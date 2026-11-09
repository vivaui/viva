package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	name  ui.Field
	saved string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		// The field owns its editor: typing, the caret, selection, the clipboard
		// and input methods need nothing more. The page tells the platform which
		// editor belongs to the focused field.
		ui.TextField(c, &m.name, "Name", "Your name").Width(320)
		if ui.Button(c, "Save").Clicked() {
			m.saved = m.name.Text()
		}
		if m.saved != "" {
			ui.Text(c, "Saved "+m.saved).Muted()
		}
	})
}

func main() {
	m := &model{}
	if err := ui.Run(app.Config{Title: "Form", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
