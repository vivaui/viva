package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type model struct {
	menuOpen   bool
	dialogOpen bool
	status     string
}

func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(24).Gap(12).Items(flex.Start).Children(func() {
		actions := s.Button("Actions").Outline()
		if actions.Clicked() {
			m.menuOpen = !m.menuOpen
		}
		s.Text(m.status).Muted()

		// A popover hangs off its anchor while menuOpen is true. A press outside it
		// or Escape closes it, and the press goes on to what is under the pointer;
		// the focus goes back to the anchor.
		s.PopoverWith(actions, flex.PopoverSpec{Width: 180, Align: flex.Start}, &m.menuOpen, func(p flex.Screen) {
			if p.Button("New file").Ghost().Clicked() {
				m.menuOpen, m.status = false, "Created"
			}
			if p.Button("Delete…").Ghost().Destructive().Clicked() {
				m.menuOpen, m.dialogOpen = false, true
			}
		})

		// A modal dims the page and keeps the keyboard while dialogOpen is true:
		// Escape, a press on the dim and the close button set it to false.
		s.ModalWith(flex.ModalSpec{Title: "Delete file?", Description: "This cannot be undone.", Width: 400}, &m.dialogOpen,
			func(d flex.ModalScreen) {
				d.Footer(func() {
					if d.Button("Cancel").Outline().Clicked() {
						m.dialogOpen = false
					}
					if d.Button("Delete").Destructive().Clicked() {
						m.dialogOpen, m.status = false, "Deleted"
					}
				})
			})
	})
}

func main() {
	m := &model{}
	if err := flex.Run(app.Config{Title: "Overlays", Width: 640, Height: 360}, m.page); err != nil {
		log.Fatal(err)
	}
}
