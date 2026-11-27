package components

import (
	"testing"

	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
)

// fixedWidth measures every rune one cell wide, so a test can say where a caret of a
// long value must land without depending on a font.
func fixedWidth(cell float32) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		return float32(len([]rune(value))) * cell
	})
	return canvas
}

// caretRect is the caret the field painted: the ink rectangle one pixel wide.
func caretRect(canvas *gfx.Canvas, ink gfx.Color) (gfx.Rectangle, bool) {
	var caret gfx.Rectangle
	found := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Width == 1 && rectangle.Color == ink {
			caret, found = rectangle, true
		}
	}
	return caret, found
}

// A single-line field that paints its own text must scroll it, as a native one does:
// before this, a value longer than the field kept its start in view however far the
// caret moved, so everything typed past the edge was invisible.
func TestInputScrollsItsOwnLineToKeepTheCaretInView(t *testing.T) {
	const cell = 10
	theme := gfx.LightTheme()
	ink := theme.Foreground
	bounds := gfx.Rect{X: 0, Y: 0, Width: 100, Height: 32}
	value := "0123456789abcdefghij" // 20 cells, twice the field
	scroll := new(base.InputScroll)
	caret := len(value)

	// The caret at the end of the value: the tail must be in view.
	canvas := fixedWidth(cell)
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "Filter", Value: value, Focused: true, ShowCaret: true,
		Caret: caret, HideEditorText: true, Scroll: scroll,
	})
	painted, ok := caretRect(canvas, ink)
	if !ok {
		t.Fatal("the field painted no caret")
	}
	if right := painted.Bounds.X + painted.Bounds.Width; right > bounds.X+bounds.Width {
		t.Fatalf("the caret at the end of a long value is painted at %v, past the field's right edge %v", painted.Bounds.X, bounds.X+bounds.Width)
	}
	if painted.Bounds.X < bounds.X {
		t.Fatalf("the caret at the end is painted at %v, left of the field", painted.Bounds.X)
	}

	// The line itself moved: the glyphs are no longer drawn from the field's start.
	var label gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == value {
			label = text
			break
		}
	}
	if label.Bounds.X >= bounds.X {
		t.Fatalf("the glyphs still start at %v: a long value did not scroll", label.Bounds.X)
	}

	// Home comes back: the caret at the start puts the line back at the field's start.
	canvas = fixedWidth(cell)
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "Filter", Value: value, Focused: true, ShowCaret: true,
		Caret: 0, HideEditorText: true, Scroll: scroll,
	})
	painted, ok = caretRect(canvas, ink)
	if !ok {
		t.Fatal("the field painted no caret after Home")
	}
	if painted.Bounds.X != bounds.X {
		t.Fatalf("the caret at the start is painted at %v, want the field's start %v", painted.Bounds.X, bounds.X)
	}
	if got := scroll.Offset(); got != 0 {
		t.Fatalf("the line is scrolled %v with the caret at the start, want 0", got)
	}
}

// The scroll follows the caret only as far as it must: a caret still inside the field
// leaves the text where it is, so typing at the end does not slide the line under a
// caret that has not reached the edge.
func TestInputScrollLeavesTheLineInPlaceWhileTheCaretFits(t *testing.T) {
	const cell = 10
	scroll := new(base.InputScroll)
	width := float32(100)
	// A caret at 40 of a 50-wide value, in a 100-wide field: everything fits.
	if got := scroll.Follow(40, 50, width); got != 0 {
		t.Fatalf("a value that fits is scrolled by %v, want 0", got)
	}
	// The caret reaches the right edge: the text moves just enough to show it.
	if got := scroll.Follow(140, 150, width); got != 140-99 {
		t.Fatalf("a caret at the edge leaves the text scrolled by %v, want %v", got, 140-99)
	}
	// A shorter field start brings the line back: the offset is kept in range.
	if got := scroll.Follow(0, 150, width); got != 0 {
		t.Fatalf("the caret at the start leaves the text scrolled by %v, want 0", got)
	}
	// A value that no longer overflows pulls the line back too.
	if got := scroll.Follow(10, 20, width); got != 0 {
		t.Fatalf("a value that fits again leaves the text scrolled by %v, want 0", got)
	}
}

// A field the platform editor paints scrolls its own line: moving the glyphs under it as
// well would scroll the caret twice.
func TestInputPlatformPaintedLineIsNotScrolled(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 0, Y: 0, Width: 100, Height: 32}
	value := "0123456789abcdefghij"
	scroll := new(base.InputScroll)
	canvas := fixedWidth(10)
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "Filter", Value: value, Focused: true, ShowCaret: true,
		Caret: len(value), PlatformEditing: true, Scroll: scroll,
	})
	if got := scroll.Offset(); got != 0 {
		t.Fatalf("a platform-painted field scrolled its line by %v, want 0", got)
	}
	var label gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == value {
			label = text
		}
	}
	if label.Bounds.X != bounds.X {
		t.Fatalf("the platform's line starts at %v, want the field's start %v", label.Bounds.X, bounds.X)
	}
}

// The scroll the component paints with is told to the platform, so a press lands where
// the glyphs are and the input method's window follows the caret.
func TestInputScrollIsDeclaredToThePlatform(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 0, Y: 0, Width: 100, Height: 32}
	value := "0123456789abcdefghij"
	scroll := new(base.InputScroll)
	canvas := fixedWidth(10)
	PaintInputContent(canvas, theme, bounds, bounds, InputSnapshot{
		ID: gfx.Item(1), Label: "Filter", Value: value, Focused: true, ShowCaret: true,
		Caret: len(value), HideEditorText: true, Scroll: scroll,
	})
	hits := canvas.Hits()
	if len(hits) != 1 {
		t.Fatalf("hits = %d, want 1", len(hits))
	}
	text := hitTextAt(canvas, 0)
	if want := float32(len(value)*10 - 99); text.Scroll != want {
		t.Fatalf("the platform is told the line is scrolled by %v, want %v", text.Scroll, want)
	}
	if text.Bounds.X != bounds.X || text.Bounds.Width != bounds.Width {
		t.Fatalf("the platform's text box is %+v, want the field's own %+v", text.Bounds, bounds)
	}
}

func TestInputScrollOffsetIsBoundedByTheLine(t *testing.T) {
	scroll := new(base.InputScroll)
	// A negative or zero width paints nothing and scrolls nothing.
	if got := scroll.Follow(500, 900, 0); got != 0 {
		t.Fatalf("a field with no width scrolls by %v, want 0", got)
	}
	// The line is never scrolled past its own end.
	got := scroll.Follow(1000, 120, 100)
	if most := float32(120 - 99); got != most {
		t.Fatalf("the line scrolls by %v, want its most %v", got, most)
	}
}
