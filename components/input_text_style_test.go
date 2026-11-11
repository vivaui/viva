package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

// weightedCanvas measures text wider the heavier it is, so a caret or a selection placed with
// the wrong weight lands somewhere else.
func weightedCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len(value)) * size / 2 * (1 + weight/1000)
	})
	return canvas
}

// The style of a field drives everything that depends on its text: the glyphs, the caret, the
// selection, the height of the caret and what the platform is told. One value, so they agree.
func TestInputTextStyleDrivesGlyphsCaretSelectionAndThePlatformGeometry(t *testing.T) {
	theme := gfx.LightTheme()
	ink := gfx.Color{Red: .8, Green: .1, Blue: .1, Alpha: 1}
	style := InputTextStyle{Size: 11, Weight: gfx.WeightSemibold, Color: ink}
	snapshot := InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "abcdef", Focused: true, ShowCaret: true, Caret: 4, SelectionStart: 1, SelectionEnd: 3, Text: style}
	bounds := gfx.Rect{X: 10, Y: 0, Width: 200, Height: 32}

	canvas := weightedCanvas()
	PaintInputContent(canvas, theme, bounds, bounds, snapshot)

	var line gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == "abcdef" {
			line = text
			break
		}
	}
	if line.Style.Size != 11 || line.Style.Weight != gfx.WeightSemibold || line.Style.Color != ink {
		t.Errorf("the glyphs are painted in %+v, want the field's size 11, semibold and its ink", line.Style)
	}
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if text.Size != 11 || text.Weight != gfx.WeightSemibold || text.Color != ink {
		t.Errorf("the platform is told size %v, weight %v, color %+v", text.Size, text.Weight, text.Color)
	}
	// The caret stands where the prefix of that weight ends, not where a regular one would.
	prefix := canvas.TextCaret("abcdef", 11, gfx.WeightSemibold, 4)
	regular := canvas.TextCaret("abcdef", 11, gfx.WeightRegular, 4)
	if prefix == regular {
		t.Fatal("the measurer does not tell weights apart: the test means nothing")
	}
	origin := line.Bounds.X
	var caret gfx.Rectangle
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Color == ink {
			caret = rectangle
		}
	}
	if caret.Bounds.Width != 1 || caret.Bounds.X != origin+prefix {
		t.Errorf("caret at x=%v, want %v (origin %v + the prefix measured in semibold %v, not %v in regular)", caret.Bounds.X, origin+prefix, origin, prefix, regular)
	}
	// The caret is as tall as the text it follows.
	if caret.Bounds.Height != inputCaretHeight(11) {
		t.Errorf("caret height = %v, want %v for 11 point text", caret.Bounds.Height, inputCaretHeight(11))
	}
	// The selection covers the measured width of the selected letters in that weight.
	var spans [4]gfx.TextSpan
	want := canvas.TextSelection(spans[:0], "abcdef", 11, gfx.WeightSemibold, 1, 3)
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if len(want) == 1 && rectangle.Bounds.X == origin+want[0].X && rectangle.Bounds.Width == want[0].Width {
			found = true
		}
	}
	if !found {
		t.Errorf("no selection over the semibold width %+v among %+v", want, canvas.Rectangles())
	}
}

// A field with no style is the shadcn field: the theme's size, regular weight and foreground.
func TestInputWithoutATextStyleIsTheThemesText(t *testing.T) {
	theme := gfx.LightTheme()
	canvas := weightedCanvas()
	bounds := gfx.Rect{Width: 200, Height: 32}
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "abc"})
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if text.Size != theme.Metrics.FontSizeM || text.Weight != gfx.WeightRegular || text.Color != theme.Foreground {
		t.Errorf("the platform is told size %v, weight %v, color %+v", text.Size, text.Weight, text.Color)
	}
}

// The placeholder is muted whatever ink the text has, and still takes the field's size and weight.
func TestInputPlaceholderIsMutedAndTakesTheFieldsSizeAndWeight(t *testing.T) {
	theme := gfx.LightTheme()
	canvas := weightedCanvas()
	bounds := gfx.Rect{Width: 200, Height: 32}
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "x", Placeholder: "type here", Text: InputTextStyle{Size: 12, Weight: gfx.WeightMedium, Color: gfx.Color{Red: 1, Alpha: 1}},
	})
	for _, text := range canvas.Texts() {
		if text.Value != "type here" {
			continue
		}
		if text.Style.Color != theme.MutedForeground || text.Style.Size != 12 || text.Style.Weight != gfx.WeightMedium {
			t.Errorf("placeholder style = %+v", text.Style)
		}
		return
	}
	t.Fatal("no placeholder painted")
}

// monoCanvas has a fixed-pitch cell of ten pixels at size 16, scaling with the size.
func monoCanvas() (*gfx.Canvas, func(size float32) float32) {
	canvas := weightedCanvas()
	cell := func(size float32) float32 { return size * 0.625 }
	canvas.SetCellMeasurer(func(size float32, face gfx.MonoFace) (float32, float32) { return cell(size), size * 1.25 })
	return canvas, cell
}

// A mono field is painted by the component whatever the snapshot says of the platform, since the
// platform's text service draws one proportional face: the glyphs are drawn in the mono face, the
// caret stands on a cell boundary, the selection spans whole cells, and the platform is kept for
// input with its own text invisible and its own caret and selection hidden.
func TestMonoInputIsPaintedByTheComponentAndTheNativeLayerIsHidden(t *testing.T) {
	theme := gfx.LightTheme()
	canvas, cell := monoCanvas()
	style := InputTextStyle{Size: 16, Weight: gfx.WeightMedium, Mono: true, MonoFace: gfx.MonoClassic}
	snapshot := InputSnapshot{
		ID: gfx.Item(1), Label: "type", Value: "int4", Focused: true, PlatformEditing: true,
		Caret: 3, SelectionStart: 1, SelectionEnd: 3, Text: style,
	}
	bounds := gfx.Rect{X: 10, Y: 0, Width: 200, Height: 32}
	PaintInputContent(canvas, theme, bounds, bounds, snapshot)

	var line gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == "int4" {
			line = text
		}
	}
	if !line.Style.Mono || line.Style.MonoFace != gfx.MonoClassic || line.Style.Weight != gfx.WeightMedium {
		t.Fatalf("the glyphs were not painted by the component in the mono face: %+v (PlatformEditing must not matter)", line.Style)
	}
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if !hit.HideNativeOverlays || text.Color != (gfx.Color{}) {
		t.Errorf("the platform's text and overlays are not hidden: hide=%v ink=%+v", hit.HideNativeOverlays, text.Color)
	}
	x := line.Bounds.X
	var caret, selection gfx.Rectangle
	for _, rectangle := range canvas.Rectangles() {
		switch {
		case rectangle.Bounds.Width == 1:
			caret = rectangle
		case rectangle.Bounds.Width > 1:
			selection = rectangle
		}
	}
	if want := x + 3*cell(16); caret.Bounds.Width != 1 || caret.Bounds.X != want {
		t.Errorf("caret at x=%v, want three cells in: %v", caret.Bounds.X, want)
	}
	if selection.Bounds.X != x+cell(16) || selection.Bounds.Width != 2*cell(16) {
		t.Errorf("selection %+v, want two cells starting at one cell in", selection.Bounds)
	}
}

// The caret of a self-painted field is drawn only while its blink phase has it lit.
func TestMonoInputCaretFollowsItsBlinkPhase(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{Width: 200, Height: 32}
	caretsOf := func(blink *CaretBlink, now time.Duration) int {
		canvas, _ := monoCanvas()
		canvas.SetClock(func() time.Duration { return now })
		PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
			ID: gfx.Item(1), Label: "x", Value: "ab", Focused: true, Caret: 1, Blink: blink,
			Text: InputTextStyle{Size: 14, Mono: true},
		})
		n := 0
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.Width == 1 {
				n++
			}
		}
		return n
	}
	var blink CaretBlink
	if got := caretsOf(&blink, 0); got != 1 {
		t.Fatalf("the first frame paints %d carets, want it lit", got)
	}
	if got := caretsOf(&blink, CaretBlinkHalf+10*time.Millisecond); got != 0 {
		t.Errorf("the second half of the period paints %d carets, want it dark", got)
	}
	if got := caretsOf(nil, CaretBlinkHalf+10*time.Millisecond); got != 1 {
		t.Errorf("a nil Blink paints %d carets, want it always lit", got)
	}
}

// A mono placeholder that does not fit ends in an ellipsis at a whole cell.
func TestMonoInputPlaceholderIsCutAtACell(t *testing.T) {
	theme := gfx.LightTheme()
	canvas, cell := monoCanvas()
	bounds := gfx.Rect{Width: 5*cell(16) + 2, Height: 32}
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "x", Placeholder: "abcdefghij", Text: InputTextStyle{Size: 16, Mono: true},
	})
	for _, text := range canvas.Texts() {
		if text.Style.Mono {
			if text.Value != "abcd…" {
				t.Errorf("placeholder = %q, want %q (five cells, the last the ellipsis)", text.Value, "abcd…")
			}
			return
		}
	}
	t.Fatal("no placeholder painted")
}

// A field that is not mono is untouched: the platform keeps drawing its text.
func TestProportionalInputKeepsThePlatformsOverlays(t *testing.T) {
	canvas := weightedCanvas()
	bounds := gfx.Rect{Width: 200, Height: 32}
	PaintInputContent(canvas, gfx.LightTheme(), bounds, bounds, InputSnapshot{ID: gfx.Item(1), Label: "x", Value: "abc", Focused: true, PlatformEditing: true})
	hits := canvas.Hits()
	hit := hits[0]
	text := hitText(canvas, &hit)
	if hit.HideNativeOverlays || text.Color == (gfx.Color{}) {
		t.Errorf("a proportional field hid the platform's text: hide=%v ink=%+v", hit.HideNativeOverlays, text.Color)
	}
	if len(canvas.Texts()) != 0 {
		t.Errorf("the component painted text the platform draws: %+v", canvas.Texts())
	}
}
