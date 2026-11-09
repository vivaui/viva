package gfx

import "testing"

// halfEngine measures every character as half the size and counts its calls.
type halfEngine struct{ widths, carets, selections, hits int }

type rasterEngine struct {
	halfEngine
	calls int
	value string
}

func (engine *rasterEngine) RasterizeText(value string, style TextStyle, scale float32, width, height int) (Bitmap, error) {
	engine.calls++
	engine.value = value
	pixels := make([]byte, width*height*4)
	if len(pixels) > 3 {
		pixels[3] = 255
	}
	return Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4}, nil
}

func (e *halfEngine) Width(value string, size, weight float32) float32 {
	e.widths++
	return float32(len([]rune(value))) * size / 2
}

func (e *halfEngine) Caret(value string, size, weight float32, offset int) float32 {
	e.carets++
	return float32(len([]rune(value[:offset]))) * size / 2
}

func (e *halfEngine) Selection(dst []TextSpan, value string, size, weight float32, start, end int) []TextSpan {
	e.selections++
	return append(dst, TextSpan{X: float32(start) * size / 2, Width: float32(end-start) * size / 2})
}

func (e *halfEngine) OffsetAt(value string, size, weight, x float32) int {
	e.hits++
	return min(max(int(x/(size/2)+0.5), 0), len(value))
}

func TestCanvasRasterizesThroughItsInstalledTextEngine(t *testing.T) {
	var canvas Canvas
	if _, err := canvas.RasterizeText("hidden", TextStyle{Size: 13}, 2, 120, 28); err != ErrTextRasterizerUnavailable {
		t.Fatalf("RasterizeText without rasterizer = %v, want %v", err, ErrTextRasterizerUnavailable)
	}

	engine := &rasterEngine{}
	canvas.SetTextEngine(engine)
	bitmap, err := canvas.RasterizeText("hidden", TextStyle{Size: 13}, 2, 120, 28)
	if err != nil {
		t.Fatal(err)
	}
	if engine.calls != 1 || bitmap.Width != 120 || bitmap.Height != 28 || bitmap.Stride != 480 || len(bitmap.Pix) != 120*28*4 {
		t.Fatalf("rasterized bitmap = %+v, calls = %d", bitmap, engine.calls)
	}
}

func TestCanvasRasterizeTextCarriesForcedParagraphDirection(t *testing.T) {
	var canvas Canvas
	engine := &rasterEngine{}
	canvas.SetTextEngine(engine)

	if _, err := canvas.RasterizeText("English first", TextStyle{Size: 13, Direction: DirectionRTL}, 1, 120, 28); err != nil {
		t.Fatal(err)
	}
	if engine.value != "\u200fEnglish first" {
		t.Fatalf("RTL raster value = %q, want an RLM-prefixed line", engine.value)
	}

	if _, err := canvas.RasterizeText("עברית", TextStyle{Size: 13, Direction: DirectionLTR}, 1, 120, 28); err != nil {
		t.Fatal(err)
	}
	if engine.value != "\u200eעברית" {
		t.Fatalf("LTR raster value = %q, want an LRM-prefixed line", engine.value)
	}
}

func TestCanvasAsksTheInstalledEngineForEveryTextQuery(t *testing.T) {
	var canvas Canvas
	engine := &halfEngine{}
	canvas.SetTextEngine(engine)

	if got := canvas.TextWidth("abcd", 10); got != 20 {
		t.Errorf("TextWidth = %v, want 20", got)
	}
	if got := canvas.TextCaret("abcd", 10, WeightRegular, 2); got != 10 {
		t.Errorf("TextCaret = %v, want 10", got)
	}
	if got := canvas.TextSelection(nil, "abcd", 10, WeightRegular, 1, 3); len(got) != 1 || got[0] != (TextSpan{X: 5, Width: 10}) {
		t.Errorf("TextSelection = %v", got)
	}
	if got := canvas.TextOffsetAt("abcd", 10, WeightRegular, 16); got != 3 {
		t.Errorf("TextOffsetAt = %v, want 3", got)
	}
	if *engine != (halfEngine{widths: 1, carets: 1, selections: 1, hits: 1}) {
		t.Errorf("calls = %+v, want one of each", *engine)
	}

	canvas.SetTextEngine(nil)
	if got := canvas.TextWidth("abcd", 10); got == 20 {
		t.Error("a canvas with no engine still measures with the removed one")
	}
}

type italicEngine struct {
	halfEngine
	styledWidths, styledCarets, styledSelections, styledHits int
}

func (engine *italicEngine) WidthStyle(value string, size, weight float32, italic bool) float32 {
	engine.styledWidths++
	if italic {
		return float32(len(value)) * size
	}
	return engine.Width(value, size, weight)
}

func (engine *italicEngine) CaretStyle(value string, size, weight float32, italic bool, offset int) float32 {
	engine.styledCarets++
	if italic {
		return float32(offset) * size
	}
	return engine.Caret(value, size, weight, offset)
}

func (engine *italicEngine) SelectionStyle(dst []TextSpan, value string, size, weight float32, italic bool, start, end int) []TextSpan {
	engine.styledSelections++
	if italic {
		return append(dst, TextSpan{X: float32(start) * size, Width: float32(end-start) * size})
	}
	return engine.Selection(dst, value, size, weight, start, end)
}

func (engine *italicEngine) OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int {
	engine.styledHits++
	if italic {
		return min(max(int(x/size+0.5), 0), len(value))
	}
	return engine.OffsetAt(value, size, weight, x)
}

func TestItalicCanvasQueriesUseStyledEngineWithoutChangingLegacyMetrics(t *testing.T) {
	var canvas Canvas
	engine := &italicEngine{}
	canvas.SetTextEngine(engine)
	style := TextStyle{Size: 10, Weight: WeightRegular, Italic: true}
	if got := canvas.TextWidthStyle("abc", style); got != 30 {
		t.Fatalf("italic width = %v", got)
	}
	if got := canvas.TextCaretStyle("abc", style, 2); got != 20 {
		t.Fatalf("italic caret = %v", got)
	}
	if got := canvas.TextSelectionStyle(nil, "abc", style, 1, 3); len(got) != 1 || got[0] != (TextSpan{X: 10, Width: 20}) {
		t.Fatalf("italic selection = %+v", got)
	}
	if got := canvas.TextOffsetAtStyle("abc", style, 16); got != 2 {
		t.Fatalf("italic hit = %v", got)
	}
	canvas.LabelCenter("abc", Rect{Width: 100, Height: 20}, style)
	if got := canvas.Texts()[0].Bounds.X; got != 35 {
		t.Fatalf("italic centered origin = %v", got)
	}
	if got := canvas.TextWidth("abc", 10); got != 15 {
		t.Fatalf("legacy regular width = %v", got)
	}
	if engine.styledWidths != 2 || engine.styledCarets != 1 || engine.styledSelections != 1 || engine.styledHits != 1 {
		t.Fatalf("styled calls = %+v", engine)
	}
}

func TestAnEmbeddedCanvasMeasuresWithTheHostsEngine(t *testing.T) {
	var host, child Canvas
	host.SetTextEngine(&halfEngine{})
	child.BeginEmbedded(&host, testMount(&host, 1))
	if got := child.TextWidth("abcd", 10); got != 20 {
		t.Errorf("embedded TextWidth = %v, want the host engine's 20", got)
	}
}

func TestAMeasurerOverridesTheEngineAndHitTestingFollowsIt(t *testing.T) {
	var canvas Canvas
	engine := &halfEngine{}
	canvas.SetTextEngine(engine)
	// One em per character: the click at 25 is nearest the boundary after 2 characters.
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return float32(len([]rune(value))) * size })
	if got := canvas.TextWidth("abcd", 10); got != 40 {
		t.Errorf("TextWidth = %v, want the measurer's 40", got)
	}
	if got := canvas.TextOffsetAt("abcd", 10, WeightRegular, 25); got != 2 && got != 3 {
		t.Errorf("TextOffsetAt = %v, want the boundary nearest 25", got)
	}
	if got := canvas.TextOffsetAt("日本語", 10, WeightRegular, 21); got != 6 {
		t.Errorf("TextOffsetAt over multibyte text = %v, want the byte offset 6 of the boundary nearest 21", got)
	}
	if engine.widths+engine.hits != 0 {
		t.Errorf("the engine was asked %+v although a measurer is installed", *engine)
	}
}

func TestTextQueriesTakeAnOffsetInsideACharacterToItsStart(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&halfEngine{})
	// "é" is two bytes; offset 1 is inside it.
	if got, want := canvas.TextCaret("é", 10, WeightRegular, 1), canvas.TextCaret("é", 10, WeightRegular, 0); got != want {
		t.Errorf("caret inside a character = %v, at its start = %v", got, want)
	}
}

// The engine is an interface: a buffer handed straight to it would escape, and every
// paint of a field would allocate its selection spans.
func TestAnEngineSelectionDoesNotMakeTheCallersBufferEscape(t *testing.T) {
	var canvas Canvas
	canvas.SetTextEngine(&halfEngine{})
	canvas.TextSelection(nil, "abcd", 10, WeightRegular, 1, 3)
	if allocations := testing.AllocsPerRun(100, func() {
		var spans [4]TextSpan
		canvas.TextSelection(spans[:0], "abcd", 10, WeightRegular, 1, 3)
	}); allocations != 0 {
		t.Errorf("TextSelection allocated %v times per call", allocations)
	}
}
