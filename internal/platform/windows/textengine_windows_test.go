package windows

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/textenginetest"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

func newTestEngine(t testing.TB) ui.TextEngine {
	t.Helper()
	engine := TextEngine()
	if engine == nil {
		t.Skip("DirectWrite is not available")
	}
	return engine
}

func TestTextEnginePassesTheContract(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	textenginetest.Run(t, newTestEngine(t))
}

// The original report: a run of digits followed by Chinese text was drawn narrower than
// the same digits alone, because the digits were squeezed into the width the reference
// metrics gave them. A line is one shaped piece now, so the digits are the same pixels.
func TestDigitsBeforeCJKAreDrawnAsTheyAreAlone(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	const width, height = 160, 32
	draw := func(value string) []byte {
		label := platform.TextLabel{Value: value, Width: width, Height: height, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
		pixels, _, done := rasterizeProportional(t, label, 1, width, height, false)
		defer done()
		return append([]byte(nil), pixels...)
	}
	alone, mixed := draw("2213123"), draw("2213123中文")
	inkEnd := 0
	for y := range height {
		for x := range width {
			if alone[(y*width+x)*4+3] != 0 {
				inkEnd = max(inkEnd, x+1)
			}
		}
	}
	if inkEnd == 0 {
		t.Fatal("the digits drew nothing")
	}
	differing := 0
	for y := range height {
		for x := range inkEnd - 1 {
			a, b := alone[(y*width+x)*4+3], mixed[(y*width+x)*4+3]
			if int(a)-int(b) > 4 || int(b)-int(a) > 4 {
				differing++
			}
		}
	}
	if differing != 0 {
		t.Errorf("%d pixels of the digits differ once Chinese text follows them", differing)
	}
	mixedEnd := 0
	for y := range height {
		for x := range width {
			if mixed[(y*width+x)*4+3] != 0 {
				mixedEnd = max(mixedEnd, x+1)
			}
		}
	}
	if mixedEnd <= inkEnd+8 {
		t.Errorf("the Chinese text ends at %d px, the digits at %d px: it was not drawn", mixedEnd, inkEnd)
	}
}

// A mark belongs to its letter and a modifier to its emoji: an offset between them is
// the start of the cluster, as it is for every other platform.
func TestAnOffsetInsideAClusterIsItsStart(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	for _, tc := range []struct {
		value      string
		start, end int // the cluster after the first byte, and where it ends
	}{
		{"xe\u0301y", 1, 4},              // e and a combining acute accent
		{"x\U0001F44D\U0001F3FDy", 1, 9}, // a thumbs up and its skin tone
	} {
		for offset := tc.start + 1; offset < tc.end; offset++ {
			if got, want := engine.Caret(tc.value, 14, 0, offset), engine.Caret(tc.value, 14, 0, tc.start); got != want {
				t.Errorf("%q: caret at %d is %v, at the cluster's start %v", tc.value, offset, got, want)
			}
		}
		if engine.Caret(tc.value, 14, 0, tc.end) <= engine.Caret(tc.value, 14, 0, tc.start) {
			t.Errorf("%q: the cluster has no width", tc.value)
		}
	}
}

// A selection that starts in Latin text and ends in Hebrew is two stretches on the
// screen, not one rectangle.
func TestASelectionAcrossAChangeOfDirectionIsSeveralSpans(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	const value = "abc שלום def"
	spans := engine.Selection(nil, value, 14, 0, 1, 10) // "bc שלו"
	if len(spans) != 2 {
		t.Fatalf("selection spans = %v, want 2", spans)
	}
	if spans[0].X >= spans[1].X {
		t.Errorf("spans are not left to right: %v", spans)
	}
}

// Viva lays text out at its logical size and draws it at the size times the display
// scale: the line has the same width either way, within the rounding of the font.
func TestWidthAtTheDeviceSizeIsTheLogicalWidthScaled(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	for _, value := range []string{"The quick brown fox", "2213123中文", "Import SSH Config"} {
		logical := engine.Width(value, 14, 0)
		for _, scale := range []float32{1.25, 1.5, 2} {
			device := engine.Width(value, 14*scale, 0) / scale
			if math.Abs(float64(device-logical)) > 0.015*float64(logical)+0.5 {
				t.Errorf("%q at %vx: %v px at the device size, %v px at the logical size", value, scale, device, logical)
			}
		}
	}
}

// Nothing drawn lies past the width layout gave the line, whatever the weight.
func TestDrawnLabelStaysInsideItsEngineWidth(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	const width, height = 240, 32
	for _, weight := range []float32{0, 0.15, 0.23, 0.40} {
		for _, value := range []string{"New Host", "Import SSH Config", "Import Catalog", "主机 2213123"} {
			layout := engine.Width(value, 12, weight)
			label := platform.TextLabel{Value: value, FontSize: 12, Weight: weight, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
			pixels, _, done := rasterizeProportional(t, label, 1, width, height, true)
			inkEnd := 0
			for y := range height {
				for x := range width {
					if pixels[(y*width+x)*4+3] != 0 && x+1 > inkEnd {
						inkEnd = x + 1
					}
				}
			}
			done()
			if float32(inkEnd) > layout+1.5 {
				t.Errorf("%q at weight %.2f: ink reaches %d px, layout width is %.2f", value, weight, inkEnd, layout)
			}
		}
	}
}

func BenchmarkTextEngineWidthHit(benchmark *testing.B) {
	engine := newTestEngine(benchmark)
	engine.Width("192.168.1.1 主机 abc", 14, 0)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		engine.Width("192.168.1.1 主机 abc", 14, 0)
	}
}

func BenchmarkTextEngineCaretHit(benchmark *testing.B) {
	engine := newTestEngine(benchmark)
	engine.Caret("192.168.1.1 主机 abc", 14, 0, 5)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		engine.Caret("192.168.1.1 主机 abc", 14, 0, 5)
	}
}

func BenchmarkTextEngineOffsetAtHit(benchmark *testing.B) {
	engine := newTestEngine(benchmark)
	engine.OffsetAt("192.168.1.1 主机 abc", 14, 0, 40)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		engine.OffsetAt("192.168.1.1 主机 abc", 14, 0, 40)
	}
}

// BenchmarkTextEngineWidthMiss shapes a line DirectWrite has not seen: the cost of a
// label the first time it is painted.
func BenchmarkTextEngineWidthMiss(benchmark *testing.B) {
	engine := newTestEngine(benchmark)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for index := range benchmark.N {
		engine.Width("192.168.1.1 主机 abc", 14+float32(index%5000)/1000, 0)
	}
}

func BenchmarkTextEngineCaretMiss(benchmark *testing.B) {
	engine := newTestEngine(benchmark)
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for index := range benchmark.N {
		engine.Caret("192.168.1.1 主机 abc", 14+float32(index%5000)/1000, 0, 5)
	}
}

// The caret, the selection and the click of a native text input stand where the glyphs
// of the shaped line are, also where Chinese text follows digits.
func TestTextInputOverlayFollowsTheShapedLine(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	const value = "2213123中文"
	input := platform.TextInput{X: 10, Y: 20, Width: 200, Height: 30, FontSize: 14, Weight: ui.WeightSemibold, Color: platform.Color{Alpha: 1},
		State: platform.TextState{Value: value, Selection: text.Range{Start: 3, End: 8}}}
	state := windowState{
		textEngine: engine, textInput: input, textState: input.State, textEnabled: true, caretVisible: true,
		baseTextLabelCount: 1, textLabels: make([]platform.TextLabel, 2), textRectangles: make([]render.Rectangle, textRectangleCount),
	}
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	if state.textLabels[1].Weight != input.Weight {
		t.Fatalf("native label weight = %v, want %v", state.textLabels[1].Weight, input.Weight)
	}
	// Runes 3 to 8 are bytes 3 to 10: the digits' tail and the first Chinese character.
	start, end := engine.Caret(value, 14, input.Weight, 3), engine.Caret(value, 14, input.Weight, 10)
	if selection := state.textRectangles[0].Bounds; math.Abs(float64(selection.X-(10+start))) > 0.01 || math.Abs(float64(selection.Width-(end-start))) > 0.01 {
		t.Errorf("selection rectangle = %+v, want x %v width %v", selection, 10+start, end-start)
	}
	if caret := state.textRectangles[1].Bounds; math.Abs(float64(caret.X-(10+end))) > 0.01 {
		t.Errorf("caret x = %v, want %v", caret.X, 10+end)
	}
	boundary := engine.Caret(value, 14, input.Weight, 7) // between the digits and the Chinese text
	event, handled, err := state.textPointerEvent(platform.PointerEvent{Kind: platform.PointerDown, Button: ui.PointerButtonPrimary, X: 10 + boundary + 0.5, Y: 35})
	if err != nil || !handled || event.Selection != (text.Range{Start: 7, End: 7}) {
		t.Errorf("a click beside the boundary placed %+v (handled %v, err %v), want rune 7", event.Selection, handled, err)
	}
}

// A source file of tens of thousands of lines, in a field that paints its own rows,
// reached DirectWrite whole: the engine refuses text of more than 1,024 lines, and
// the label of the native editor, drawn with it, closed the application. The field's
// caret, selection and label are the caret's line now, and are drawn.
func TestTextInputOfAVeryLongDocumentIsLaidOutByTheLine(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine := newTestEngine(t)
	var document strings.Builder
	for line := 0; line < 27094; line++ {
		fmt.Fprintf(&document, "line %d 中文\n", line)
	}
	value := document.String()
	start := strings.Index(value, "line 20000 ")
	caret := utf8.RuneCountInString(value[:start]) + 8
	input := platform.TextInput{X: 10, Y: 20, Width: 400, Height: 30, FontSize: 14, Multiline: true, Placed: true, HideNativeOverlays: true,
		Color: platform.Color{Alpha: 1}, State: platform.TextState{Value: value, Selection: text.Range{Start: caret - 2, End: caret}}}
	state := windowState{
		textEngine: engine, textInput: input, textState: input.State, textEnabled: true, caretVisible: true,
		baseTextLabelCount: 1, textLabels: make([]platform.TextLabel, 2), textRectangles: make([]render.Rectangle, textRectangleCount),
	}
	if err := state.updateTextVisuals(); err != nil {
		t.Fatal(err)
	}
	label := state.textLabels[1]
	if label.Value != "line 20000 中文" {
		t.Fatalf("label = %q, want the caret's line", label.Value)
	}
	if state.layoutLine().value != label.Value {
		t.Fatalf("layout line = %q", state.layoutLine().value)
	}
	if want := engine.Caret(label.Value, 14, 0, len("line 200")); state.textCaret(14, caret) != want || want <= 0 {
		t.Fatalf("caret x = %v, want %v", state.textCaret(14, caret), want)
	}
	pixels, _, done := rasterizeProportional(t, platform.TextLabel{Value: label.Value, Width: 200, Height: 30, FontSize: 14, Color: platform.Color{Alpha: 1}}, 1, 200, 30, false)
	defer done()
	if len(pixels) == 0 {
		t.Fatal("the line was not drawn")
	}
}
