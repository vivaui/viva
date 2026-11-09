package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type zone struct {
	id   int
	name string
}

func (z zone) String() string { return z.name }

var zones = []zone{{1, "Asia/Shanghai"}, {2, "Asia/Tokyo"}, {3, "Europe/Berlin"}, {4, "Europe/London"}, {5, "America/New_York"}}

type comboModel struct {
	zone    zone
	changes int
	saves   int
}

func (m *comboModel) page(c ui.C) {
	if ui.Shortcut(c, 0, "f2") { // a plain key the field must keep
		m.saves++
	}
	ui.Column(c).Pad(16).Gap(8).Items(ui.Start).Children(func() {
		if ui.Combobox(c, &m.zone, zones, zone.String, "Time zone").Width(240).Changed() {
			m.changes++
		}
		ui.Button(c, "After")
	})
}

// options lists the labels of the open list's rows.
func options(d *pageDriver) []string {
	var out []string
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleOption && !hit.IsLabel {
			out = append(out, hit.Label)
		}
	}
	return out
}

// Typing filters the choices, the arrows and Enter choose one, and the choice is
// written to the value, by type.
func TestComboboxFiltersAndChooses(t *testing.T) {
	m := &comboModel{}
	d := openPage(t, m.page)
	d.click("Time zone")
	d.typeText("europe")
	if got := options(d); len(got) != 2 || got[0] != "Europe/Berlin" || got[1] != "Europe/London" {
		t.Fatalf("typing europe lists %v", got)
	}
	d.key(gfx.KeyDown)
	d.key(gfx.KeyEnter)
	if m.zone != zones[3] || m.changes != 1 {
		t.Fatalf("chose %v, changes %d", m.zone, m.changes)
	}
	if len(options(d)) != 0 {
		t.Fatal("the list stayed open after a choice")
	}
	if hit := d.hit("Time zone"); hit.Value != "Europe/London" {
		t.Fatalf("the field shows %q", hit.Value)
	}
}

// A click chooses; Escape closes without choosing; the program's value is shown.
func TestComboboxClickEscapeAndProgram(t *testing.T) {
	m := &comboModel{zone: zones[0]}
	d := openPage(t, m.page)
	d.click("Time zone")
	if !d.page.Overlay() || len(options(d)) != len(zones) {
		t.Fatalf("overlay %v, rows %v", d.page.Overlay(), options(d))
	}
	d.click("Asia/Tokyo")
	if m.zone != zones[1] {
		t.Fatalf("a click chose %v", m.zone)
	}
	d.click("Time zone")
	d.typeText("ber")
	d.key(gfx.KeyEscape)
	if m.zone != zones[1] || len(options(d)) != 0 {
		t.Fatalf("Escape: zone %v, rows %v", m.zone, options(d))
	}
	m.zone = zones[4]
	d.repaint()
	if hit := d.hit("Time zone"); hit.Value != "America/New_York" || m.changes != 1 {
		t.Fatalf("the program's value: shown %q, changes %d", hit.Value, m.changes)
	}
}

// A plain key typed in the field is the field's, not a page Shortcut's.
func TestComboboxKeepsItsKeys(t *testing.T) {
	m := &comboModel{}
	d := openPage(t, m.page)
	d.click("Time zone")
	d.key(gfx.KeyEscape) // the list is closed: the overlay no longer holds the keys
	d.key(gfx.KeyF2)
	if m.saves != 0 {
		t.Fatal("F2 in the field acted as the page's Shortcut")
	}
}

func BenchmarkComboboxOpen(b *testing.B) {
	m := &comboModel{}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.page)
	}
}
