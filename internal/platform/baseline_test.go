package platform

import (
	_ "embed"
	"regexp"
	"strconv"
	"testing"
)

// The baseline rule reproduces where CoreText has always put the baseline of a
// label in Viva's line box (the size plus seven pixels high), which is the look the
// other backends are brought to.
func TestLabelBaselineIsWhereMacOSAlwaysPutIt(t *testing.T) {
	for size, want := range map[int]int{11: 13, 12: 14, 13: 14, 14: 15, 16: 17, 18: 19, 20: 21} {
		if got := LabelBaseline(size+7, float64(size)); got != want {
			t.Errorf("size %d: baseline row %d, want %d", size, got, want)
		}
	}
	// At 2x the same label has twice the rows, and the baseline is on an even or odd
	// row as the arithmetic says, never between rows.
	if got := LabelBaseline(54, 40); got != 42 {
		t.Errorf("size 20 at 2x: baseline row %d, want 42", got)
	}
}

// A fixed-pitch label stands on the font's own row, which the canvas worked out in logical
// pixels and the renderer rounds to a device pixel; the rest stand on LabelBaseline.
func TestTextBaselineFollowsTheFixedPitchLine(t *testing.T) {
	mono := TextLabel{FontSize: 10, Mono: true, Baseline: 12.5}
	if got := TextBaseline(mono, 20, 2); got != 25 {
		t.Errorf("fixed-pitch baseline row %d at 2x, want 25", got)
	}
	if got := TextBaseline(TextLabel{FontSize: 10}, 20, 2); got != LabelBaseline(20, 20) {
		t.Errorf("proportional baseline row %d, want LabelBaseline's %d", got, LabelBaseline(20, 20))
	}
	if got := TextBaseline(TextLabel{Mono: true, Baseline: 12.5}, 20, 0); got != LabelBaseline(20, 0) {
		t.Errorf("baseline row %d of a label with no font size, want LabelBaseline's %d", got, LabelBaseline(20, 0))
	}
}

//go:embed darwin/native.h
var nativeHeader []byte

func TestNativeHeaderRepeatsTheBaselineConstant(t *testing.T) {
	header := nativeHeader
	match := regexp.MustCompile(`#define VIVA_BASELINE_BELOW_CENTER ([0-9.]+)`).FindSubmatch(header)
	if match == nil {
		t.Fatal("native.h does not define VIVA_BASELINE_BELOW_CENTER")
	}
	if value, _ := strconv.ParseFloat(string(match[1]), 64); value != BaselineBelowCenter {
		t.Errorf("native.h has %v, Go has %v", value, BaselineBelowCenter)
	}
}
