package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func BenchmarkInputPaintSurface(b *testing.B) {
	for _, translucent := range []bool{false, true} {
		name := "opaque"
		if translucent {
			name = "translucent"
		}
		b.Run(name, func(b *testing.B) {
			theme := ui.LightTheme()
			if translucent {
				theme.Background.Alpha = .1
			}
			var canvas ui.Canvas
			bounds := ui.Rect{Width: 200, Height: 32}
			snapshot := InputSnapshot{ID: ui.Item(1), Value: "Input", Focused: true}
			PaintInput(&canvas, theme, bounds, snapshot)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				canvas.Reset()
				PaintInput(&canvas, theme, bounds, snapshot)
			}
		})
	}
}
