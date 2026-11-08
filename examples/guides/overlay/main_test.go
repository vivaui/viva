package main

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/flex"
	"github.com/vivaui/viva/ui"
)

func TestOverlays(t *testing.T) {
	m := &model{}
	page := flex.NewPage(flex.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 640, Height: 360}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	click(t, driver, "Actions")
	if !m.menuOpen {
		t.Fatal("menu did not open")
	}
	if err := driver.Click(500, 300); err != nil {
		t.Fatal(err)
	}
	if m.menuOpen {
		t.Fatal("an outside press must close the menu")
	}

	click(t, driver, "Actions")
	if err := driver.Key(ui.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if m.menuOpen {
		t.Fatal("Escape must close the menu")
	}

	// Delete… opens the dialog; confirming closes it.
	click(t, driver, "Actions")
	click(t, driver, "Delete…")
	if !m.dialogOpen || m.menuOpen {
		t.Fatalf("dialog=%v menu=%v", m.dialogOpen, m.menuOpen)
	}
	click(t, driver, "Delete")
	if m.dialogOpen || m.status != "Deleted" {
		t.Fatalf("dialog=%v status=%q", m.dialogOpen, m.status)
	}
}

// click presses the middle of the control with the given accessible name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	snapshot := driver.Snapshot()
	for i := len(snapshot.Hits) - 1; i >= 0; i-- { // the front-most first
		if hit := snapshot.Hits[i]; hit.Label == name && !hit.IsLabel && !hit.Backdrop {
			if err := driver.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q", name)
}
