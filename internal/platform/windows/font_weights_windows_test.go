package windows

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A family's weights are what the settings page offers: the weights it is installed
// with, ascending and without repeats, and none for a family the system does not have.
func TestFontWeightsListsTheFamiliesOwnWeights(t *testing.T) {
	weights := FontWeights("Segoe UI")
	if len(weights) < 2 {
		t.Fatalf("Segoe UI lists %v, fewer weights than it ships", weights)
	}
	for index, weight := range weights {
		if weight <= 0 || weight > 999 {
			t.Errorf("weight %d is outside DirectWrite's range", weight)
		}
		if index > 0 && weights[index-1] >= weight {
			t.Errorf("weights are not strictly ascending: %v", weights)
		}
	}
	if got := FontWeights("No such family, certainly not installed"); len(got) != 0 {
		t.Errorf("a missing family lists %v", got)
	}
}

// The weights reach the shaped text: the bold band moves to the weight named, the
// main band to its own, and a weight named twice shapes the same line again.
func TestTheInterfaceWeightsShapeTheText(t *testing.T) {
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	engine.SetFontFamily("Segoe UI")
	t.Cleanup(func() {
		engine.SetFontFamily("")
		engine.SetFontWeights(0, 0)
	})
	width := func(weight float32) float32 {
		line, err := engine.shapeStyle("WWWWWWWWWW", 14, weight, false)
		if err != nil {
			t.Fatalf("shape: %v", err)
		}
		return line.width
	}
	engine.SetFontWeights(0, 0)
	regular, bold := width(gfx.WeightRegular), width(gfx.WeightBold)
	engine.SetFontWeights(400, 900) // Segoe UI Black against its Bold
	black := width(gfx.WeightBold)
	if black == bold {
		t.Fatalf("bold text is %.3f wide whether its weight is 700 or 900", bold)
	}
	if got := width(gfx.WeightRegular); got != regular {
		t.Fatalf("naming the regular weight again changed the main text: %.3f to %.3f", regular, got)
	}
	if got := width(gfx.WeightMedium); got != regular {
		t.Fatalf("medium text is drawn at %.3f, not the main weight's %.3f", got, regular)
	}
}

// The fixed-pitch weights are the terminal's own: the cell it is laid out on is
// measured in the weight it draws with, so a family whose faces differ in advance
// changes the cell when the weight changes, and back when it is taken away.
func TestTheMonoWeightsChooseTheCell(t *testing.T) {
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	t.Cleanup(func() { engine.SetMonoWeights(0, 0) })
	// A proportional family used as a grid face: its regular and bold faces differ
	// in advance, where a fixed-pitch family's would not.
	face := gfx.MonoFamily("Segoe UI")
	engine.SetMonoWeights(0, 0)
	regular := engine.MonoCell(14, face)
	engine.SetMonoWeights(700, 0)
	bold := engine.MonoCell(14, face)
	if bold.Advance == regular.Advance {
		t.Fatalf("the cell is %.3f wide at both regular and bold weights", regular.Advance)
	}
	engine.SetMonoWeights(0, 0)
	if again := engine.MonoCell(14, face); again.Advance != regular.Advance {
		t.Fatalf("taking the weight away left the cell at %.3f, not %.3f", again.Advance, regular.Advance)
	}
}
