package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Radio is the theme's option: it chooses, paints its circle in the theme's primary,
// and a disabled option is inert and marked so.
func TestRadioChoosesPaintsAndHonoursDisabled(t *testing.T) {
	var size string
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Gap(8).Children(func() {
			ui.Radio(c, &size, "small", "Small")
			ui.Radio(c, &size, "large", "Large")
			ui.Radio(c, &size, "huge", "Huge").Disabled(true)
		})
	})
	if hit := d.hit("Small"); hit.Role != gfx.RoleRadio || hit.Checked {
		t.Fatalf("Small hit = %+v, want an unchosen radio", hit)
	}
	d.click("Large")
	if size != "large" {
		t.Fatalf("Radio chose %q", size)
	}
	d.click("Huge")
	if size != "large" {
		t.Fatalf("a disabled Radio chose %q", size)
	}
	disabled := d.hit("Huge")
	if !disabled.Disabled || disabled.Focusable {
		t.Fatalf("a disabled Radio's hit = %+v, want disabled and not focusable", disabled)
	}
	d.repaint()
	if !d.hit("Large").Checked || d.hit("Small").Checked {
		t.Fatal("assistive technology is not told which Radio is chosen")
	}
	// The chosen circle is filled in the theme's primary, on the option's left edge.
	chosen := d.hit("Large")
	found := false
	for _, p := range d.driver.Snapshot().Primitives {
		if p.Color == pageTheme.Primary && p.Bounds.Width == 16 && p.Bounds.Height == 16 &&
			p.Bounds.X == chosen.Bounds.X && p.Bounds.Y >= chosen.Bounds.Y && p.Bounds.Y+p.Bounds.Height <= chosen.Bounds.Y+chosen.Bounds.Height {
			found = true
		}
	}
	if !found {
		t.Fatal("the chosen Radio painted no primary circle")
	}
}

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
