package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A RangeSlider is a page control with two focusable thumbs; dragging the low
// thumb moves its value and keeps it below the high one.
func TestRangeSliderDragsItsThumbs(t *testing.T) {
	state := ui.NewRangeSliderState(20, 80)
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	})
	low := d.hit("Minimum").Bounds
	x, y := center(low)
	if err := d.driver.Press(x, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Move(x+40, y); err != nil {
		t.Fatal(err)
	}
	if err := d.driver.Release(x+40, y, gfx.PointerButtonPrimary); err != nil {
		t.Fatal(err)
	}
	if state.Low <= 20 || state.Low >= state.High {
		t.Fatalf("low=%v high=%v after dragging", state.Low, state.High)
	}
}
