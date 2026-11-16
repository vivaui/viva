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
	// The heaviest face the family ships, which is not the bold the platform picks for
	// weight 700: a family need not ship Black.
	weights := FontWeights("Segoe UI")
	if len(weights) < 2 {
		t.Skipf("Segoe UI lists %v weights", weights)
	}
	heavy := weights[len(weights)-1]
	if heavy == 700 {
		t.Skipf("Segoe UI's heaviest face is bold (%d)", heavy)
	}
	engine.SetFontWeights(400, heavy) // the family's heaviest against its Bold
	heavyWidth := width(gfx.WeightBold)
	if heavyWidth == bold {
		t.Fatalf("bold text is %.3f wide whether its weight is 700 or %d", bold, heavy)
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
	// The weight is the family's own: a stripped image may not install the one the
	// test would name, and the invariant is that some installed weight moves the cell.
	chosen := 0
	for _, weight := range FontWeights("Segoe UI") {
		if weight == 400 {
			continue
		}
		engine.SetMonoWeights(weight, 0)
		if engine.MonoCell(14, face).Advance != regular.Advance {
			chosen = weight
			break
		}
	}
	engine.SetMonoWeights(0, 0)
	if chosen == 0 {
		t.Skip("no installed weight of Segoe UI changes the cell from regular")
	}
	engine.SetMonoWeights(chosen, 0)
	if bold := engine.MonoCell(14, face); bold.Advance == regular.Advance {
		t.Fatalf("the cell is %.3f wide at both regular and weight %d", regular.Advance, chosen)
	}
	engine.SetMonoWeights(0, 0)
	if again := engine.MonoCell(14, face); again.Advance != regular.Advance {
		t.Fatalf("taking the weight away left the cell at %.3f, not %.3f", again.Advance, regular.Advance)
	}
}
