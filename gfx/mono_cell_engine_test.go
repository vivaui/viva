package gfx

import "testing"

// The cell is Ghostty's (src/font/Metrics.zig): the line box rounded to a whole pixel, and
// the font's baseline, half the line gap above it, centered in that cell. The first row is
// JetBrains Maple Mono at 15px, whose 19.8px line box rounds to 20 rows with its baseline
// 16 down; the last two round down, where the row taken away is split above and below it.
func TestMonoCellMetricsFollowTheFontsLineBox(t *testing.T) {
	for _, test := range []struct {
		metrics  MonoCellMetrics
		height   float32
		baseline float32
	}{
		{MonoCellMetrics{Ascent: 15.3, Descent: 4.5}, 20, 16},
		{MonoCellMetrics{Ascent: 13, Descent: 3}, 16, 13},
		{MonoCellMetrics{Ascent: 11.6, Descent: 3.4, LineGap: 0.2}, 15, 11},
		{MonoCellMetrics{Ascent: 13, Descent: 3, LineGap: 0.4}, 16, 13},
	} {
		if got := test.metrics.CellHeight(); got != test.height {
			t.Errorf("%+v: cell height %v, want %v", test.metrics, got, test.height)
		}
		if got := test.metrics.CellBaseline(); got != test.baseline {
			t.Errorf("%+v: cell baseline %v, want %v", test.metrics, got, test.baseline)
		}
	}
}

// A fixed-pitch line carries the row its glyphs stand on, so a renderer that only receives
// the label draws them on the font's baseline; a proportional line carries none.
func TestMonoLinesCarryTheFontsBaseline(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.8, Descent: 0.25, LineGap: 0.08}})
	canvas.LabelRuns("hi", Rect{Width: 22, Height: 23}, TextStyle{Size: 20}, 11, []TextRun{{Bytes: 2, Cells: 2}})
	canvas.Label("plain", Rect{Y: 23, Width: 22, Height: 23}, TextStyle{Size: 20})
	texts := canvas.Texts()
	if texts[0].Baseline != 17 {
		t.Errorf("a grid line stands on row %v of its cell, want 17", texts[0].Baseline)
	}
	if texts[1].Baseline != 0 {
		t.Errorf("a proportional line carries baseline %v, want none", texts[1].Baseline)
	}
}

// monoCellEngine is a TextEngine that measures cells like a font whose advance and line
// box are fractions of the size, and counts how often it was asked.
type monoCellEngine struct {
	referenceEngine
	MonoCellMetrics
	asked      int
	generation uint32
}

func (engine *monoCellEngine) MonoCell(size float32, face MonoFace) MonoCellMetrics {
	engine.asked++
	cell := engine.MonoCellMetrics
	cell.Advance *= size
	cell.Ascent *= size
	cell.Descent *= size
	cell.LineGap *= size
	return cell
}

func (engine *monoCellEngine) SetFontFamily(string)   {}
func (engine *monoCellEngine) FontGeneration() uint32 { return engine.generation }

// The cell of a terminal is the font's: its advance and its line, not the reference
// 0.6 by 1.18, so a font narrower than the reference does not get a looser grid.
func TestMonoCellSizeFollowsTheEnginesFont(t *testing.T) {
	var canvas Canvas
	engine := &monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.80, Descent: 0.25, LineGap: 0.08}}
	canvas.SetTextEngine(engine)
	// 20px: 16 above the baseline, 5 below, 1.6 of line gap, so a line box of 22.6 that the
	// cell rounds to 23, with the baseline 17 rows down.
	width, height := canvas.MonoCellSize(20, MonoSystem)
	if width != 20*0.55 {
		t.Errorf("width = %v, want the font's advance %v", width, 20*0.55)
	}
	if height != 23 {
		t.Errorf("height = %v, want the font's line box 22.6 rounded to 23", height)
	}
	if width >= 20*0.6 {
		t.Errorf("width %v is not narrower than the reference cell", width)
	}
	if baseline := canvas.MonoBaseline(20, MonoSystem, height); baseline != 17 {
		t.Errorf("baseline = %v, want row 17 of the cell", baseline)
	}
}

// A line shorter than its row, as a roomier code editor draws one, centers the font's cell
// in the row, so its glyphs move down with the cell rather than growing apart.
func TestMonoBaselineCentersTheCellInATallerLabel(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.80, Descent: 0.25, LineGap: 0.08}})
	if baseline := canvas.MonoBaseline(20, MonoSystem, 29); baseline != 20 {
		t.Errorf("baseline = %v, want row 20: three rows of cell centered in 29, plus the cell's 17", baseline)
	}
}

// The engine's answer is made whole in device pixels like the reference one.
func TestMonoCellSizeFromAnEngineIsWholeDevicePixels(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.5857, Ascent: 0.9, Descent: 0.3, LineGap: 0.12}})
	width, _ := canvas.MonoCellSize(13, MonoSystem)
	if device := width * 2; device != float32(int(device)) || device < 1 {
		t.Errorf("width %v is %v device pixels, want a whole number", width, device)
	}
}

// Measuring a font is shaping a line: a repeated question must not ask the engine
// again, and two faces asked in turn, as a window showing two of them does every paint,
// must not push each other out. A change of the engine's fonts makes it ask again.
func TestMonoCellSizeAsksTheEngineOncePerFaceAndGeneration(t *testing.T) {
	var canvas Canvas
	engine := &monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.8, Descent: 0.25, LineGap: 0.08}}
	canvas.SetTextEngine(engine)
	for range 5 {
		canvas.MonoCellSize(13, MonoSystem)
		canvas.MonoCellSize(13, MonoClassic)
	}
	if engine.asked != 2 {
		t.Fatalf("the engine was asked %d times for two faces", engine.asked)
	}
	engine.generation++
	engine.Advance = 0.62
	width, _ := canvas.MonoCellSize(13, MonoSystem)
	if engine.asked != 3 || width != 13*0.62 {
		t.Errorf("after the fonts changed: asked %d, width %v, want a new measurement of %v", engine.asked, width, 13*0.62)
	}
}

// An engine that cannot measure a cell, like the reference one, leaves the reference
// cell, and installing an engine drops what an earlier one had answered.
func TestMonoCellSizeFallsBackToTheReferenceCell(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.5, Ascent: 1, Descent: 0.4, LineGap: 0.1}})
	measured, _ := canvas.MonoCellSize(20, MonoSystem)
	canvas.SetTextEngine(ReferenceTextEngine())
	reference, _ := canvas.MonoCellSize(20, MonoSystem)
	if measured == reference || reference != 20*0.6 {
		t.Errorf("measured %v, reference %v: want the reference cell %v after the engine is replaced", measured, reference, 20*0.6)
	}
	canvas.SetTextEngine(&monoCellEngine{})
	if w, h := canvas.MonoCellSize(20, MonoSystem); w != 20*0.6 || h < 20*1.18 {
		t.Errorf("an engine that cannot say gave %v,%v, want the reference cell", w, h)
	}
	if baseline := canvas.MonoBaseline(20, MonoSystem, 24); baseline != 0 {
		t.Errorf("baseline = %v without a measured font, want 0", baseline)
	}
}

func BenchmarkMonoCellSizeFromEngine(b *testing.B) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.8, Descent: 0.25, LineGap: 0.08}})
	canvas.MonoCellSize(13, MonoSystem)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.MonoCellSize(13, MonoSystem)
	}
}
