package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func splitPageWith(split *ui.SplitterState) func(ui.C) {
	return func(c ui.C) {
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Width(split.Pos).Children(func() { ui.Text(c, "left") })
			ui.Splitter(c, split).Mins(80, 80).Label("Resize panes")
			ui.Column(c).Grow(1).Children(func() { ui.Text(c, "right") })
		})
	}
}

// The page routes the keys of the focused divider, so a Splitter in a page needs no
// OnKey wiring to move with the arrows.
func TestSplitterKeysAreRoutedByThePage(t *testing.T) {
	split := ui.SplitterState{Pos: 200}
	d := openPage(t, splitPageWith(&split))
	if d.focused() != "" {
		t.Fatalf("the splitter starts focused on %q", d.focused())
	}
	d.click("Resize panes")
	d.key(gfx.KeyRight)
	if split.Pos != 200+16 {
		t.Fatalf("Right moved the splitter to %v, want 216", split.Pos)
	}
	d.key(gfx.KeyEnd)
	if want := float32(480 - 8 - 80); split.Pos != want {
		t.Fatalf("End moved the splitter to %v, want %v", split.Pos, want)
	}
	d.key(gfx.KeyHome)
	if split.Pos != 80 {
		t.Fatalf("Home moved the splitter to %v, want the minimum 80", split.Pos)
	}
}

// A window that is too narrow for the width the user chose clamps the pane, and the
// width they chose comes back when the window grows again.
func TestSplitterPreferredWidthSurvivesANarrowWindow(t *testing.T) {
	split := ui.SplitterState{Pos: 300}
	d := openPage(t, splitPageWith(&split))
	if split.Pos != 300 {
		t.Fatalf("the first pane is %v, want 300", split.Pos)
	}
	if err := d.driver.Resize(340, 480); err != nil {
		t.Fatal(err)
	}
	if want := float32(340 - 8 - 80); split.Pos != want {
		t.Fatalf("in a narrow window the first pane is %v, want %v", split.Pos, want)
	}
	if err := d.driver.Resize(480, 480); err != nil {
		t.Fatal(err)
	}
	if split.Pos != 300 {
		t.Fatalf("after the window grew the first pane is %v, want its preferred 300", split.Pos)
	}
}

// A splitter that was never given a position leaves its first pane to the pane, as
// it did before: Pos starts at zero there, which is "unset", not a width, so the
// divider must not pull the pane down to its minimum.
func TestSplitterWithoutAPositionLeavesItsPaneAlone(t *testing.T) {
	var split ui.SplitterState
	page := func(c ui.C) {
		ui.Row(c).Fill().Children(func() {
			ui.Column(c).Width(200).Children(func() { ui.Text(c, "left") })
			ui.Splitter(c, &split).Mins(80, 80).Label("Resize panes")
			ui.Column(c).Grow(1).Children(func() { ui.Text(c, "right") })
		})
	}
	d := openPage(t, page)
	d.repaint()
	if got := d.hit("Resize panes").Bounds.X; got < 198 || got > 202 {
		t.Fatalf("the divider is at %v, want it after the 200-pixel pane", got)
	}
}
