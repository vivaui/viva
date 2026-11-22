package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// An alert dialog on a page opens on its bool, as a Modal does: the keyboard
// starts on Cancel, the confirming action reports true and closes it, and Escape
// closes it without confirming.
func TestAPageAlertDialogConfirmsOnItsBool(t *testing.T) {
	open, confirmed := true, 0
	d := openPage(t, func(c ui.C) {
		ui.Button(c, "Delete host")
		if ui.AlertDialog(c, &open, ui.AlertDialogSpec{
			Title: "Delete host?", Description: "This cannot be undone.",
			ActionLabel: "Delete", CancelLabel: "Cancel", ActionVariant: ui.ButtonDestructive,
		}) {
			confirmed++
		}
	})
	if d.Focused() != "Cancel" {
		t.Fatalf("the alert dialog opened with the keyboard on %q", d.Focused())
	}
	d.click("Delete")
	if open || confirmed != 1 {
		t.Fatalf("the confirming action left open=%v and confirmed %d times", open, confirmed)
	}

	open = true
	d.repaint()
	d.key(gfx.KeyEscape)
	if open || confirmed != 1 {
		t.Fatalf("Escape left open=%v and confirmed %d times", open, confirmed)
	}
}
