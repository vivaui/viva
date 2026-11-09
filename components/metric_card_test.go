package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestMetricCardRowsStackWithoutOverlapAtEveryDensity(t *testing.T) {
	for _, density := range []gfx.Density{gfx.DensityCompact, gfx.DensityRegular, gfx.DensityLarge} {
		theme := gfx.LightTheme()
		theme.Metrics = theme.Metrics.WithDensity(density)
		var canvas gfx.Canvas
		MetricCard(&canvas, theme, gfx.Rect{Width: 220, Height: 160}, "Revenue", "$48.2K", "+12.4%", true)
		texts := canvas.Texts()
		if len(texts) != 3 {
			t.Fatalf("density %d: %d texts", density, len(texts))
		}
		for index := 1; index < len(texts); index++ {
			if texts[index].Bounds.Y < texts[index-1].Bounds.Y+texts[index-1].Bounds.Height {
				t.Fatalf("density %d: row %d %+v overlaps row %d %+v", density, index, texts[index].Bounds, index-1, texts[index-1].Bounds)
			}
		}
	}
}
