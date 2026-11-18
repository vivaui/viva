package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func TestColorSwatchChoosesByClick(t *testing.T) {
	red, blue := gfx.RGB(220, 38, 38), gfx.RGB(37, 99, 235)
	chosen := red
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Gap(4).Children(func() {
			for _, s := range []struct {
				name string
				col  gfx.Color
			}{{"Red", red}, {"Blue", blue}} {
				if ui.ColorSwatch(c.Key(s.name), s.col, chosen == s.col, s.name).Clicked() {
					chosen = s.col
				}
			}
		})
	})
	d.click("Blue")
	if chosen != blue {
		t.Fatalf("a click chose %v", chosen)
	}
	n := 0
	for _, h := range d.driver.Snapshot().Hits {
		if h.Label == "Blue" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("swatch declares %d hits, want 1", n)
	}
}

func pickerPage(t *testing.T, state *ui.ColorPickerState, color *gfx.Color) *pageDriver {
	return openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() { ui.ColorPicker(c, state, color, "Accent") })
	})
}

func TestColorPickerAppliesATypedHex(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	if !state.Open() {
		t.Fatal("the trigger did not open the panel")
	}
	d.click("Hex color")
	state.TypeHexForTest("#FF0000")
	d.click("OK")
	if state.Open() || !state.Applied() {
		t.Fatalf("Apply left open=%v", state.Open())
	}
	if color != gfx.RGB(255, 0, 0) {
		t.Fatalf("applied %v, want red", color)
	}
}

func TestColorPickerCancelAndEscapeKeepTheColor(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	d.click("Cancel")
	if state.Open() || color != gfx.RGB(10, 20, 30) {
		t.Fatalf("Cancel: open=%v color=%v", state.Open(), color)
	}
	d.click("Accent")
	d.key(gfx.KeyEscape)
	if state.Open() || color != gfx.RGB(10, 20, 30) {
		t.Fatalf("Escape: open=%v color=%v", state.Open(), color)
	}
	d.click("Accent")
	d.click("Clear")
	if color.Alpha != 0 || !state.Applied() {
		t.Fatalf("Clear left %v", color)
	}
}

func TestColorPickerPlaneAndHueMoveTheStagedColor(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(255, 0, 0)
	d := pickerPage(t, &state, &color)
	d.click("Accent")
	plane := d.hit("Saturation and brightness").Bounds
	if err := d.driver.Click(plane.X+plane.Width*0.5, plane.Y+plane.Height*0.5); err != nil {
		t.Fatal(err)
	}
	d.key(gfx.KeyRight)
	d.click("OK")
	if color == gfx.RGB(255, 0, 0) {
		t.Fatal("a press on the plane and an arrow key did not change the staged color")
	}
}

func TestColorPickerDraftFollowsTheDragBeforeApply(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(255, 0, 0)
	d := pickerPage(t, &state, &color)
	if _, ok := state.Draft(); ok {
		t.Fatal("a closed picker has a draft")
	}
	d.click("Accent")
	plane := d.hit("Saturation and brightness").Bounds
	if err := d.driver.Click(plane.X+plane.Width*0.5, plane.Y+plane.Height*0.5); err != nil {
		t.Fatal(err)
	}
	draft, ok := state.Draft()
	if !ok || draft == gfx.RGB(255, 0, 0) {
		t.Fatalf("draft = %v, %v after a press on the plane", draft, ok)
	}
	if color != gfx.RGB(255, 0, 0) {
		t.Fatal("the application's color changed before Apply")
	}
}
