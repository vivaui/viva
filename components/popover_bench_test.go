package components

import (
	"testing"
	"unsafe"

	"github.com/vivaui/viva/gfx"
)

func benchmarkPopoverRendering(b *testing.B, options PopoverOptions) {
	var tree gfx.Tree
	feature := tree.Feature("popover-benchmark")
	options.DismissID, options.Scope = tree.ID(feature, "dismiss"), tree.ID(feature, "scope")
	options.Layer = 2
	options.ContentSize = gfx.Size{Width: 320, Height: 160}
	var canvas gfx.Canvas
	canvas.SetTree(&tree)
	theme := gfx.LightTheme()
	viewport, anchor := gfx.Rect{Width: 800, Height: 600}, gfx.Rect{X: 400, Y: 20, Width: 28, Height: 28}
	paint := func() {
		canvas.Reset()
		BeginPopover(&canvas, theme, viewport, anchor, options).Paint("Transfers", "Tasks across all connections")
	}
	for range 3 {
		paint()
	}
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
	b.ReportMetric(float64(unsafe.Sizeof(options)), "B/options")
}

func BenchmarkPopoverRenderingDefault(b *testing.B) { benchmarkPopoverRendering(b, PopoverOptions{}) }

func BenchmarkPopoverRenderingEnhanced(b *testing.B) {
	benchmarkPopoverRendering(b, PopoverOptions{AnchorDismiss: true, TitleWeight: gfx.WeightBold})
}
