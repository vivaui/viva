package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func open(t *testing.T, m *model) (*app.Driver, *app.Invalidator) {
	t.Helper()
	invalidator := new(app.Invalidator)
	page := flex.NewPage(flex.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 460, Height: 900, Invalidator: invalidator}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver, invalidator
}

func control(t *testing.T, driver *app.Driver, name string) ui.Hit {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			return hit
		}
	}
	t.Fatalf("no control named %q", name)
	return ui.Hit{}
}

func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	b := control(t, driver, name).Bounds
	if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
		t.Fatal(err)
	}
}

func focusedName(driver *app.Driver) string {
	snapshot := driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused && !hit.IsLabel {
			return hit.Label
		}
	}
	return ""
}

func TestCheckboxesWriteIntoTheirHosts(t *testing.T) {
	m := newModel()
	driver, _ := open(t, m)
	click(t, driver, "edge-02")
	if !m.hosts[1].handled || m.handled() != 3 {
		t.Fatalf("edge-02 handled=%v, %d handled", m.hosts[1].handled, m.handled())
	}
	click(t, driver, "Scan again")
	if m.scans != 1 {
		t.Fatalf("scans = %d", m.scans)
	}
}

// Filtering moves the rows; the focused checkbox stays with its host, and Space
// toggles that host.
func TestFilteringKeepsTheFocusOnItsHost(t *testing.T) {
	m := newModel()
	driver, invalidator := open(t, m)
	click(t, driver, "jump-01")
	if !m.hosts[6].handled {
		t.Fatal("the click did not reach jump-01")
	}
	m.filter = "Handled" // the rows above jump-01 change
	invalidator.Invalidate()
	if err := driver.Pump(); err != nil {
		t.Fatal(err)
	}
	if focusedName(driver) != "jump-01" {
		t.Fatalf("focus is on %q after filtering, want jump-01", focusedName(driver))
	}
	if err := driver.Key(ui.KeySpace, 0); err != nil {
		t.Fatal(err)
	}
	if m.hosts[6].handled || m.handled() != 2 {
		t.Fatalf("Space toggled the wrong host: jump-01 handled=%v, %d handled", m.hosts[6].handled, m.handled())
	}
}
