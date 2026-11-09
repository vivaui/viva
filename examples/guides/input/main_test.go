package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func hasLabel(driver *app.Driver, want string) bool {
	for _, label := range driver.Snapshot().Labels {
		if strings.Contains(label.Value, want) {
			return true
		}
	}
	return false
}

func center(t *testing.T, driver *app.Driver, id gfx.ID) (float32, float32) {
	t.Helper()
	for _, hit := range driver.Snapshot().Hits {
		if hit.ID == id {
			return hit.Bounds.X + hit.Bounds.Width/2, hit.Bounds.Y + hit.Bounds.Height/2
		}
	}
	t.Fatalf("no control %v on screen", id)
	return 0, 0
}

func TestInput(t *testing.T) {
	m := &model{split: ui.SplitterState{Pos: 220}}
	var canvas gfx.Canvas
	driver, err := app.NewDriver(newConfig(m), &canvas)
	if err != nil {
		t.Fatal(err)
	}
	defer driver.Close()

	// Drag the divider.
	x, y := center(t, driver, divider)
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

	// A click selects a row, and keys reach the focused row.
	x, y = center(t, driver, rows.At(0))
	if err := driver.Click(x, y); err != nil {
		t.Fatal(err)
	}
	if err := driver.Key(gfx.KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if m.selected != 1 || !hasLabel(driver, "Drafts selected") {
		t.Fatalf("selected = %d", m.selected)
	}

	// A secondary press is seen by OnPointer.
	x, y = center(t, driver, rows.At(2))
	if err := driver.Press(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if err := driver.Release(x, y, gfx.PointerButtonSecondary); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver, "Context menu for Sent") {
		t.Fatal("secondary press did not reach OnPointer")
	}

	// Shortcut.
	if handled, err := driver.ShortcutChord(primary(), 'k'); err != nil || !handled || !m.palette {
		t.Fatalf("shortcut handled=%v err=%v palette=%v", handled, err, m.palette)
	}

	// The wheel scrolls the list under the pointer.
	if err := driver.Resize(640, 100); err != nil {
		t.Fatal(err)
	}
	x, y = center(t, driver, rows.At(0))
	if err := driver.Scroll(gfx.ScrollEvent{X: x, Y: y, DeltaY: 40}); err != nil {
		t.Fatal(err)
	}
	if m.view.ScrollState(list).Offset != 40 {
		t.Fatalf("offset = %v", m.view.ScrollState(list).Offset)
	}
}
