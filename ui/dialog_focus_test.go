package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Dialog whose spec names no initial focus takes the keyboard on the first
// control of its body as it opens, as a Modal does, and gives it back to the
// control that opened it as it closes.
func TestADialogTakesTheKeyboardOnItsFirstControl(t *testing.T) {
	open := false
	d := openPage(t, func(c ui.C) {
		if ui.Button(c, "Open dialog").Clicked() {
			open = true
		}
		if open {
			dialog := ui.Dialog(c, ui.DialogSpec{Title: "Rename", Scope: gfx.Item(900), DismissID: gfx.Item(901), CloseID: gfx.Item(902)})
			dialog.Body(func() { ui.Button(c, "First") })
			if dialog.Dismissed() {
				open = false
			}
		}
	})
	d.key(gfx.KeyTab)
	d.key(gfx.KeyEnter)
	if d.Focused() != "First" {
		t.Fatalf("the dialog opened with the keyboard on %q", d.Focused())
	}
	d.key(gfx.KeyEscape)
	if open || d.Focused() != "Open dialog" {
		t.Fatalf("closing: open %v, keyboard on %q", open, d.Focused())
	}
}
