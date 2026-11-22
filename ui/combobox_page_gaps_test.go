package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func roleHits(d *pageDriver, role gfx.Role) int {
	count := 0
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == role && !hit.IsLabel && !hit.Backdrop {
			count++
		}
	}
	return count
}

// The page API can start a Combobox open with a caller-owned query, and mark the
// field invalid even while its list shows. Both starting states apply once: Escape
// closes the list and a later paint does not open it again.
func TestComboboxInitiallyOpenQueryAndInvalid(t *testing.T) {
	frameworks := []string{"Viva Native", "WebGPU", "Headless tests"}
	var value string
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.Combobox(c, &value, frameworks, nil, "Framework").
				Empty("No matching frameworks").Invalid(true).InitiallyOpen(true).InitialQuery("mobile")
		})
	})
	if !d.page.Overlay() {
		t.Fatal("InitiallyOpen did not declare the open list as an overlay")
	}
	field := d.hit("Framework")
	if field.Value != "mobile" {
		t.Fatalf("the initial query left the field showing %q", field.Value)
	}
	if roleHits(d, gfx.RoleOption) != 0 {
		t.Fatal("the query mobile matched an option")
	}
	snapshot := d.driver.Snapshot()
	empty := false
	for _, text := range snapshot.Texts {
		if text.Value == "No matching frameworks" {
			empty = true
		}
	}
	if !empty {
		t.Fatal("the no-match row was not painted")
	}
	invalid := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Color == pageTheme.Danger {
			invalid = true
		}
	}
	if !invalid {
		t.Fatal("the field did not paint the destructive border")
	}
	d.key(gfx.KeyEscape)
	if d.page.Overlay() || roleHits(d, gfx.RoleOption) != 0 {
		t.Fatal("Escape did not close the initially open list")
	}
	d.repaint()
	if d.page.Overlay() || roleHits(d, gfx.RoleOption) != 0 {
		t.Fatal("a later paint reopened the list the user closed")
	}
}

// The initial options are one-shot declarations, so a warm open list paints
// without allocating.
func TestComboboxInitialOptionsPaintWithoutAllocating(t *testing.T) {
	choice := "beta"
	d := openPage(t, func(c ui.C) {
		ui.Combobox(c, &choice, []string{"alpha", "beta", "gamma"}, nil, "Choice").
			Clearable().Invalid(true).InitiallyOpen(true).InitialQuery("a")
	})
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("a warm combobox with initial options paints with %v allocations, want 0", allocs)
	}
}
