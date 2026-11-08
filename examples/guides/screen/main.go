// Fleet check with flex.Screen: the page examples/guides/fleet declares with a
// Frame and a tree of identities, written without naming a control.
//
// Each checkbox writes into the host it shows, and is named by that host's field,
// so filtering the list moves no focus or click from one host to another.
package main

import (
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
)

type host struct {
	name    string
	addr    string
	item    string
	handled bool
}

type model struct {
	hosts  []host
	filter int
	scans  int
}

var filters = []string{"All", "Open", "Handled"}

func newModel() *model {
	return &model{hosts: []host{
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
	switch filters[m.filter] {
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

func (m *model) page(s flex.Screen) {
	s.Column().Fill().Pad(22).Gap(14).Children(func() {
		s.Text("Fleet check").FontSize(26).Bold()
		s.Text("SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()
		s.Choice(&m.filter, filters)

		// The list takes the room left over, so the footer stays on screen.
		s.ScrollArea("hosts").Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				if h := &m.hosts[i]; m.matches(h) {
					m.row(s, h)
				}
			}
		})

		handled, total := m.handled(), len(m.hosts)
		s.Progress(float32(handled) / float32(max(total, 1))).FillWidth()
		s.Row().FillWidth().Gap(10).Items(flex.Center).Children(func() {
			s.Textf("%d of %d handled", handled, total).FontSize(12).Muted().Mono()
			s.Spacer()
			s.Textf("scanned %dx", m.scans).FontSize(12).Muted().Mono()
			if s.Button("Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func (m *model) row(s flex.Screen, h *host) {
	theme := s.Theme()
	s.Row().FillWidth().Pad(12).Gap(12).Radius(10).Items(flex.Center).Background(theme.Card).Children(func() {
		if h.handled {
			s.Text("ok").FontSize(11).Color(theme.Success)
		} else {
			s.Text("open").FontSize(11).Color(theme.Danger)
		}
		s.Column().Grow(1).Gap(2).Children(func() {
			s.Row().Gap(8).Items(flex.Center).Children(func() {
				s.Text(h.name).FontSize(14).Bold()
				s.Text(h.addr).FontSize(12).Muted().Mono()
			})
			item := s.Text(h.item).FontSize(12)
			if h.handled {
				item.Muted()
			}
		})
		// The checkbox writes into the host; the bar above derives itself from the
		// hosts on the next paint.
		s.Checkbox(&h.handled, "").Label(h.name)
	})
}

func main() {
	if err := flex.Run(app.Config{Title: "Viva fleet", Width: 460, Height: 560}, newModel().page); err != nil {
		log.Fatal(err)
	}
}
