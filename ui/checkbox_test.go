package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// A mixed Checkbox paints the dash of components.Checkbox.Indeterminate: its fill is the
// checked fill, not the unchecked one, and a press still reaches the application.
func TestCheckboxMixedPaintsTheDash(t *testing.T) {
	fill := func(mixed bool) (primitives int) {
		on := false
		d := openPage(t, func(c ui.C) {
			ui.Column(c).Pad(16).Children(func() {
				ui.Checkbox(c, &on, "All").Mixed(mixed)
			})
		})
		for _, p := range d.driver.Snapshot().Primitives {
			if p.Kind == app.RectanglePrimitive && p.Color == pageTheme.Primary {
				primitives++
			}
		}
		return primitives
	}
	if fill(false) != 0 {
		t.Fatal("an unchecked box has a primary fill")
	}
	if fill(true) == 0 {
		t.Fatal("a mixed box shows no primary fill")
	}
}
