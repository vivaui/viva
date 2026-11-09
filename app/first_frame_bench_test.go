package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The first frame has no previous publish to reuse: it constructs the painter,
// runs the paint callback for the first time, builds the scene and the frame,
// and publishes everything. Applications pay it once, so it is measured apart
// from the steady-state repaint benchmarks.
func BenchmarkPainterFirstFrame(b *testing.B) {
	config := Config{Title: "first frame", Width: 1200, Height: 800}
	config.Paint = func(canvas *gfx.Canvas, _ gfx.Size) {
		for index := 0; index < 200; index++ {
			id := gfx.Item(index + 1)
			bounds := gfx.Rect{X: float32(index%20) * 50, Y: float32(index/20) * 16, Width: 44, Height: 12}
			canvas.HitWithLabel(id, bounds, "item")
			canvas.Label("item", gfx.Rect{X: bounds.X + 4, Y: bounds.Y + 2, Width: 36, Height: 8}, gfx.TextStyle{Size: 10})
		}
	}
	b.ReportAllocs()
	for b.Loop() {
		var canvas gfx.Canvas
		if painter := newTestPainter(b, &canvas, config); painter == nil {
			b.Fatal("no painter")
		}
	}
}
