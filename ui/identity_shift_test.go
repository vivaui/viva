package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A button that appears before the focused one takes its number among its kind,
// and with it its identity: the focus follows the button it was on, so Enter
// presses Save, not the banner's Dismiss that now holds Save's old name.
func TestFocusFollowsAButtonWhoseNumberMoved(t *testing.T) {
	show, got := false, ""
	d := openPage(t, func(c ui.C) {
		if show {
			if ui.Button(c, "Dismiss banner").Clicked() {
				got = "dismiss"
			}
		}
		if ui.Button(c, "Save").Clicked() {
			got = "save"
		}
	})
	d.key(gfx.KeyTab)
	show = true
	d.repaint()
	if d.Focused() != "Save" {
		t.Fatalf("a button appearing before Save took the focus: %q", d.Focused())
	}
	d.key(gfx.KeyEnter)
	if got != "save" {
		t.Fatalf("Enter pressed %q", got)
	}
}

// A focused button whose own label changes is the same button: it keeps the focus.
func TestARelabeledButtonKeepsTheFocus(t *testing.T) {
	label := "Connect"
	d := openPage(t, func(c ui.C) {
		ui.Button(c, label)
		ui.Button(c, "Cancel")
	})
	d.key(gfx.KeyTab)
	label = "Connecting…"
	d.repaint()
	if d.Focused() != "Connecting…" {
		t.Fatalf("the relabeled button lost the focus to %q", d.Focused())
	}
}
