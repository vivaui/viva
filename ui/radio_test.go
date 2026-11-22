package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestRadioGroupChoosesByClickAndArrows(t *testing.T) {
	choice := "a"
	d := openPage(t, func(c ui.C) {
		ui.RadioGroup(c).Children(func() {
			for _, name := range []string{"a", "b", "c"} {
				ui.Radio(c.Key(name), &choice, name, name)
			}
		})
	})
	d.click("b")
	if choice != "b" {
		t.Fatalf("a click chose %q", choice)
	}
	d.key(gfx.KeyDown)
	if choice != "c" || d.Focused() != "c" {
		t.Fatalf("Down chose %q with the focus on %q", choice, d.Focused())
	}
	d.key(gfx.KeyDown)
	if choice != "a" {
		t.Fatalf("Down from the last chose %q, want it to wrap to a", choice)
	}
}

func TestRadioIDKeepsTheApplicationsIdentity(t *testing.T) {
	id := gfx.Item(41)
	ungroup := false
	d := openPage(t, func(c ui.C) {
		ui.RadioGroup(c).Children(func() {
			ui.RadioID(c, gfx.Item(40), &ungroup, false, "Keep")
			ui.RadioID(c, id, &ungroup, true, "Ungroup")
		})
	})
	d.click("Ungroup")
	if !ungroup {
		t.Fatal("a click on the identified option did not choose it")
	}
	d.key(gfx.KeyUp)
	if ungroup || d.Focused() != "Keep" {
		t.Fatalf("Up chose ungroup=%v with the focus on %q", ungroup, d.Focused())
	}
}
