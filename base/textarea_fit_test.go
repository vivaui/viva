package base

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

const fitTestSize, fitTestWidth, fitTestLine = 13, 180, 20

// shapedCanvas measures unlike the reference metrics, as a platform text engine does,
// and records what it measured.
func shapedCanvas(measured *[]string) *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		if measured != nil {
			*measured = append(*measured, value)
		}
		return float32(len(value)) * size * 0.45
	})
	return canvas
}

// paragraphs splits text at its newlines into the starts of its paragraphs.
func paragraphStarts(text string) []int {
	starts := []int{0}
	for at, c := range []byte(text) {
		if c == '\n' {
			starts = append(starts, at+1)
		}
	}
	return starts
}

// checkRows fails unless every paragraph's rows are the ones the canvas wraps it to
// when they are marked exact, and the estimate of them when they are not.
func checkRows(t *testing.T, canvas *gfx.Canvas, layout *TextareaLayout, text string, wrap bool) {
	t.Helper()
	width := float32(fitTestWidth)
	if !wrap {
		width = 0
	}
	if len(layout.exact) != len(layout.Rows) {
		t.Fatalf("%d rows but %d exactness marks", len(layout.Rows), len(layout.exact))
	}
	estimated := 0
	row := 0
	for _, start := range paragraphStarts(text) {
		if row >= len(layout.Rows) || layout.Rows[row].Start != start {
			t.Fatalf("no row starts paragraph %d of %q: %v", start, text, layout.Rows)
		}
		exact := layout.exact[row]
		var want []gfx.TextRow
		for at := start; ; {
			var r gfx.TextRow
			if exact {
				r, at = canvas.WrapRow(text, at, fitTestSize, gfx.WeightRegular, width)
			} else {
				r, at = gfx.WrapRowEstimate(text, at, fitTestSize, gfx.WeightRegular, width)
			}
			want = append(want, r)
			if at < 0 || text[at-1] == '\n' {
				break
			}
		}
		got := layout.Rows[row : row+len(want)]
		if !slices.Equal(got, want) {
			t.Fatalf("paragraph at %d (exact %v) of %q:\n got %v\nwant %v", start, exact, text, got, want)
		}
		for index := row; index < row+len(want); index++ {
			if layout.exact[index] != exact {
				t.Fatalf("paragraph at %d is partly exact", start)
			}
			if !exact {
				estimated++
			}
		}
		row += len(want)
	}
	if row != len(layout.Rows) {
		t.Fatalf("%d rows past the last paragraph of %q: %v exact %v", len(layout.Rows)-row, text, layout.Rows, layout.exact)
	}
	if estimated != layout.estimated {
		t.Fatalf("%d estimated rows counted as %d", estimated, layout.estimated)
	}
}

// However the text was edited, scrolled and shown, every paragraph is laid out either as
// the canvas wraps it or as the estimate does, and counting the rows makes them all the
// canvas's.
func TestTextareaLayoutKeepsEveryParagraphEstimatedOrExact(t *testing.T) {
	pieces := []string{"a", "word ", "\n", "\r\n", "  ", "中文", "the quick brown fox ", "averyveryverylongwordthatmustbreak", "é", "\n\n"}
	random := rand.New(rand.NewPCG(1, 2))
	piece := func() string {
		var b strings.Builder
		for range random.IntN(8) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		return b.String()
	}
	for _, wrap := range []bool{true, false} {
		canvas := shapedCanvas(nil)
		var layout TextareaLayout
		text := ""
		for step := range 3000 {
			at := random.IntN(len(text) + 1)
			cut := at + random.IntN(len(text)-at+1)
			if random.IntN(3) == 0 {
				cut = at // a pure insertion
			}
			switch random.IntN(20) {
			case 0:
				at, cut = 0, len(text) // everything replaced
			case 1:
				at, cut = len(text), len(text) // appended
			}
			text = text[:at] + piece() + text[cut:]
			layout.Fit(canvas, text, fitTestSize, fitTestWidth, wrap)
			layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 3 * fitTestLine}, fitTestLine
			if random.IntN(2) == 0 {
				layout.Scroll = layout.clampScroll(float32(random.IntN(len(layout.Rows)+1)) * fitTestLine)
				layout.Show(random.IntN(len(text) + 1))
			}
			checkRows(t, canvas, &layout, text, wrap)
			if step%500 == 0 {
				rows := layout.RowCount(canvas, gfx.LightTheme(), text, fitTestWidth+2*gfx.LightTheme().Metrics.TextInsetX(), !wrap)
				width := float32(fitTestWidth)
				if !wrap {
					width = 0
				}
				// RowCount lays the text out in the theme's size.
				if want := canvas.WrapRows(nil, text, gfx.LightTheme().Metrics.FontSizeM, gfx.WeightRegular, width); rows != len(want) || !slices.Equal(layout.Rows, want) {
					t.Fatalf("step %d: counted rows are not the canvas's", step)
				}
			}
		}
	}
}

func longText(lines int) string {
	var b strings.Builder
	for i := range lines {
		fmt.Fprintf(&b, "line %d the quick brown fox jumps over the lazy dog\n", i)
	}
	return b.String()
}

// fromParagraphs fails unless every measured value is part of one of paragraphs.
func fromParagraphs(t *testing.T, what string, measured []string, paragraphs ...string) {
	t.Helper()
	if len(measured) == 0 {
		t.Fatalf("%s: nothing was measured", what)
	}
	for _, value := range measured {
		value = strings.TrimRight(value, " ")
		if !slices.ContainsFunc(paragraphs, func(p string) bool { return strings.Contains(p, value) }) {
			t.Fatalf("%s: measured %q, which is in none of %q", what, value, paragraphs)
		}
	}
}

// Opening a long text shapes the rows the box shows, not every row of the text.
func TestTextareaLayoutShapesOnlyTheRowsInView(t *testing.T) {
	var measured []string
	canvas := shapedCanvas(&measured)
	text := longText(20000)
	var layout TextareaLayout
	layout.Fit(canvas, text, fitTestSize, fitTestWidth, true)
	layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 3 * fitTestLine}, fitTestLine
	layout.Show(0)
	lines := strings.Split(text, "\n")
	fromParagraphs(t, "at the start", measured, lines[:4]...)

	// At the end, which a caret there scrolls to.
	measured = measured[:0]
	layout.Show(len(text))
	if caret := layout.RowOf(len(text)); float32(caret)*fitTestLine < layout.Scroll || float32(caret+1)*fitTestLine > layout.Scroll+layout.Content.Height {
		t.Fatalf("the caret's row %d is out of view at scroll %v", caret, layout.Scroll)
	}
	fromParagraphs(t, "at the end", measured, lines[len(lines)-5:]...)
}

// Typing in, or appending to, a long text shapes only the paragraph it changed.
func TestTextareaLayoutShapesOnlyTheEditedParagraph(t *testing.T) {
	var measured []string
	canvas := shapedCanvas(&measured)
	text := longText(2000)
	middle := strings.Index(text, "line 1000 ")
	for _, edit := range []struct {
		name, text, paragraph string
		caret                 int
	}{
		{"appended", text + "a new line at the end", "a new line at the end", len(text) + 21},
		{"typed", text[:middle+10] + "x" + text[middle+10:], "line 1000 xthe quick brown fox jumps over the lazy dog", middle + 11},
	} {
		var layout TextareaLayout
		layout.Fit(canvas, text, fitTestSize, fitTestWidth, true)
		layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 3 * fitTestLine}, fitTestLine
		layout.Show(edit.caret - 1)
		measured = measured[:0]
		layout.Fit(canvas, edit.text, fitTestSize, fitTestWidth, true)
		layout.Show(edit.caret)
		fromParagraphs(t, edit.name, measured, edit.paragraph)
		layout.text = edit.text
		checkRows(t, canvas, &layout, edit.text, true)
	}
}

// A box scrolled away from its caret stays where it was as the rows it shows are
// wrapped with the canvas: the text at its top does not move, and it is not taken back
// to the caret.
func TestTextareaLayoutKeepsItsTopTextAsRowsAreShaped(t *testing.T) {
	canvas := shapedCanvas(nil)
	var b strings.Builder
	for i := range 400 {
		fmt.Fprintf(&b, "paragraph %d %s\n", i, strings.Repeat("words that wrap differently ", 1+i%4))
	}
	text := b.String()
	var layout TextareaLayout
	layout.Fit(canvas, text, fitTestSize, fitTestWidth, true)
	layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 5 * fitTestLine}, fitTestLine
	layout.Show(0)
	layout.Scroll = float32(len(layout.Rows)/2)*fitTestLine + 7 // a wheel took it to the middle
	top := layout.Rows[int(layout.Scroll/fitTestLine)].Start
	within := layout.Scroll - float32(int(layout.Scroll/fitTestLine))*fitTestLine
	layout.Show(0)
	row := int(layout.Scroll / fitTestLine)
	if got := layout.Rows[row]; got.Start > top || got.End < top && row+1 < len(layout.Rows) && layout.Rows[row+1].Start <= top {
		t.Fatalf("the row at the top is %v, which does not hold offset %d it held", got, top)
	}
	if got := layout.Scroll - float32(row)*fitTestLine; got != within {
		t.Fatalf("the top row is %v pixels in, was %v", got, within)
	}
	if layout.Scroll < fitTestLine {
		t.Fatal("the box went back to the caret")
	}
	for index := row; index <= min(row+5, len(layout.Rows)-1); index++ {
		if !layout.exact[index] {
			t.Fatalf("row %d in view is still estimated", index)
		}
	}
}

// An AutoGrow box measures the rows it can grow to, not the whole text, and is exact
// while the text fits it.
func TestTextareaLayoutGrowsByTheRowsItCanShow(t *testing.T) {
	var measured []string
	canvas := shapedCanvas(&measured)
	theme := gfx.LightTheme()
	width := float32(fitTestWidth) + 2*theme.Metrics.TextInsetX()
	text := longText(20000)
	var layout TextareaLayout
	height := layout.ContentHeightUpTo(canvas, theme, text, width, false, 5)
	if want := float32(6)*(theme.Metrics.FontSizeM+7) + 2*TextareaPadY; height < want {
		t.Fatalf("a long text is %v tall, want past five rows (%v)", height, want)
	}
	fromParagraphs(t, "grow", measured, strings.Split(text, "\n")[:6]...)

	short := "one line\nand a second one that is long enough to wrap in the box"
	var small TextareaLayout
	if got, want := small.ContentHeightUpTo(canvas, theme, short, width, false, 5), small.ContentHeight(shapedCanvas(nil), theme, short, width, false); got != want {
		t.Fatalf("a short text is %v tall, want the exact %v", got, want)
	}
}

// BenchmarkTextareaLayoutOpen lays out a text of 20000 lines and shows its first rows:
// the work of the first paint.
func BenchmarkTextareaLayoutOpen(b *testing.B) {
	canvas := shapedCanvas(nil)
	text := longText(20000)
	b.ReportAllocs()
	for b.Loop() {
		var layout TextareaLayout
		layout.Fit(canvas, text, fitTestSize, fitTestWidth, true)
		layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 20 * fitTestLine}, fitTestLine
		layout.Show(0)
	}
}

// BenchmarkTextareaLayoutAppendLine is a log of 20000 lines growing by one line:
// the work of the frame that shows it.
func BenchmarkTextareaLayoutAppendLine(b *testing.B) {
	canvas := shapedCanvas(nil)
	base := longText(20000)
	texts := make([]string, 64)
	for i := range texts {
		texts[i] = base + fmt.Sprintf("appended %d\n", i)
	}
	var layout TextareaLayout
	layout.Fit(canvas, base, fitTestSize, fitTestWidth, true)
	layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 20 * fitTestLine}, fitTestLine
	layout.Show(len(base))
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		text := texts[i%len(texts)]
		layout.Fit(canvas, text, fitTestSize, fitTestWidth, true)
		layout.Show(len(text))
	}
}

// BenchmarkTextareaLayoutTypeInTheMiddle is a key typed in the middle of a text of
// 20000 lines.
func BenchmarkTextareaLayoutTypeInTheMiddle(b *testing.B) {
	canvas := shapedCanvas(nil)
	base := longText(20000)
	middle := strings.Index(base, "line 10000 ")
	texts := [2]string{base, base[:middle] + "x" + base[middle:]}
	var layout TextareaLayout
	layout.Fit(canvas, base, fitTestSize, fitTestWidth, true)
	layout.Content, layout.LineHeight = gfx.Rect{Width: fitTestWidth, Height: 20 * fitTestLine}, fitTestLine
	layout.Show(middle)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		layout.Fit(canvas, texts[(i+1)%2], fitTestSize, fitTestWidth, true)
		layout.Show(middle + (i+1)%2)
	}
}
