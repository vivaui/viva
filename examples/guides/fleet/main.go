// Fleet check in Viva: the same screen the nanoui and mygo versions show, so
// the three can be compared on equal data.
//
// Two differences from the other two are Viva's architecture, not style:
//
//   - Controls carry identities from a declared tree, not pointers into the
//     model. A checkbox reports a click; the program decides what that means.
//   - Paint is a function of state, and state changes only where a control says
//     it was clicked, so the progress figure is derived from the same rows the
//     list draws.
package main

import (
	"fmt"
	"log"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Identities are names in a tree, not numbers.
var (
	tree = gfx.Tree{}
	page = tree.Feature("fleet")

	filterScope = tree.Sub(page, tree.Name("filter"))
	scanButton  = tree.ID(page, "scan")
	listScroll  = tree.ID(page, "list")

	// One checkbox identity per row, made once at start-up.
	rows = [8]gfx.ID{
		tree.ID(page, "row-0"), tree.ID(page, "row-1"),
		tree.ID(page, "row-2"), tree.ID(page, "row-3"),
		tree.ID(page, "row-4"), tree.ID(page, "row-5"),
		tree.ID(page, "row-6"), tree.ID(page, "row-7"),
	}
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
	view   ui.View
}

const (
	filterAll = iota
	filterOpen
	filterHandled
)

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

// handledCount reads the same slice the rows come from, so the bar cannot
// disagree with the rows above it.
func (m *model) handledCount() int {
	n := 0
	for _, h := range m.hosts {
		if h.handled {
			n++
		}
	}
	return n
}

func (m *model) matches(h host) bool {
	switch m.filter {
	case filterOpen:
		return !h.handled
	case filterHandled:
		return h.handled
	}
	return true
}

func newConfig(m *model) app.Config {
	theme := gfx.LightTheme()
	config := app.Config{Tree: &tree, Title: "Viva fleet", Width: 460, Height: 560}

	config.Paint = func(canvas *gfx.Canvas, size gfx.Size) {
		bounds := gfx.Rect{Width: size.Width, Height: size.Height}
		canvas.Fill(bounds, theme.Background)

		m.view.Draw(canvas, theme, bounds, func(f *ui.Frame) {
			f.Column().Fill().Pad(22).Gap(14).Children(func() {
				f.Text("Fleet check").FontSize(26).Bold()
				f.Text("SSH posture across the hosts this workstation reaches.").FontSize(13).Muted()

				labels := []string{"All", "Open", "Handled"}
				if i, ok := f.Choice(filterScope, labels, m.filter).ChosenIndex(); ok {
					m.filter = i
				}

				// A scroll region that takes only the room left over, so the
				// footer below it stays on screen.
				f.ScrollArea(listScroll).Grow(1).Gap(8).Children(func() {
					for i := range m.hosts {
						h := &m.hosts[i]
						if !m.matches(*h) {
							continue
						}
						m.row(f, theme, rows[i], h)
					}
				})

				handled := m.handledCount()
				total := len(m.hosts)
				frac := float32(0)
				if total > 0 {
					frac = float32(handled) / float32(total)
				}
				f.Progress(frac).FillWidth()

				f.Row().FillWidth().Gap(10).Items(ui.Center).Children(func() {
					f.Text(fmt.Sprintf("%d of %d handled", handled, total)).FontSize(12).Muted().Mono()
					f.Spacer()
					f.Text(fmt.Sprintf("scanned %dx", m.scans)).FontSize(12).Muted().Mono()
					if f.Button(scanButton, "Scan again").Clicked() {
						m.scans++
					}
				})
			})
		})
	}
	return config
}

func (m *model) row(f *ui.Frame, theme gfx.Theme, id gfx.ID, h *host) {
	f.Row().FillWidth().Pad(12).Gap(12).Radius(10).Items(ui.Center).
		Background(theme.Card).Children(func() {
		label := "open"
		color := theme.Danger
		if h.handled {
			label = "ok"
			color = theme.Success
		}
		f.Text(label).FontSize(11).Color(color)

		f.Column().Grow(1).Gap(2).Children(func() {
			f.Row().Gap(8).Items(ui.Center).Children(func() {
				f.Text(h.name).FontSize(14).Bold()
				f.Text(h.addr).FontSize(12).Muted().Mono()
			})
			if h.handled {
				f.Text(h.item).FontSize(12).Muted()
			} else {
				f.Text(h.item).FontSize(12)
			}
		})

		// The checkbox only reports the click; the model is what changes, and
		// the bar above re-derives itself from it on the next paint.
		if f.Checkbox(id, h.handled).Clicked() {
			h.handled = !h.handled
		}
	})
}

func main() {
	var canvas gfx.Canvas
	if err := app.RunCanvas(newConfig(newModel()), &canvas); err != nil {
		log.Fatal(err)
	}
}
