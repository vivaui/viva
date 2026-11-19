package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A Disabled ColorSwatch is painted dimmed, declared disabled and inert: the
// generic hit is skipped for a swatch, so the painter must pass the element's
// disabled state to the component that declares the swatch's radio hit.
func TestColorSwatchDisabledIsDimmedAndInert(t *testing.T) {
	red, blue := gfx.RGB(220, 38, 38), gfx.RGB(37, 99, 235)
	chosen := 0
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Gap(4).Children(func() {
			ui.ColorSwatch(c, red, chosen == 0, "Red")
			ui.ColorSwatch(c, blue, chosen == 1, "Blue").Disabled(true)
		})
	})
	hit := d.hit("Blue")
	if !hit.Disabled || hit.Focusable {
		t.Fatalf("disabled swatch hit: Disabled=%v Focusable=%v", hit.Disabled, hit.Focusable)
	}
	d.click("Blue")
	if chosen != 0 {
		t.Fatalf("a disabled swatch chose %d", chosen)
	}
	dimmed := false
	for _, primitive := range d.driver.Snapshot().Primitives {
		c := primitive.Color
		if abs32(c.Red-blue.Red) < .01 && abs32(c.Green-blue.Green) < .01 && abs32(c.Blue-blue.Blue) < .01 &&
			c.Alpha > .35 && c.Alpha < .45 {
			dimmed = true
		}
	}
	if !dimmed {
		t.Fatal("the disabled swatch's color was not painted at the disabled opacity")
	}
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// With ShowAlpha the picker paints the opacity bar and stages and applies
// #RRGGBBAA; the alpha the caller's color had is kept.
func TestColorPickerShowAlphaStagesAndAppliesAlpha(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.ColorPicker(c, &state, &color, "Accent").ShowAlpha(true)
		})
	})
	d.click("Accent")
	if !state.Open() || !state.ShowsAlpha() || !d.has("Opacity") {
		t.Fatalf("open=%v showsAlpha=%v opacity bar=%v", state.Open(), state.ShowsAlpha(), d.has("Opacity"))
	}
	d.click("Hex color")
	state.SetHexText("#FF000080")
	d.click("OK")
	if state.Open() || !state.Applied() {
		t.Fatalf("Apply left open=%v applied=%v", state.Open(), state.Applied())
	}
	if abs32(color.Red-1) > .01 || color.Green != 0 || color.Blue != 0 || abs32(color.Alpha-float32(128)/255) > .01 {
		t.Fatalf("applied %+v, want #FF000080", color)
	}
}

// Without ShowAlpha the picker has no opacity bar and an alpha hex is invalid.
func TestColorPickerWithoutShowAlphaRejectsAlphaHex(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.ColorPicker(c, &state, &color, "Accent")
		})
	})
	d.click("Accent")
	if state.ShowsAlpha() || d.has("Opacity") {
		t.Fatal("a picker not given ShowAlpha offered opacity")
	}
	d.click("Hex color")
	state.SetHexText("#FF000080")
	d.click("OK")
	if !state.Open() {
		t.Fatal("an alpha hex was accepted by a picker without ShowAlpha")
	}
	if color != gfx.RGB(10, 20, 30) {
		t.Fatalf("the rejected input changed the color to %+v", color)
	}
}

// The open alpha panel paints without allocating once it is warm.
func TestOpenAlphaColorPickerPaintsWithoutAllocating(t *testing.T) {
	var state ui.ColorPickerState
	color := gfx.RGB(10, 20, 30)
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			ui.ColorPicker(c, &state, &color, "Accent").ShowAlpha(true)
		})
	})
	d.click("Accent")
	if !state.Open() {
		t.Fatal("the trigger did not open the panel")
	}
	d.repaint()
	if allocs := testing.AllocsPerRun(100, d.repaint); allocs != 0 {
		t.Fatalf("a warm open alpha picker paints with %v allocations, want 0", allocs)
	}
}
