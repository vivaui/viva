package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A page RangeSlider announces the caller's localized thumb labels and value
// texts, as the canvas-level preview does.
func TestRangeSliderPaintsCallerThumbLabelsAndValueText(t *testing.T) {
	state := ui.NewRangeSliderState(25, 80)
	state.LowLabel = "Minimum price"
	state.HighLabel = "Maximum price"
	state.LowValueText = "¥25"
	state.HighValueText = "¥80"
	d := openPage(t, func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	})
	if low := d.hit("Minimum price"); low.Value != "¥25" {
		t.Errorf("low thumb value is %q, want ¥25", low.Value)
	}
	if high := d.hit("Maximum price"); high.Value != "¥80" {
		t.Errorf("high thumb value is %q, want ¥80", high.Value)
	}
}

// A warm range slider paints without allocating: the thumb labels and the
// cached default value texts are read, not rebuilt.
func TestRangeSliderPaintsWithoutAllocatingOnceWarm(t *testing.T) {
	state := ui.NewRangeSliderState(25, 80)
	state.LowLabel = "Minimum price"
	state.HighLabel = "Maximum price"
	page := ui.NewPage(ui.PageConfig{Theme: &pageTheme})
	canvas := benchCanvas()
	view := func(c ui.C) {
		ui.Row(c).Children(func() {
			ui.RangeSlider(c, state).Width(240)
		})
	}
	draw := func() { drawPage(page, canvas, view) }
	draw()
	if allocs := testing.AllocsPerRun(100, draw); allocs != 0 {
		t.Fatalf("a warm range slider allocates %v times a frame, want 0", allocs)
	}
}
