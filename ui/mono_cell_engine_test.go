package ui

import "testing"

// monoCellEngine is a TextEngine that measures cells like a font whose advance and line
// box are fractions of the size, and counts how often it was asked.
type monoCellEngine struct {
	referenceEngine
	advance, line float32
	asked         int
	generation    uint32
}

func (engine *monoCellEngine) MonoCell(size float32, face MonoFace) (float32, float32) {
	engine.asked++
	return size * engine.advance, size * engine.line
}

func (engine *monoCellEngine) SetFontFamily(string)   {}
func (engine *monoCellEngine) FontGeneration() uint32 { return engine.generation }

// The cell of a terminal is the font's: its advance and its line, not the reference
// 0.6 by 1.18, so a font narrower than the reference does not get a looser grid.
func TestMonoCellSizeFollowsTheEnginesFont(t *testing.T) {
	var canvas Canvas
	engine := &monoCellEngine{advance: 0.55, line: 1.17}
	canvas.SetTextEngine(engine)
	width, height := canvas.MonoCellSize(20, MonoSystem)
	if width != 20*0.55 {
		t.Errorf("width = %v, want the font's advance %v", width, 20*0.55)
	}
	if height < 20*1.17 || height >= 20*1.17+1 {
		t.Errorf("height = %v, want the font's line %v rounded up to a whole pixel", height, 20*1.17)
	}
	if width >= 20*0.6 {
		t.Errorf("width %v is not narrower than the reference cell", width)
	}
}

// The engine's answer is made whole in device pixels like the reference one.
func TestMonoCellSizeFromAnEngineIsWholeDevicePixels(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	canvas.SetTextEngine(&monoCellEngine{advance: 0.5857, line: 1.32})
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
	engine := &monoCellEngine{advance: 0.55, line: 1.17}
	canvas.SetTextEngine(engine)
	for range 5 {
		canvas.MonoCellSize(13, MonoSystem)
		canvas.MonoCellSize(13, MonoClassic)
	}
	if engine.asked != 2 {
		t.Fatalf("the engine was asked %d times for two faces", engine.asked)
	}
	engine.generation++
	engine.advance = 0.62
	width, _ := canvas.MonoCellSize(13, MonoSystem)
	if engine.asked != 3 || width != 13*0.62 {
		t.Errorf("after the fonts changed: asked %d, width %v, want a new measurement of %v", engine.asked, width, 13*0.62)
	}
}

// An engine that cannot measure a cell, like the reference one, leaves the reference
// cell, and installing an engine drops what an earlier one had answered.
func TestMonoCellSizeFallsBackToTheReferenceCell(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&monoCellEngine{advance: 0.5, line: 1.5})
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
}

func BenchmarkMonoCellSizeFromEngine(b *testing.B) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	canvas.SetTextEngine(&monoCellEngine{advance: 0.55, line: 1.17})
	canvas.MonoCellSize(13, MonoSystem)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		canvas.MonoCellSize(13, MonoSystem)
	}
}
