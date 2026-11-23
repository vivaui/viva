//go:build darwin && vivahead && !ios

package darwin

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

// BenchmarkRasterizeInterfaceLabels is the cost of drawing 60 interface labels (Latin,
// CJK and mixed, regular and semibold, 12 to 16 px) into textures from scratch: the first
// frame of a screen, or a theme change.
func BenchmarkRasterizeInterfaceLabels(b *testing.B) {
	words := []string{"Connect", "Cancel", "Settings", "host-01", "连接", "已连接 3 sessions", "New connection", "Preferences", "界面一致性", "12:45:07"}
	build := func(round int) []TextLabel {
		labels := make([]TextLabel, 60)
		for index := range labels {
			labels[index] = TextLabel{
				Value: fmt.Sprintf("%s %d", words[index%len(words)], round*1000+index),
				X:     float32(10 + index%4*150), Y: float32(8 + index/4*24), Width: 140, Height: 20,
				FontSize: float32(12 + index%3*2), Weight: float32(index % 2 * 3 / 5),
				Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1},
			}
		}
		return labels
	}
	for _, scale := range []float64{1, 2} {
		b.Run(fmt.Sprintf("rasterize@%gx", scale), func(b *testing.B) {
			offscreen, err := NewOffscreen(640, 400, scale, platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
			if err != nil {
				b.Skipf("no Metal device: %v", err)
			}
			defer offscreen.Close()
			round := 0
			b.ReportAllocs()
			for b.Loop() {
				round++
				if err := offscreen.Set(nil, build(round), nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
