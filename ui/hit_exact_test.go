package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// HitExact gives a Checkbox or Switch the slot it was declared in as its
// pointer target, instead of the enlarged desktop target around its painted
// square or track.
func TestHitExactUsesTheSlot(t *testing.T) {
	var checked bool
	var checkbox, switchBox ui.Box
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Gap(8).Children(func() {
			checkbox = ui.Checkbox(c, &checked, "Select").HitExact().Size(22, 24)
			switchBox = ui.Switch(c, &checked, "Enabled").HitExact().Size(52, 32)
		})
	})
	if got, want := d.hit("Select").Bounds, checkbox.Rect(); got != want {
		t.Fatalf("checkbox hit %v, slot %v", got, want)
	}
	if got, want := d.hit("Enabled").Bounds, switchBox.Rect(); got != want {
		t.Fatalf("switch hit %v, slot %v", got, want)
	}
}
