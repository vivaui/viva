package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A labelled page used to cost O(controls*hits) in nameFromLabels. This keeps
// the linear implementation honest; the page has one label per control.
func BenchmarkPainterLabelledPage(b *testing.B) {
	for _, controls := range []int{1000, 4000} {
		b.Run(map[int]string{1000: "1k", 4000: "4k"}[controls], func(b *testing.B) {
			var canvas gfx.Canvas
			config := Config{Title: "labels", Width: 1200, Height: 1200}
			config.Paint = func(c *gfx.Canvas, _ gfx.Size) {
				for index := 0; index < controls; index++ {
					id := gfx.Item(index + 1)
					bounds := gfx.Rect{X: float32(index%100) * 10, Y: float32(index/100) * 10, Width: 8, Height: 8}
					c.HitWithLabel(id, bounds, "control")
					c.HitLabel(id, gfx.Rect{X: bounds.X, Y: bounds.Y + 2, Width: 4, Height: 4}, "label", false)
				}
			}
			painter := newTestPainter(b, &canvas, config)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if _, err := painter.repaint(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// A repaint that changes a label opens the published-paint gate: it pays the
// full region and semantic mirror. This is the cost of a real label change, as
// opposed to BenchmarkPainterLabelledPage's no-op repaint.
func BenchmarkPainterLabelledPageChanged(b *testing.B) {
	for _, controls := range []int{1000, 4000} {
		b.Run(map[int]string{1000: "1k", 4000: "4k"}[controls], func(b *testing.B) {
			var canvas gfx.Canvas
			config := Config{Title: "labels", Width: 1200, Height: 1200}
			label := "odd"
			config.Paint = func(c *gfx.Canvas, _ gfx.Size) {
				for index := 0; index < controls; index++ {
					id := gfx.Item(index + 1)
					bounds := gfx.Rect{X: float32(index%100) * 10, Y: float32(index/100) * 10, Width: 8, Height: 8}
					c.HitWithLabel(id, bounds, "control")
					c.HitLabel(id, gfx.Rect{X: bounds.X, Y: bounds.Y + 2, Width: 4, Height: 4}, label, false)
				}
			}
			painter := newTestPainter(b, &canvas, config)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if label == "odd" {
					label = "even"
				} else {
					label = "odd"
				}
				if _, err := painter.repaint(); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
