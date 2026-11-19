package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// LayoutScrollArea is the one geometry both a page ScrollState and
// components.ScrollArea use. It must not allocate: a scrollbar is painted on
// every scrolling frame.
func BenchmarkLayoutScrollArea(b *testing.B) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 320, Height: 200}
	content := gfx.Size{Width: 900, Height: 4000}
	layout := ScrollAreaLayout{OffsetY: 120, BarSize: ScrollAreaBarSize}
	b.ReportAllocs()
	for b.Loop() {
		LayoutScrollArea(bounds, content, layout)
	}
}
