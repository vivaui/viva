package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{
		Width: 640, Height: 360,
		OnKey: m.split.Key,
	}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

// control finds the front-most control with the accessible name label: a row is
// named by ListState.Label, the divider and the menu items by their own names.
func control(t *testing.T, driver *app.Driver, label string) gfx.Hit {
	t.Helper()
	hits := driver.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		hit := hits[i]
		if hit.Label == label && !hit.IsLabel && !hit.Backdrop && !hit.ID.IsZero() {
			return hit
		}
	}
	t.Fatalf("no control named %q on screen", label)
	return gfx.Hit{}
}

func center(t *testing.T, driver *app.Driver, label string) (float32, float32) {
	t.Helper()
	b := control(t, driver, label).Bounds
	return b.X + b.Width/2, b.Y + b.Height/2
}

func click(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	x, y := center(t, driver, label)
	if err := driver.Click(x, y); err != nil {
		t.Fatal(err)
	}
}

func hasLabel(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func TestInput(t *testing.T) {
	m := newModel()
	driver := open(t, m)

	// Drag the divider.
	x, y := center(t, driver, "Resize panes")
	if err := driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Move(x+80, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x+80, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if m.split.Pos != 300 {
		t.Fatalf("panel width = %v", m.split.Pos)
	}

	// The divider also moves with the arrow keys, which its state keeps.
	if err := driver.Click(x+80, y); err != nil {
		t.Fatal(err)
	}
	before := m.split.Pos
	if err := driver.Key(gfx.KeyRight, 0); err != nil {
		t.Fatal(err)
	}
	if m.split.Pos <= before {
		t.Fatalf("the arrow key left the divider at %v", m.split.Pos)
	}

	// A click selects a row, and the list itself moves the choice with the keys.
	click(t, driver, "Inbox")
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if m.list.Selected() != 1 || !hasLabel(driver, "Drafts selected") {
		t.Fatalf("selected = %d", m.list.Selected())
	}

	// A secondary press opens the row's context menu; its action records the row.
	x, y = center(t, driver, "Sent")
	if err := driver.Press(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	click(t, driver, "Open")
	if m.context != "Opened Sent" || !hasLabel(driver, "Opened Sent") {
		t.Fatalf("context = %q", m.context)
	}

	// Shortcut.
	if handled, err := driver.ShortcutChord(ui.Primary, 'k'); err != nil || !handled || !m.palette {
		t.Fatalf("shortcut handled=%v err=%v palette=%v", handled, err, m.palette)
	}
	if !hasLabel(driver, "Command palette") {
		t.Fatal("the shortcut did not change the detail")
	}

	// The wheel scrolls the list under the pointer, and the list keeps the offset.
	if err := driver.Resize(640, 100); err != nil {
		t.Fatal(err)
	}
	if m.list.AtEnd() {
		t.Fatal("the list starts at its end")
	}
	x, y = center(t, driver, "Inbox")
	if err := driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: 1000}); err != nil {
		t.Fatal(err)
	}
	if !m.list.AtEnd() {
		t.Fatal("the wheel did not scroll the list")
	}
}
