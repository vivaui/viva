package windows

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A cell carries the thicknesses the face draws its own box drawing lines with
// (gfx.MonoCellMetrics.Rule and HeavyRule), so a border drawn as geometry weighs what the
// font's own glyphs weigh. DirectWrite reports each glyph's bearings in design units; the
// engine scales the ink between them to the logical size asked.
func TestTheEngineReportsTheFontsBoxDrawingStrokes(t *testing.T) {
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite text engine: %v", err)
	}
	reported := 0
	for _, name := range []string{"Consolas", "Cascadia Mono", "Courier New"} {
		face := gfx.MonoFamily(name)
		cell := engine.MonoCell(13, face)
		if cell.Advance <= 0 {
			continue // the runner does not have this family
		}
		if cell.Rule <= 0 || cell.Rule >= cell.Advance {
			t.Errorf("%s: light line %v of a cell %v wide, want a positive stroke narrower than the cell", name, cell.Rule, cell.Advance)
			continue
		}
		if cell.HeavyRule > 0 && cell.HeavyRule < cell.Rule {
			t.Errorf("%s: heavy line %v is thinner than the light one %v", name, cell.HeavyRule, cell.Rule)
		}
		reported++
		at26 := engine.MonoCell(26, face)
		if at26.Rule <= cell.Rule {
			t.Errorf("%s: light line %v at size 26, %v at size 13, want it to follow the size", name, at26.Rule, cell.Rule)
		}
		if cell.HeavyRule > 0 && at26.HeavyRule <= cell.HeavyRule {
			t.Errorf("%s: heavy line %v at size 26, %v at size 13, want it to follow the size", name, at26.HeavyRule, cell.HeavyRule)
		}
	}
	if reported == 0 {
		t.Fatal("no installed fixed-pitch face reported its box drawing strokes")
	}
}
