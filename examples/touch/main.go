// Touch is the page a phone is checked with: what a finger does to each kind of control.
// A long list scrolls under a finger and keeps going when it lifts; a tap presses; a
// finger held on a row opens its menu; a slider takes a moving finger as a drag; and a
// field brings the keyboard. The same page runs with a mouse on a desktop.
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

const rows = 200

type model struct {
	taps   int
	volume ui.SliderState
	notify bool
	name   string
	list   ui.ListState
	menu   string
}

func newModel() *model {
	m := &model{volume: ui.SliderState{Value: 40}}
	m.list.Label = func(i int) string { return fmt.Sprintf("Row %d", i+1) }
	return m
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Gap(12).Items(ui.Stretch).Children(func() {
		ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
			if ui.Button(c, "Tap me").Clicked() {
				m.taps++
			}
			ui.Text(c, fmt.Sprintf("Tapped %d times", m.taps))
		})
		ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("Volume %.0f", m.volume.Value))
			ui.Slider(c, &m.volume).Width(220).Step(1)
		})
		ui.Switch(c, &m.notify, "Notifications")
		ui.TextField(c, &m.name, "Name", "Your name").Width(320)
		if m.menu != "" {
			ui.Text(c, m.menu).Muted()
		}
		ui.List(c, &m.list, rows, m.row).Label("Rows").Grow(1)
	})
}

// row is one row of the list: a finger lifted on it chooses it, a finger held on it
// opens a menu.
func (m *model) row(r ui.ListRow) {
	theme := r.C.Theme()
	r.Element.Pad(12).PadX(12)
	if r.Index == m.list.Selected() {
		r.Element.Background(theme.Hover)
	}
	ui.Text(r.C, fmt.Sprintf("Row %d", r.Index+1))
	ui.ContextMenu(r.C, r.Element, func(menu *ui.MenuItems) {
		if menu.Item("Open").Clicked() {
			m.menu = fmt.Sprintf("Opened row %d", r.Index+1)
		}
		if menu.Item("Delete").Clicked() {
			m.menu = fmt.Sprintf("Deleted row %d", r.Index+1)
		}
	})
}

func main() {
	m := newModel()
	if err := ui.Run(app.Config{Title: "Touch", Width: 420, Height: 760}, ui.PageConfig{View: m.page}); err != nil {
		log.Fatal(err)
	}
}
