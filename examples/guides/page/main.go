// Fleet check written as a page: the same screen the nanoui and mygo versions
// show, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter string
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{filter: "All", hosts: []host{
		{name: "edge-01", addr: "10.0.4.11", item: "Disable password auth"},
		{name: "edge-02", addr: "10.0.4.12", item: "Rotate host key"},
		{name: "build-01", addr: "10.0.8.3", item: "Pin to ed25519"},
		{name: "build-02", addr: "10.0.8.4", item: "Restrict agent forwarding", handled: true},
		{name: "db-primary", addr: "10.0.9.2", item: "Close port 22 to the world"},
		{name: "db-replica", addr: "10.0.9.3", item: "Enable key-only login", handled: true},
		{name: "jump-01", addr: "10.0.1.5", item: "Set a login grace time"},
		{name: "nas-01", addr: "10.0.12.7", item: "Upgrade OpenSSH"},
	}}
}

func (m *model) matches(h *host) bool {
	switch m.filter {
	case "Open":
		return !h.handled
	case "Handled":
		return h.handled
	}
	return true
}

func (m *model) handled() int {
	n := 0
	for i := range m.hosts {
		if m.hosts[i].handled {
			n++
		}
	}
	return n
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(22).Gap(14).Children(func() {
		ui.Text(c, "Fleet check").FontSize(26).Bold()
		ui.Text(c, "SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		ui.Choice(c, &m.filter, filters, nil)

		// The list takes the room left over, so the footer stays on screen.
		ui.Scroll(c).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(c, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		ui.Progress(c, float32(handled)/float32(max(total, 1))).FillWidth()
		ui.Row(c).FillWidth().Gap(10).Items(ui.Center).Children(func() {
			ui.Textf(c, "%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			ui.Spacer(c)
			ui.Textf(c, "scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if ui.Button(c, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

// row is one host: a component of the page's own, written as Viva's elements are.
func (m *model) row(c ui.C, h *host) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Pad(12).Gap(12).Radius(10).Items(ui.Center).Background(theme.Card).Children(func() {
		if h.handled {
			ui.Text(c, "ok").FontSize(11).Color(theme.Success)
		} else {
			ui.Text(c, "open").FontSize(11).Color(theme.Danger)
		}
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
				ui.Text(c, h.name).FontSize(14).Bold()
				ui.Text(c, h.addr).FontSize(12).Muted().Mono()
			})
			item := ui.Text(c, h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		ui.Checkbox(c, &h.handled, "").Label(h.name)
	})
}

func main() {
	if err := ui.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
