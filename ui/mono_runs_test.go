package ui

import (
	"fmt"
	"strings"
	"testing"
)

// newMonoTestCanvas is a canvas whose fixed-pitch cell at size 10 is exactly 6 wide and
// 12 tall, with no pixel scale to round it.
func newMonoTestCanvas(t *testing.T) *Canvas {
	t.Helper()
	canvas := new(Canvas)
	canvas.SetCellMeasurer(func(size float32, _ MonoFace) (float32, float32) { return size * 3 / 5, size * 6 / 5 })
	if width, _ := canvas.MonoCellSize(10, MonoSystem); width != 6 {
		t.Fatalf("test cell is %v wide at size 10, want 6", width)
	}
	return canvas
}

// A Mono label is set on its grid: each stretch of ASCII and each other character, with
// the combining marks after it, is one run on the cells TextWidthStyle counts for it, in
// the color, weight and slant of the style.
func TestMonoLabelIsSetOnItsGridOfCells(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	style := TextStyle{Size: 10, Mono: true, Color: Color{Red: 1, Alpha: 1}, Weight: WeightBold, Italic: true}
	value := "ab界e\u0301c" // two ASCII, one wide, one letter with a combining mark, one ASCII
	canvas.Label(value, Rect{Width: 100, Height: 12}, style)

	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	text := texts[0]
	if text.CellWidth != 6 {
		t.Errorf("cell width = %v, want 6", text.CellWidth)
	}
	want := []TextRun{
		{Column: 0, Bytes: 2, Cells: 2},
		{Column: 2, Bytes: 3, Cells: 2},
		{Column: 4, Bytes: 3, Cells: 1},
		{Column: 5, Bytes: 1, Cells: 1},
	}
	for i := range want {
		want[i].Color, want[i].Weight, want[i].Italic = style.Color, style.Weight, style.Italic
	}
	got := canvas.TextRuns(text)
	if len(got) != len(want) {
		t.Fatalf("runs = %+v, want %+v", got, want)
	}
	bytes := int32(0)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("run %d = %+v, want %+v", i, got[i], want[i])
		}
		bytes += got[i].Bytes
	}
	if int(bytes) != len(value) {
		t.Errorf("the runs hold %d bytes, want the %d of the value", bytes, len(value))
	}
	if width := canvas.TextWidthStyle(value, style); width != 36 {
		t.Errorf("width = %v, want 36 (6 cells of 6)", width)
	}
}

// In an installed family a stretch of two or more ASCII characters is shaped as one line
// fitted to its cells, so the family's ligatures appear; a lone character and every
// character that is not ASCII still stand on their own cells.
func TestMonoLabelInAnInstalledFamilyShapesItsASCIIStretches(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	style := TextStyle{Size: 10, Mono: true, MonoFace: MonoFamily("Viva Test Ligature Mono")}
	canvas.Label("if a != b界x", Rect{Width: 100, Height: 12}, style)

	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	got := canvas.TextRuns(texts[0])
	want := []TextRun{
		{Column: 0, Bytes: 9, Cells: 9, Ligatures: true},
		{Column: 9, Bytes: 3, Cells: 2},
		{Column: 11, Bytes: 1, Cells: 1},
	}
	if len(got) != len(want) {
		t.Fatalf("runs = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("run %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The character a combining mark follows leaves the stretch of ASCII before it for a run
// of its own with the mark, after a long stretch and after a control character too.
func TestMonoLabelKeepsACombiningMarkWithItsCharacter(t *testing.T) {
	cases := []struct {
		name  string
		value string
		want  []TextRun
	}{
		{"after a long stretch", "abcdefghije\u0301", []TextRun{{Column: 0, Bytes: 10, Cells: 10}, {Column: 10, Bytes: 3, Cells: 1}}},
		{"after a control character", "\t\u0301ab", []TextRun{{Column: 0, Bytes: 3, Cells: 1}, {Column: 1, Bytes: 2, Cells: 2}}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			canvas := newMonoTestCanvas(t)
			canvas.Label(c.value, Rect{Width: 100, Height: 12}, TextStyle{Size: 10, Mono: true})
			got := canvas.TextRuns(canvas.Texts()[0])
			if len(got) != len(c.want) {
				t.Fatalf("runs = %+v, want %+v", got, c.want)
			}
			for i := range c.want {
				if got[i] != c.want[i] {
					t.Errorf("run %d = %+v, want %+v", i, got[i], c.want[i])
				}
			}
		})
	}
}

// The scans that read eight bytes at a time stop at the first byte outside their range,
// wherever in a word it is, and pass over the bytes at the edges of the range.
func TestASCIIScansStopAtTheFirstByteOutsideTheirRange(t *testing.T) {
	const length = 20
	for position := 0; position < length; position++ {
		for _, c := range []struct {
			b                  byte
			printable, isASCII bool
		}{
			{0x00, false, true}, {0x1f, false, true}, {0x20, true, true}, {0x7e, true, true},
			{0x7f, false, true}, {0x80, false, false}, {0xc3, false, false}, {0xff, false, false},
		} {
			line := []byte(strings.Repeat("a", length))
			line[position] = c.b
			value := string(line)
			for _, from := range []int{0, 3} {
				if from > position {
					continue
				}
				printable, ascii := length, length
				if !c.printable {
					printable = position
				}
				if !c.isASCII {
					ascii = position
				}
				if got := printableASCIIEnd(value, from); got != printable {
					t.Errorf("printableASCIIEnd with %#x at %d, from %d = %d, want %d", c.b, position, from, got, printable)
				}
				if got := asciiEnd(value, from); got != ascii {
					t.Errorf("asciiEnd with %#x at %d, from %d = %d, want %d", c.b, position, from, got, ascii)
				}
			}
		}
	}
}

// A combining mark with no character before it still has to be drawn: it gets a run of its
// own that spans no cell, and the next character starts on the same column.
func TestMonoLabelStartingWithACombiningMarkGivesItARunOfNoCells(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	canvas.Label("\u0301x", Rect{Width: 100, Height: 12}, TextStyle{Size: 10, Mono: true})

	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(texts))
	}
	got := canvas.TextRuns(texts[0])
	want := []TextRun{{Column: 0, Bytes: 2, Cells: 0}, {Column: 0, Bytes: 1, Cells: 1}}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("runs = %+v, want %+v", got, want)
	}
}

// A line that is drawn in the visual order of UAX #9, or that no run may split, or that
// has nothing to set, keeps the font's own layout: no cell width and no runs.
func TestMonoLabelsThatCannotBeSetOnAGridGetNoRuns(t *testing.T) {
	cases := []struct {
		name  string
		value string
		style TextStyle
	}{
		{"right to left", "שלום", TextStyle{Size: 10, Mono: true}},
		{"mixed directions", "abc שלום", TextStyle{Size: 10, Mono: true}},
		{"left to right forced right to left", "abc", TextStyle{Size: 10, Mono: true, Direction: DirectionRTL}},
		{"invalid UTF-8", "a\xffb", TextStyle{Size: 10, Mono: true}},
		{"empty", "", TextStyle{Size: 10, Mono: true}},
		{"not fixed-pitch", "abc", TextStyle{Size: 10}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			canvas := newMonoTestCanvas(t)
			canvas.Label("xyz", Rect{Width: 100, Height: 12}, TextStyle{Size: 10, Mono: true})
			before := len(canvas.AllTextRuns())
			if before != 1 {
				t.Fatalf("the first label has %d runs, want 1", before)
			}

			canvas.Label(c.value, Rect{Y: 12, Width: 100, Height: 12}, c.style)
			texts := canvas.Texts()
			if len(texts) != 2 {
				t.Fatalf("texts = %d, want 2", len(texts))
			}
			text := texts[1]
			if text.RunCount != 0 || text.CellWidth != 0 {
				t.Errorf("run count %d and cell width %v, want none", text.RunCount, text.CellWidth)
			}
			if runs := canvas.TextRuns(text); runs != nil {
				t.Errorf("runs = %+v, want nil", runs)
			}
			if after := len(canvas.AllTextRuns()); after != before {
				t.Errorf("the run arena went from %d to %d runs, want it unchanged", before, after)
			}
		})
	}
}

// Numbers aligned to the end of a column end on one edge, whatever they are made of, since
// the width LabelAligned places them by is the width of the cells they stand on.
func TestRightAlignedMonoNumbersEndOnTheSameEdge(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	style := TextStyle{Size: 10, Mono: true}
	cases := []struct {
		value string
		cells int
		x     float32
	}{
		{"7", 1, 64},
		{"65536", 5, 40},
		{"界", 2, 58},
	}
	for _, c := range cases {
		canvas.LabelAligned(c.value, Rect{X: 10, Width: 60, Height: 12}, style, AlignEnd)
	}

	texts := canvas.Texts()
	if len(texts) != len(cases) {
		t.Fatalf("texts = %d, want %d", len(texts), len(cases))
	}
	for i, c := range cases {
		text := texts[i]
		if end := text.Bounds.X + float32(c.cells)*6; end != 70 {
			t.Errorf("%q ends at %v, want 70", c.value, end)
		}
		if text.Bounds.X != c.x {
			t.Errorf("%q starts at %v, want %v", c.value, text.Bounds.X, c.x)
		}
		if text.Bounds.Width != 60 {
			t.Errorf("%q has bounds %v wide, want the 60 it was given", c.value, text.Bounds.Width)
		}
	}
}

// The caret, the selection and a click count cells, not characters: a wide character is
// two of them, and a combining mark none.
func TestMonoCaretSelectionAndHitTestingCountCells(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	style := TextStyle{Size: 10, Mono: true}
	value := "a界b" // offsets 0, 1 and 4 start a character; 5 is the end

	if got := canvas.TextCaretStyle(value, style, 4); got != 18 {
		t.Errorf("caret after the wide character = %v, want 18", got)
	}
	spans := canvas.TextSelectionStyle(nil, value, style, 1, 4)
	if len(spans) != 1 || spans[0] != (TextSpan{X: 6, Width: 12}) {
		t.Errorf("selection of the wide character = %+v, want one span at 6, 12 wide", spans)
	}
	if got := canvas.TextOffsetAtStyle(value, style, 17); got != 4 {
		t.Errorf("offset at 17 = %d, want 4, the boundary at 18", got)
	}
	if got := canvas.TextOffsetAtStyle(value, style, 2); got != 0 {
		t.Errorf("offset at 2 = %d, want 0, the boundary at 0", got)
	}
	if got := canvas.TextWidthStyle("e\u0301", style); got != 6 {
		t.Errorf("width of a letter with a combining mark = %v, want 6", got)
	}
}

// Once its arenas have grown, painting a screen of Mono labels allocates nothing, though
// every label gets its runs.
func TestWarmMonoLabelsDoNotAllocate(t *testing.T) {
	canvas := newMonoTestCanvas(t)
	style := TextStyle{Size: 10, Mono: true, Color: Color{Red: 1, Alpha: 1}}
	paint := func() {
		canvas.Reset()
		for row := 0; row < 40; row++ {
			canvas.Label("some text per row 界 \u00e9", Rect{Y: float32(row) * 15, Width: 700, Height: 15}, style)
		}
	}
	paint()
	if len(canvas.AllTextRuns()) == 0 {
		t.Fatal("the labels got no runs")
	}
	if allocs := testing.AllocsPerRun(20, paint); allocs != 0 {
		t.Errorf("a warm paint of 40 Mono labels allocates %v times", allocs)
	}
}

// BenchmarkCanvasMonoLabels paints 40 Mono labels: rows of 120 digits, which have no
// strong direction, so that the base direction is looked for through the whole line, and
// lines of code.
func BenchmarkCanvasMonoLabels(b *testing.B) {
	code := make([]string, 40)
	for row := range code {
		code[row] = fmt.Sprintf("%6d  func (editor *Editor) column(index, tabWidth int) int { return editor.lines[index].cells(tabWidth) + 1 }", row)
	}
	digits := make([]string, 40)
	for row := range digits {
		digits[row] = strings.Repeat("0123456789", 12)
	}
	for _, lines := range []struct {
		name  string
		lines []string
	}{{"digits", digits}, {"code", code}} {
		b.Run(lines.name, func(b *testing.B) {
			var canvas Canvas
			canvas.SetCellMeasurer(func(size float32, _ MonoFace) (float32, float32) { return size * 0.6, size * 1.2 })
			style := TextStyle{Size: 13, Mono: true, Color: Color{Red: 1, Alpha: 1}}
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				canvas.Reset()
				for row, line := range lines.lines {
					canvas.Label(line, Rect{Y: float32(row) * 16, Width: 940, Height: 16}, style)
				}
			}
		})
	}
}
