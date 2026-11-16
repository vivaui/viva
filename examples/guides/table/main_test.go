package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 720, Height: 480}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// click presses the middle of the control a user would find by its name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if strings.HasPrefix(hit.Label, name) {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}

func shows(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestTheTableSortsAndSelects(t *testing.T) {
	m := newModel()
	driver := open(t, m)
	if !shows(driver, "web-01") || !shows(driver, "cache") {
		t.Fatal("the rows should be painted")
	}
	click(t, driver, "Name")
	column, descending, ok := m.table.Sort()
	if !ok || column != 0 || descending {
		t.Fatalf("sort = %d, %v, %v; the first header click sorts ascending", column, descending, ok)
	}
	click(t, driver, "web-02")
	if got := m.table.Selected(); got < 0 || !strings.Contains(m.rows[got][0], "web-02") {
		t.Fatalf("selected row = %d (%v)", got, m.rows)
	}
}
