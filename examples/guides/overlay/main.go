package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type model struct {
	menuOpen   bool
	dialogOpen bool
	status     string
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(24).Gap(12).Items(ui.Start).Children(func() {
		actions := ui.Button(c, "Actions").Outline()
		if actions.Clicked() {
			m.menuOpen = !m.menuOpen
		}
		ui.Text(c, m.status).Muted()

		// A popover hangs off its anchor while menuOpen is true. A press outside it
		// or Escape closes it, and the press goes on to what is under the pointer;
		// the focus goes back to the anchor.
		ui.PopoverWith(c, actions, ui.PopoverSpec{Width: 180, Align: ui.Start}, &m.menuOpen, func() {
			if ui.Button(c, "New file").Ghost().Clicked() {
				m.menuOpen, m.status = false, "Created"
			}
			if ui.Button(c, "Delete…").Ghost().Destructive().Clicked() {
				m.menuOpen, m.dialogOpen = false, true
			}
		})

		// A modal dims the page and keeps the keyboard while dialogOpen is true:
		// Escape, a press on the dim and the close button set it to false.
		ui.ModalWith(c, &m.dialogOpen, "Delete file?", ui.ModalSpec{Description: "This cannot be undone.", Width: 400}, nil, func() {
			if ui.Button(c, "Cancel").Outline().Clicked() {
				m.dialogOpen = false
			}
			if ui.Button(c, "Delete").Destructive().Clicked() {
				m.dialogOpen, m.status = false, "Deleted"
			}
		})
	})
}

func main() {
	m := &model{}
	if err := ui.Run(app.Config{Title: "Overlays", Width: 640, Height: 360}, m.page); err != nil {
		log.Fatal(err)
	}
}
