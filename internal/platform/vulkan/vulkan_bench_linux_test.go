//go:build linux

package vulkan

import (
	"errors"
	"fmt"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// BenchmarkRendererChangedFrame is a repaint in which one rectangle changes color: the
// window's apply path (fence wait, command pool reset, upload and command recording) on a
// real Vulkan device, without presentation. The offscreen target has one command buffer; a
// window records one per swapchain image, so a window costs more per repaint.
func BenchmarkRendererChangedFrame(benchmark *testing.B) {
	for _, count := range []int{1, 100, 1000} {
		benchmark.Run(fmt.Sprintf("rectangles=%d", count), func(benchmark *testing.B) {
			offscreen, err := NewOffscreen(800, 600, 1, platform.Color{Alpha: 1}, newSolidRasterizer)
			if errors.Is(err, platform.ErrNoGraphics) {
				benchmark.Skipf("no Vulkan device: %v", err)
			}
			if err != nil {
				benchmark.Fatal(err)
			}
			defer offscreen.Close()
			rectangles := make([]render.Rectangle, count)
			for index := range rectangles {
				rectangles[index] = render.Rectangle{
					Bounds: scene.Rect{X: float32(index%50) * 16, Y: float32(index/50) * 16, Width: 12, Height: 12},
					Color:  scene.Color{Red: 0.2, Green: 0.4, Blue: 0.8, Alpha: 1},
				}
			}
			if err := offscreen.Set(rectangles, nil); err != nil {
				benchmark.Fatal(err)
			}
			renderer := offscreen.renderer
			benchmark.ReportAllocs()
			benchmark.ResetTimer()
			for iteration := range benchmark.N {
				rectangles[0].Color.Red = float32(iteration&1) * 0.5
				if _, err := renderer.Apply(platform.Frame{Update: render.Update{First: 0, Count: 1}, Rectangles: rectangles}); err != nil {
					benchmark.Fatal(err)
				}
			}
		})
	}
}
