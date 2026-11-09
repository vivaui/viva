package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	name  string
	saved string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		// m.name is the text: the field writes what is typed into it, and shows
		// what the program puts there. Typing, the caret, selection, the clipboard
		// and input methods need nothing more; the page keeps the field's editor.
		ui.TextField(c, &m.name, "Name", "Your name").Width(320)
		if ui.Button(c, "Save").Clicked() {
			m.saved = m.name
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
