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

type groupModel struct {
	groups  []string
	group   string
	changes int
	created []string
}

func (m *groupModel) page(c ui.C) {
	ui.Column(c).Pad(16).Items(ui.Start).Children(func() {
		field := ui.Combobox(c, &m.group, m.groups, nil, "Group").Width(240).Clearable().Empty("No groups").Creatable("New group ")
		if field.Changed() {
			m.changes++
		}
		if name, ok := field.Created(); ok {
			m.created = append(m.created, name)
			m.groups = append(m.groups, name)
			m.group = name
		}
	})
}

// What is typed and no choice yet is offered as one: Created reports it, the
// application adds it, and the list does not then show it twice.
func TestComboboxCreates(t *testing.T) {
	m := &groupModel{groups: []string{"prod", "staging"}}
	d := openPage(t, m.page)
	d.click("Group")
	d.typeText("dev")
	if got := options(d); len(got) != 1 || got[0] != "New group dev" {
		t.Fatalf("typing a new name lists %v", got)
	}
	d.key(gfx.KeyEnter)
	d.repaint()
	if len(m.created) != 1 || m.created[0] != "dev" || m.group != "dev" {
		t.Fatalf("created %v, group %q", m.created, m.group)
	}
	d.click("Group")
	count := 0
	for _, label := range options(d) {
		if label == "dev" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("the created group shows %d times: %v", count, options(d))
	}
}

// Clear sets the value back to none.
func TestComboboxClear(t *testing.T) {
	m := &groupModel{groups: []string{"prod", "staging"}, group: "prod"}
	d := openPage(t, m.page)
	d.click("Clear Group")
	if m.group != "" || m.changes != 1 {
		t.Fatalf("clear: group %q, changes %d", m.group, m.changes)
	}
}

// A list with nothing to show says what Empty says.
func TestComboboxEmpty(t *testing.T) {
	chosen := zones[0]
	d := openPage(t, func(c ui.C) {
		ui.Combobox(c, &chosen, zones, zone.String, "Time zone").Width(240).Empty("No zone matches")
	})
	d.click("Time zone")
	d.typeText("mars")
	if len(options(d)) != 0 || !tooltipShown(d, "No zone matches") {
		t.Fatalf("rows %v, the empty text shown %v", options(d), tooltipShown(d, "No zone matches"))
	}
}
