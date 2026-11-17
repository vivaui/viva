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
	if choice != "c" || d.focused() != "c" {
		t.Fatalf("Down chose %q with the focus on %q", choice, d.focused())
	}
	d.key(gfx.KeyDown)
	if choice != "a" {
		t.Fatalf("Down from the last chose %q, want it to wrap to a", choice)
	}
}
