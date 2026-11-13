package windows

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The cell of a fixed-pitch face carries the thickness the font underlines with: the rule
// a cell glyph draws with (gfx.MonoCellMetrics.Underline), so a box drawing border drawn
// as geometry weighs what the font's own glyphs weigh. DirectWrite reports the thickness
// in design units; the engine scales it to the logical size asked.
func TestTheEngineReportsTheFontsUnderline(t *testing.T) {
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
		if cell.Underline <= 0 || cell.Underline >= cell.Advance {
			t.Errorf("%s: underline %v of a cell %v wide, want a positive stroke narrower than the cell", name, cell.Underline, cell.Advance)
			continue
		}
		reported++
		if at26 := engine.MonoCell(26, face); at26.Underline <= cell.Underline {
			t.Errorf("%s: underline %v at size 26, %v at size 13, want it to follow the size", name, at26.Underline, cell.Underline)
		}
	}
	if reported == 0 {
		t.Fatal("no installed fixed-pitch face reported its underline thickness")
	}
}
