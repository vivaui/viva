package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

func TestTextField(t *testing.T) {
	m := &model{}
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 240}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()
	click(t, driver, "Name")
	if err := driver.Type("Ada"); err != nil {
		t.Fatal(err)
	}
	if m.name != "Ada" {
		t.Fatalf("editor = %q", m.name)
	}
	if editor := driver.Snapshot().Editor; editor == nil || editor.Value != "Ada" {
		t.Fatalf("native editor overlay = %+v", driver.Snapshot().Editor)
	}
	click(t, driver, "Save")
	if m.saved != "Ada" {
		t.Fatalf("saved = %q", m.saved)
	}
	found := false
	for _, label := range driver.Snapshot().Labels {
		found = found || strings.Contains(label.Value, "Saved Ada")
	}
	if !found {
		t.Fatal("saved label missing")
	}
	// After focus leaves, the text is a static label again.
	if driver.Snapshot().Editor != nil {
		t.Fatal("the native editor must be gone once the field loses focus")
	}
}

// click presses the middle of the control with the given accessible name.
func click(t *testing.T, driver *app.Driver, name string) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.Label == name && !hit.IsLabel {
			b := hit.Bounds
			if err := driver.Click(b.X+b.Width/2, b.Y+b.Height/2); err != nil {
				t.Fatal(err)
			}
			return
		}
	}
	t.Fatalf("no control named %q on screen", name)
}
