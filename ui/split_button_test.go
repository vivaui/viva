package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A split button declares two actions on one surface: the primary action sized to
// its label and the trigger, 32 wide by default, both nameable, described and
// reported separately, with the trigger telling assistive technology when the menu
// is open.
func TestSplitButtonJoinsTwoNamedActions(t *testing.T) {
	model := struct{ primary, menu int }{}
	view := func(c ui.C) {
		b := ui.SplitButton(c, ui.SplitButtonSpec{
			PrimaryID: gfx.Item(1), MenuID: gfx.Item(2),
			Label: "Save", MenuLabel: "Save as",
			Hint: "Save the file", MenuHint: "More ways to save",
			Open: true,
		})
		if id, ok := b.Chosen(); ok {
			if id == gfx.Item(1) {
				model.primary++
			} else if id == gfx.Item(2) {
				model.menu++
			}
		}
	}
	d := openPage(t, view)
	primary, menu := d.hit("Save"), d.hit("Save as")
	if primary.Role != gfx.RoleButton || menu.Role != gfx.RoleButton {
		t.Errorf("roles %v and %v, want buttons", primary.Role, menu.Role)
	}
	if primary.Hint != "Save the file" || menu.Hint != "More ways to save" {
		t.Errorf("hints %q and %q, want the ones declared", primary.Hint, menu.Hint)
	}
	if !menu.Expandable || !menu.Expanded {
		t.Errorf("trigger expandable=%v expanded=%v, want an expanded menu", menu.Expandable, menu.Expanded)
	}
	if menu.Bounds.Width != 32 {
		t.Errorf("trigger width %v, want the default 32", menu.Bounds.Width)
	}
	if menu.Bounds.Height != primary.Bounds.Height || menu.Bounds.Y != primary.Bounds.Y {
		t.Errorf("halves %v and %v do not share a surface", primary.Bounds, menu.Bounds)
	}
	if want := primary.Bounds.X + primary.Bounds.Width; menu.Bounds.X != want {
		t.Errorf("trigger starts at %v, want %v, at the primary's edge", menu.Bounds.X, want)
	}

	d.click("Save")
	if model.primary != 1 || model.menu != 0 {
		t.Errorf("a click on the primary action reported primary=%d menu=%d, want 1 and 0", model.primary, model.menu)
	}
	d.click("Save as")
	if model.primary != 1 || model.menu != 1 {
		t.Errorf("a click on the trigger reported primary=%d menu=%d, want 1 and 1", model.primary, model.menu)
	}
}

// A split button declared without identities is named by the page, as a Button is,
// and still reports the half a click chose.
func TestSplitButtonNamesItsHalvesWhenGivenNoIdentity(t *testing.T) {
	var chosen gfx.ID
	view := func(c ui.C) {
		if id, ok := ui.SplitButton(c, ui.SplitButtonSpec{Label: "Save", MenuLabel: "Save as"}).Chosen(); ok {
			chosen = id
		}
	}
	d := openPage(t, view)
	primary, menu := d.hit("Save"), d.hit("Save as")
	if primary.ID.IsZero() || menu.ID.IsZero() || primary.ID == menu.ID {
		t.Errorf("page-named halves are %v and %v, want two distinct identities", primary.ID, menu.ID)
	}
	chosen = gfx.ID{}
	d.click("Save")
	if chosen != primary.ID {
		t.Errorf("a click on the primary reported %v, want %v", chosen, primary.ID)
	}
	chosen = gfx.ID{}
	d.click("Save as")
	if chosen != menu.ID {
		t.Errorf("a click on the trigger reported %v, want %v", chosen, menu.ID)
	}
}
