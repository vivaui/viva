package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type focusGroupModel struct {
	axis  ui.FocusAxis
	query string
	picks []string
}

func (m *focusGroupModel) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		ui.Button(c, "before")
		ui.Row(c).Gap(4).FocusGroup(m.axis).Children(func() {
			for _, name := range []string{"cut", "copy", "paste"} {
				if ui.Button(c.Key(name), name).Clicked() {
					m.picks = append(m.picks, name)
				}
			}
			ui.TextField(c, &m.query, "Find", "").Width(120)
			ui.Button(c, "undo")
		})
		ui.Button(c, "after")
	})
}

// The controls of a group are one stop of Tab; the arrows of its axis move among them,
// wrapping, and Home and End go to the ends.
func TestFocusGroupIsOneTabStopWithArrows(t *testing.T) {
	m := &focusGroupModel{axis: ui.FocusHorizontal}
	d := openPage(t, m.page)
	d.click("before")
	d.key(gfx.KeyTab)
	if d.focused() != "cut" {
		t.Fatalf("Tab into the group reached %q", d.focused())
	}
	d.key(gfx.KeyTab)
	if d.focused() != "Find" {
		t.Fatalf("Tab from the group's stop reached %q, not the field that keeps its own", d.focused())
	}

	d.click("copy")
	pressed := len(m.picks) // the click pressed copy
	for _, step := range []struct {
		key  gfx.Key
		want string
	}{
		{gfx.KeyRight, "paste"}, {gfx.KeyRight, "undo"}, {gfx.KeyRight, "cut"}, // wraps
		{gfx.KeyLeft, "undo"}, {gfx.KeyHome, "cut"}, {gfx.KeyEnd, "undo"},
	} {
		d.key(step.key)
		if d.focused() != step.want {
			t.Fatalf("key %v: focused %q, want %q", step.key, d.focused(), step.want)
		}
	}
	// Arrows move the focus, not the choice: nothing was activated.
	if len(m.picks) != pressed {
		t.Fatalf("arrows pressed %v", m.picks[pressed:])
	}
	d.key(gfx.KeyHome)
	d.key(gfx.KeyEnter)
	if len(m.picks) != pressed+1 || m.picks[pressed] != "cut" {
		t.Fatalf("Enter on cut pressed %v", m.picks[pressed:])
	}
}

// Tab comes back to the member that had the focus last, and out of the group in
// both directions.
func TestFocusGroupRemembersItsStop(t *testing.T) {
	m := &focusGroupModel{axis: ui.FocusHorizontal}
	d := openPage(t, m.page)
	d.click("paste")
	d.click("after")
	if err := d.driver.Key(gfx.KeyTab, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if d.focused() != "Find" {
		// undo is a member of the group, so Tab passes over it: the field is before after.
		t.Fatalf("Shift+Tab from after reached %q", d.focused())
	}
	if err := d.driver.Key(gfx.KeyTab, gfx.ModifierShift); err != nil {
		t.Fatal(err)
	}
	if d.focused() != "paste" {
		t.Fatalf("Shift+Tab back into the group reached %q, not the stop it had, paste", d.focused())
	}
}

// A text field inside keeps its arrows, and the other axis' arrows are not the group's.
func TestFocusGroupLeavesWhatIsNotItsAxis(t *testing.T) {
	m := &focusGroupModel{axis: ui.FocusHorizontal}
	d := openPage(t, m.page)
	d.click("Find")
	d.typeText("ab")
	d.key(gfx.KeyLeft)
	if d.focused() != "Find" {
		t.Fatalf("Left in the field moved the focus to %q", d.focused())
	}

	d.click("copy")
	d.key(gfx.KeyDown)
	if d.focused() != "copy" {
		t.Fatalf("Down in a horizontal group moved to %q", d.focused())
	}
	v := &focusGroupModel{axis: ui.FocusVertical}
	dv := openPage(t, v.page)
	dv.click("copy")
	dv.key(gfx.KeyRight)
	dv.key(gfx.KeyLeft)
	if dv.focused() != "copy" {
		t.Fatalf("Left or Right in a vertical group moved to %q", dv.focused())
	}
	dv.key(gfx.KeyDown)
	if dv.focused() != "paste" {
		t.Fatalf("Down in a vertical group reached %q", dv.focused())
	}
}

// A group repaints without allocating.
func TestFocusGroupRepaintAllocates(t *testing.T) {
	m := &focusGroupModel{axis: ui.FocusBoth}
	d := openPage(t, m.page)
	d.click("copy")
	d.key(gfx.KeyRight)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}
