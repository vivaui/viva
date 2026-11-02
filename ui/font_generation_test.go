package ui

import (
	"testing"
	"unicode/utf8"
)

// factorEngine is a FontFamilyAware engine whose glyphs are factor em wide, so
// changing the factor stands in for naming another family.
type factorEngine struct {
	factor     float32
	generation uint32
}

func (engine *factorEngine) Width(value string, size, weight float32) float32 {
	return float32(utf8.RuneCountInString(value)) * size * engine.factor
}
func (engine *factorEngine) Caret(string, float32, float32, int) float32    { return 0 }
func (engine *factorEngine) OffsetAt(string, float32, float32, float32) int { return 0 }
func (engine *factorEngine) FontGeneration() uint32                         { return engine.generation }
func (engine *factorEngine) SetFontFamily(string)                           {}
func (engine *factorEngine) Selection(dst []TextSpan, _ string, _, _ float32, _, _ int) []TextSpan {
	return dst
}

func (engine *factorEngine) change(factor float32) {
	engine.factor = factor
	engine.generation++
}

func TestCutLinesAreMadeAgainWhenTheFontsChange(t *testing.T) {
	engine := &factorEngine{factor: 0.5}
	var canvas Canvas
	canvas.SetTextEngine(engine)
	line := "a long file name with spaces.tar.gz"
	var cache EllipsisCache
	cut := func() (fit, wrapped string) {
		lines := canvas.WrapLinesCached(nil, line, 10, 0, 60, 1, &cache)
		return canvas.FitLine(line, 10, 0, 60), lines[0]
	}
	narrow, narrowWrapped := cut()
	if narrow == line || narrow != narrowWrapped {
		t.Fatalf("cut at 0.5 em = %q and %q", narrow, narrowWrapped)
	}
	engine.change(0.25)
	wide, wideWrapped := cut()
	if len(wide) <= len(narrow) || len(wideWrapped) <= len(narrow) {
		t.Fatalf("narrower glyphs cut %q and %q, no more than %q: the old cut was kept", wide, wideWrapped, narrow)
	}
	if canvas.FontGeneration() != 1 {
		t.Fatalf("generation %d", canvas.FontGeneration())
	}
}

func TestAnEngineWithoutFontGenerationsHasGeneration0(t *testing.T) {
	var canvas Canvas
	if canvas.FontGeneration() != 0 {
		t.Fatal("a canvas with no engine has a generation")
	}
	canvas.SetTextEngine(ReferenceTextEngine())
	if canvas.FontGeneration() != 0 {
		t.Fatal("the reference engine has a generation")
	}
}
