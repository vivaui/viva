package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestKbdLaysKeycapsSideBySide(t *testing.T) {
	var group ui.Box
	openPage(t, func(c ui.C) {
		ui.Column(c).Pad(8).Children(func() { group = ui.Kbd(c, "⌘", "W") })
	})
	r := group.Rect()
	if r.Width <= 2*20 || r.Height < 12 {
		t.Fatalf("keycaps are %vx%v, want two chips side by side", r.Width, r.Height)
	}
}
