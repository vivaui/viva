package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A page Toggle is the styled two-state button: a click flips the bound value
// and assistive technology hears a pressed button.
func TestPageToggleFlipsItsValue(t *testing.T) {
	var on bool
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.Toggle(c, &on, "Bold").Outline()
		})
	})
	d.click("Bold")
	if !on {
		t.Fatal("the toggle did not flip its value")
	}
	hit := d.hit("Bold")
	if !hit.Checked || !hit.Selected {
		t.Fatalf("toggle hit checked=%v selected=%v, want both", hit.Checked, hit.Selected)
	}
	d.click("Bold")
	if on {
		t.Fatal("the second click did not flip the value back")
	}
}
