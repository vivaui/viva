package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The memo hands a line that arrives unchanged back as it replaced it, and a
// line whose runs changed — a recolour keeps the value — gets a replacement
// built for the new runs, not the remembered one.
func TestReplaceCellGlyphsMemoFollowsItsInput(t *testing.T) {
	painter := &painter{}
	value := "┌──┐ ok"
	runs := []gfx.TextRun{{Bytes: int32(len(value)), Cells: 6, Color: gfx.Color{Alpha: 1}}}
	firstValue, firstRuns := painter.replaceCellGlyphs(value, runs)
	if firstValue == value {
		t.Fatalf("a line with cell glyphs came back unchanged: %q", firstValue)
	}
	againValue, againRuns := painter.replaceCellGlyphs(value, runs)
	if againValue != firstValue || &againRuns[0] != &firstRuns[0] {
		t.Fatal("the same line was replaced again instead of remembered")
	}

	recolored := []gfx.TextRun{{Bytes: int32(len(value)), Cells: 6, Color: gfx.Color{Red: 1, Alpha: 1}}}
	recValue, recRuns := painter.replaceCellGlyphs(value, recolored)
	if recRuns[0].Color != recolored[0].Color {
		t.Fatalf("a recoloured line got the remembered colour %+v", recRuns[0].Color)
	}
	if recValue != firstValue {
		t.Fatalf("the replacement text changed with the colour: %q and %q", recValue, firstValue)
	}

	plain := "plain line"
	plainRuns := []gfx.TextRun{{Bytes: int32(len(plain)), Cells: 10, Color: gfx.Color{Alpha: 1}}}
	outValue, outRuns := painter.replaceCellGlyphs(plain, plainRuns)
	if outValue != plain || &outRuns[0] != &plainRuns[0] {
		t.Fatal("a line without cell glyphs was copied or remembered")
	}
}

// A keystroke in a grid-shaped page re-sends every row's label; the rows that
// did not change reach the label sync with the same value and runs, and their
// cell glyph replacement is remembered rather than rebuilt.
func BenchmarkPainterBorderedRowsOneChanged(b *testing.B) {
	const rows = 50
	values := make([]string, rows)
	for row := range values {
		values[row] = "┌────┤ title ├────┐ padding"
	}
	var canvas gfx.Canvas
	config := Config{Title: "rows", Width: 800, Height: 800}
	changed := 0
	config.Paint = func(c *gfx.Canvas, _ gfx.Size) {
		for row := 0; row < rows; row++ {
			value := values[row]
			c.LabelRuns(value, gfx.Rect{X: 4, Y: float32(row) * 16, Width: 768, Height: 16},
				gfx.TextStyle{Size: 12}, 7.2,
				[]gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Color: gfx.Color{Alpha: 1}}})
		}
	}
	painter := newTestPainter(b, &canvas, config)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		if values[changed] == "┌────┤ title ├────┐ padding" {
			values[changed] = "┌────┤ t1tle ├────┐ padding"
		} else {
			values[changed] = "┌────┤ title ├────┐ padding"
		}
		changed = (changed + 1) % rows
		if _, err := painter.repaint(); err != nil {
			b.Fatal(err)
		}
	}
}
