package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A click on a disabled RadioBase is not a choice: Box.Disabled is the option's, not
// only its look.
func TestRadioBaseHonoursDisabled(t *testing.T) {
	value := "one"
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.RadioBase(c, &value, "one", "one").Pad(6).Children(func() { ui.Text(c, "one") })
			ui.RadioBase(c, &value, "two", "two").Pad(6).Children(func() { ui.Text(c, "two") }).Disabled(true)
		})
	})
	d.click("two")
	if value != "one" {
		t.Fatalf("a disabled RadioBase chose %q", value)
	}
	if hit := d.hit("two"); !hit.Disabled {
		t.Fatalf("a disabled RadioBase's hit = %+v", hit)
	}
}
