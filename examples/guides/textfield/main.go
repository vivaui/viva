package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type model struct {
	name  flex.Field
	saved string
}

func (m *model) page(c flex.C) {
	flex.Column(c).Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
		// The field owns its editor: typing, the caret, selection, the clipboard
		// and input methods need nothing more. The page tells the platform which
		// editor belongs to the focused field.
		flex.TextField(c, &m.name, "Name", "Your name").Width(320)
		if flex.Button(c, "Save").Clicked() {
			m.saved = m.name.Text()
		}
		if m.saved != "" {
			flex.Text(c, "Saved "+m.saved).Muted()
		}
	})
}

func main() {
	m := &model{}
	if err := flex.Run(app.Config{Title: "Form", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
