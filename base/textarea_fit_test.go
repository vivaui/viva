package base

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

const fitTestSize, fitTestWidth = 13, 180

// An edit leaves the rows a fresh wrap of the new text would give, whatever it
// changed: inside a paragraph, across paragraphs, a newline added or removed, at
// either end, the whole text.
func TestTextareaLayoutRewrapsAnEditAsAFreshWrapWould(t *testing.T) {
	pieces := []string{"a", "word ", "\n", "\r\n", "  ", "中文", "the quick brown fox ", "averyveryverylongwordthatmustbreak", "é", "\n\n"}
	random := rand.New(rand.NewPCG(1, 2))
	piece := func() string {
		var b strings.Builder
		for range random.IntN(6) {
			b.WriteString(pieces[random.IntN(len(pieces))])
		}
		return b.String()
	}
	for _, wrap := range []bool{true, false} {
		var canvas gfx.Canvas
		var layout TextareaLayout
		text := ""
		for step := range 2000 {
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
			layout.Fit(&canvas, text, fitTestSize, fitTestWidth, wrap)
			width := float32(fitTestWidth)
			if !wrap {
				width = 0
			}
			want := canvas.WrapRows(nil, text, fitTestSize, gfx.WeightRegular, width)
			if !slices.Equal(layout.Rows, want) {
				t.Fatalf("wrap %v, step %d: rows of %q\n got %v\nwant %v", wrap, step, text, layout.Rows, want)
			}
		}
	}
}

// Typing in, or appending to, a long text measures only the paragraph it changed,
// so the cost of an edit does not grow with the text.
func TestTextareaLayoutMeasuresOnlyTheEditedParagraph(t *testing.T) {
	var canvas gfx.Canvas
	var measured []string
	canvas.SetMeasurer(func(value string, size, weight float32) float32 {
		measured = append(measured, value)
		return float32(len(value)) * size / 2
	})
	var b strings.Builder
	for i := range 2000 {
		fmt.Fprintf(&b, "line %d the quick brown fox jumps over the lazy dog\n", i)
	}
	text := b.String()
	var layout TextareaLayout
	layout.Fit(&canvas, text, fitTestSize, fitTestWidth, true)
	for _, edit := range []struct{ name, text, paragraph string }{
		{"appended", text + "a new line at the end", "a new line at the end"},
		{"typed", strings.Replace(text, "line 1000 the", "line 1000 xthe", 1), "line 1000 xthe quick brown fox jumps over the lazy dog"},
	} {
		measured = measured[:0]
		layout.Fit(&canvas, edit.text, fitTestSize, fitTestWidth, true)
		if len(measured) == 0 {
			t.Fatalf("%s: nothing was measured", edit.name)
		}
		for _, value := range measured {
			if !strings.Contains(edit.paragraph, strings.TrimRight(value, " ")) {
				t.Fatalf("%s: measured %q, which is not in the edited paragraph %q", edit.name, value, edit.paragraph)
			}
		}
		if want := canvas.WrapRows(nil, edit.text, fitTestSize, gfx.WeightRegular, fitTestWidth); !slices.Equal(layout.Rows, want) {
			t.Fatalf("%s: the rows differ from a fresh wrap", edit.name)
		}
		layout.Fit(&canvas, text, fitTestSize, fitTestWidth, true)
	}
}

// BenchmarkTextareaLayoutAppendLine is a log of 20000 lines growing by one line:
// the work of the frame that shows it.
func BenchmarkTextareaLayoutAppendLine(b *testing.B) {
	var canvas gfx.Canvas
	var text strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&text, "line %d the quick brown fox jumps over the lazy dog 中文混排\n", i)
	}
	base := text.String()
	texts := make([]string, 64)
	for i := range texts {
		texts[i] = base + fmt.Sprintf("appended %d\n", i)
	}
	var layout TextareaLayout
	layout.Fit(&canvas, base, fitTestSize, fitTestWidth, true)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		layout.Fit(&canvas, texts[i%len(texts)], fitTestSize, fitTestWidth, true)
	}
}

// BenchmarkTextareaLayoutTypeInTheMiddle is a key typed in the middle of a text of
// 20000 lines.
func BenchmarkTextareaLayoutTypeInTheMiddle(b *testing.B) {
	var canvas gfx.Canvas
	var text strings.Builder
	for i := range 20000 {
		fmt.Fprintf(&text, "line %d the quick brown fox jumps over the lazy dog 中文混排\n", i)
	}
	base := text.String()
	middle := strings.Index(base, "line 10000 ")
	texts := [2]string{base, base[:middle] + "x" + base[middle:]}
	var layout TextareaLayout
	layout.Fit(&canvas, base, fitTestSize, fitTestWidth, true)
	b.ReportAllocs()
	for i := 0; b.Loop(); i++ {
		layout.Fit(&canvas, texts[(i+1)%2], fitTestSize, fitTestWidth, true)
	}
}
