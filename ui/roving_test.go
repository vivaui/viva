package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type rovingModel struct {
	mode, tab string
	changed   int
}

func (m *rovingModel) page(c ui.C) {
	ui.Column(c).Pad(16).Gap(8).Children(func() {
		if ui.Choice(c, &m.mode, []string{"light", "dark", "auto"}, nil).Changed() {
			m.changed++
		}
		ui.Tabs(c, &m.tab, []string{"general", "network", "about"}, nil)
		ui.Button(c, "after")
	})
}

// The arrow keys move through the options of a Choice, wrapping at the ends, and
// choose the one they land on, as a radio group does; Home and End go to the ends.
func TestChoiceArrowKeysMoveAndChoose(t *testing.T) {
	m := &rovingModel{mode: "light", tab: "general"}
	d := openPage(t, m.page)
	d.click("light")
	steps := []struct {
		key  gfx.Key
		want string
	}{
		{gfx.KeyRight, "dark"}, {gfx.KeyRight, "auto"}, {gfx.KeyRight, "light"}, // wraps
		{gfx.KeyLeft, "auto"}, {gfx.KeyHome, "light"}, {gfx.KeyEnd, "auto"},
	}
	for _, step := range steps {
		d.key(step.key)
		if d.focused() != step.want || m.mode != step.want {
			t.Fatalf("key %v: focused %q, chosen %q, want %q", step.key, d.focused(), m.mode, step.want)
		}
	}
	if m.changed != 6 {
		t.Fatalf("Changed reported %d times for 6 moves", m.changed)
	}
	d.key(gfx.KeyTab) // the Choice is one tab stop: Tab leaves it
	switch d.focused() {
	case "light", "dark", "auto":
		t.Fatalf("Tab stayed in the Choice, on %q", d.focused())
	}
}

// Tabs take the same keys, and Up and Down are not theirs when horizontal.
func TestTabsArrowKeysMoveAndChoose(t *testing.T) {
	m := &rovingModel{mode: "light", tab: "general"}
	d := openPage(t, m.page)
	d.click("general")
	d.key(gfx.KeyRight)
	if d.focused() != "network" || m.tab != "network" {
		t.Fatalf("Right: focused %q, tab %q", d.focused(), m.tab)
	}
	d.key(gfx.KeyDown)
	if m.tab != "network" {
		t.Fatalf("Down changed the tab to %q", m.tab)
	}
	d.key(gfx.KeyEnd)
	if d.focused() != "about" || m.tab != "about" {
		t.Fatalf("End: focused %q, tab %q", d.focused(), m.tab)
	}
}

// Keys in a Choice do not move the one beside it, and a key elsewhere is left alone.
func TestChoiceKeysStayInTheirControl(t *testing.T) {
	m := &rovingModel{mode: "light", tab: "general"}
	d := openPage(t, m.page)
	d.click("after")
	d.key(gfx.KeyRight)
	if m.mode != "light" || m.tab != "general" {
		t.Fatalf("an arrow on a button changed mode %q, tab %q", m.mode, m.tab)
	}
}

// Moving through a Choice repaints without allocating.
func TestChoiceRepaintAllocates(t *testing.T) {
	m := &rovingModel{mode: "light", tab: "general"}
	d := openPage(t, m.page)
	d.click("light")
	d.key(gfx.KeyRight)
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("%v allocations a repaint", allocs)
	}
}

// An arrow with a modifier is not the control's: Alt+Left is the window's.
func TestChoiceIgnoresModifiedArrows(t *testing.T) {
	m := &rovingModel{mode: "light", tab: "general"}
	d := openPage(t, m.page)
	d.click("light")
	if err := d.driver.Key(gfx.KeyRight, gfx.ModifierAlt); err != nil {
		t.Fatal(err)
	}
	if m.mode != "light" {
		t.Fatalf("Alt+Right chose %q", m.mode)
	}
}
