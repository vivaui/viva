package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

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
