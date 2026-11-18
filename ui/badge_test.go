package ui_test

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Badge paints the component's chip at the size its label measures, is
// presentational by default, and reports a click only when given an identity.
func TestBadgeIsPresentationalUntilItIsGivenAnIdentity(t *testing.T) {
	var hit ui.Box
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(10).Children(func() {
			ui.Badge(c, "beta", components.BadgeSecondary)
			hit = ui.Badge(c, "remove ×", components.BadgeDestructive).Hit(gfx.Item(1), "remove ×")
		})
	}
	d := openPage(t, view)
	target := d.hit("remove ×")
	if target.Role != gfx.RoleButton {
		t.Errorf("the badge's role is %v, want a button", target.Role)
	}
	if target.ID != gfx.Item(1) {
		t.Errorf("the badge's identity is %v, want the one Hit gave", target.ID)
	}
	painted := false
	for _, text := range d.driver.Snapshot().Texts {
		if text.Value == "remove ×" && text.Bounds.X >= target.Bounds.X && text.Bounds.X < target.Bounds.X+target.Bounds.Width &&
			text.Bounds.Y >= target.Bounds.Y && text.Bounds.Y < target.Bounds.Y+target.Bounds.Height {
			painted = true
		}
	}
	if !painted {
		t.Error("the badge's label is not painted inside its hit")
	}
	for _, other := range d.driver.Snapshot().Hits {
		if other.Label == "beta" {
			t.Error("a badge without Hit declared a target")
		}
	}
	// It sits above the first badge and shares its row's baseline.
	if hit.Rect().Y <= 10 {
		t.Errorf("the second badge is at y %v, want it below the first", hit.Rect().Y)
	}
}
