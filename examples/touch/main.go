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
	// lifecycle is what the system last did to the application: it went to the background, came
	// back, or asked for memory.
	lifecycle string
	// detail is the row open on its own page, or -1: going back closes it.
	detail int
	// invalidator asks for a new frame after a change the page was not told of.
	invalidator *app.Invalidator
}

func newModel() *model {
	m := &model{volume: ui.SliderState{Value: 40}, detail: -1}
	m.list.Label = func(i int) string { return fmt.Sprintf("Row %d", i+1) }
	return m
}

// back is the user going back: the open row's page closes. It reports whether it did, and
// with the list showing the system's answer follows (an Android phone closes the window).
func (m *model) back() bool {
	if m.detail < 0 {
		return false
	}
	m.detail = -1
	m.list.Select(-1) // the same row opens again when it is tapped again
	return true
}

// detailPage is a row on a page of its own, which the back gesture closes.
func (m *model) detailPage(c ui.C) {
	ui.SafeArea(c).Children(func() {
		ui.Column(c).Fill().Pad(16).Gap(12).Children(func() {
			ui.Text(c, fmt.Sprintf("Row %d", m.detail+1)).FontSize(24).Weight(600)
			ui.Text(c, "Swipe back, or press back, to return to the list.").Muted()
			if ui.Button(c, "Back").Clicked() {
				m.detail = -1
			}
		})
	})
}

func (m *model) page(c ui.C) {
	if m.detail >= 0 {
		m.detailPage(c)
		return
	}
	ui.SafeArea(c).Children(func() {
		ui.Column(c).Fill().Pad(16).Gap(12).Items(ui.Stretch).Children(func() {
			ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
				if ui.Button(c, "Tap me").Tooltip("Counts your taps").Clicked() {
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
			if m.lifecycle != "" {
				ui.Text(c, "System: "+m.lifecycle).Muted()
			}
			ui.List(c, &m.list, rows, m.row).Label("Rows").Grow(1)
			if m.list.Changed() && m.list.Selected() >= 0 {
				m.detail = m.list.Selected() // a tap chooses a row, and a phone opens it
			}
		})
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

// systemChanged notes what the system did; the next frame shows it.
func (m *model) systemChanged(event app.LifecycleEvent) {
	switch event {
	case app.LifecycleBackground:
		m.lifecycle = "background"
	case app.LifecycleForeground:
		m.lifecycle = "foreground"
	case app.LifecycleMemoryWarning:
		m.lifecycle = "memory warning"
	}
	m.invalidator.Invalidate()
}

func main() {
	m := newModel()
	m.invalidator = new(app.Invalidator)
	config := app.Config{Title: "Touch", Width: 420, Height: 760, OnLifecycle: m.systemChanged, OnBack: m.back}
	if err := ui.Run(config, ui.PageConfig{View: m.page, Invalidator: m.invalidator}); err != nil {
		log.Fatal(err)
	}
}
