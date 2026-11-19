package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"
)

// The thumb labels and value texts default to the same words the page painted
// before callers could supply them, and a caller's text wins until it is
// cleared.
func TestRangeSliderThumbTextDefaultsAndOverrides(t *testing.T) {
	state := NewRangeSliderState(25, 75)
	if got := state.ThumbLabel(0); got != "Minimum" {
		t.Errorf("low thumb label = %q, want Minimum", got)
	}
	if got := state.ThumbLabel(1); got != "Maximum" {
		t.Errorf("high thumb label = %q, want Maximum", got)
	}
	if got := state.ThumbValueText(0); got != "25" {
		t.Errorf("low thumb value = %q, want 25", got)
	}
	if got := state.ThumbValueText(1); got != "75" {
		t.Errorf("high thumb value = %q, want 75", got)
	}
	state.LowLabel, state.HighLabel = "Low price", "High price"
	state.LowValueText, state.HighValueText = "¥25", "¥75"
	if got := state.ThumbLabel(0); got != "Low price" {
		t.Errorf("low thumb label = %q, want the caller's Low price", got)
	}
	if got := state.ThumbValueText(1); got != "¥75" {
		t.Errorf("high thumb value = %q, want the caller's ¥75", got)
	}
	state.HighValueText = ""
	state.High = 80
	if got := state.ThumbValueText(1); got != "80" {
		t.Errorf("cleared high thumb value = %q, want the value 80", got)
	}
	if got := state.ThumbLabel(2); got != "" {
		t.Errorf("a third thumb has label %q, want none", got)
	}
	if got := state.ThumbValueText(2); got != "" {
		t.Errorf("a third thumb has value %q, want none", got)
	}
}
