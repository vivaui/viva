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

func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
		// The field owns its editor: typing, the caret, selection, the clipboard
		// and input methods need nothing more. The Screen tells the platform which
		// editor belongs to the focused field.
		s.Field(&m.name, "Name", "Your name").Width(320)
		if s.Button("Save").Clicked() {
			m.saved = m.name.Text()
		}
		if m.saved != "" {
			s.Text("Saved " + m.saved).Muted()
		}
	})
}

func main() {
	m := &model{}
	if err := flex.Run(app.Config{Title: "Form", Width: 480, Height: 240}, m.page); err != nil {
		log.Fatal(err)
	}
}
