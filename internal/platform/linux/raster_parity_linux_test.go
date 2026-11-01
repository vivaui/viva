package linux

import (
	"errors"
	"image"
	"math"
	"math/rand"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// paritySceneRectangles is a scene of every kind of instance, with layers and clips: the shapes the
// CPU rasterizer has to draw as the GPU does.
func paritySceneRectangles() []render.Rectangle {
	random := rand.New(rand.NewSource(7))
	var out []render.Rectangle
	color := func() scene.Color {
		return scene.Color{Red: random.Float32(), Green: random.Float32(), Blue: random.Float32(), Alpha: 0.4 + 0.6*random.Float32()}
	}
	for i := 0; i < 40; i++ {
		out = append(out, render.Rectangle{
			Kind:   scene.KindRectangle,
			Bounds: scene.Rect{X: random.Float32() * 150, Y: random.Float32() * 100, Width: 4 + random.Float32()*60, Height: 4 + random.Float32()*40},
			Color:  color(), Radius: float32(random.Intn(3)) * 5 * random.Float32(), Layer: uint16(random.Intn(3)),
		})
	}
	// Shadows, with and without an offset, under rounded cards.
	for i := 0; i < 4; i++ {
		x, y := float32(20+i*40), float32(30+i*10)
		card := scene.Rect{X: x, Y: y, Width: 30, Height: 20}
		out = append(out,
			render.Rectangle{Kind: scene.KindShadow, Bounds: card, Radius: 6, Color: scene.Color{Alpha: 0.35}, Shadow: scene.Shadow{OffsetY: 3, Blur: 8, Spread: float32(i)}, Layer: 3},
			render.Rectangle{Kind: scene.KindRectangle, Bounds: card, Radius: 6, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Layer: 3})
	}
	// A shaded mesh with a shared diagonal and feathered silhouette, a lone triangle, a sliver.
	p := func(x, y float32) scene.Point { return scene.Point{X: x, Y: y} }
	a, b, c, d := p(100.3, 20.2), p(150.9, 22.6), p(98.4, 70.7), p(153.1, 69.9)
	shade := scene.Shade{On: true, Colors: [2]scene.Color{{Green: 1, Alpha: 1}, {Blue: 1, Alpha: 1}}}
	out = append(out,
		render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{a, b, c}, Color: scene.Color{Red: 1, Alpha: 1}, Shade: shade, EdgeMask: 0b101, Layer: 4},
		render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{b, d, c}, Color: scene.Color{Green: 1, Alpha: 1}, EdgeMask: 0b011, Layer: 4},
		render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{p(20.5, 80.5), p(60.1, 95.7), p(35.2, 110.3)}, Color: scene.Color{Red: 0.2, Green: 0.4, Blue: 0.9, Alpha: 0.8}, EdgeMask: 7, Layer: 4},
		render.Rectangle{Kind: scene.KindTriangle, Points: [3]scene.Point{p(70, 90), p(140, 91.5), p(70, 90.5)}, Color: scene.Color{Red: 0.9, Alpha: 1}, EdgeMask: 7, Layer: 4})
	// A clipped rectangle.
	out = append(out, render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 10, Y: 100, Width: 90, Height: 30}, Color: scene.Color{Red: 0.8, Blue: 0.3, Alpha: 1}, Radius: 8, Layer: 5,
		Clipped: true, Clip: scene.Rect{X: 20.4, Y: 105.3, Width: 60.2, Height: 20.6}})
	return out
}

// TestRasterizerMatchesTheGPUPath renders the same scene with the Vulkan pipelines and with the CPU
// rasterizer, and says how far apart they are. It skips on a machine with no Vulkan driver.
func TestRasterizerMatchesTheGPUPath(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		const width, height = 180, 140
		clear := platform.Color{Red: 0.93, Green: 0.95, Blue: 0.98, Alpha: 1}
		off, err := NewOffscreen(width, height, scale, clear)
		if errors.Is(err, platform.ErrNoGraphics) {
			t.Skip("no Vulkan driver")
		}
		if err != nil {
			t.Fatalf("scale %v: %v", scale, err)
		}
		rects := paritySceneRectangles()
		if err := off.Set(rects, nil); err != nil {
			off.Close()
			t.Fatalf("scale %v: %v", scale, err)
		}
		gpu, err := off.Image()
		off.Close()
		if err != nil {
			t.Fatalf("scale %v: %v", scale, err)
		}

		cpu := raster.NewCanvas(gpu.Bounds().Dx(), gpu.Bounds().Dy())
		var r raster.Rasterizer
		r.Draw(cpu, raster.Params{Scale: float32(scale), Clear: scene.Color{Red: float32(clear.Red), Green: float32(clear.Green), Blue: float32(clear.Blue), Alpha: float32(clear.Alpha)}}, rects, nil)

		worst, over2, over8, total := 0, 0, 0, 0
		var worstAt image.Point
		for y := 0; y < cpu.Height; y++ {
			for x := 0; x < cpu.Width; x++ {
				g := gpu.RGBAAt(x, y)
				i := y*cpu.Stride + x*4
				for k, want := range [4]uint8{g.B, g.G, g.R, g.A} {
					diff := int(cpu.Pix[i+k]) - int(want)
					diff = int(math.Abs(float64(diff)))
					total++
					if diff > 2 {
						over2++
					}
					if diff > 8 {
						over8++
					}
					if diff > worst {
						worst, worstAt = diff, image.Pt(x, y)
					}
				}
			}
		}
		t.Logf("scale %.2f: %dx%d  max channel difference %d at %v;  >2 levels: %.3f%%  >8 levels: %.3f%%",
			scale, cpu.Width, cpu.Height, worst, worstAt, 100*float64(over2)/float64(total), 100*float64(over8)/float64(total))
		// Measured on Mesa's lavapipe: at most 3 levels apart, and only one value in 100,000 apart by
		// more than 2. A hardware GPU rounds differently in places; this guards the shapes and the
		// blending, not the last level.
		if worst > 6 || float64(over2)/float64(total) > 0.0001 || over8 > 0 {
			t.Errorf("scale %v: worst difference %d at %v; %.4f%% of channel values differ by more than 2 levels", scale, worst, worstAt, 100*float64(over2)/float64(total))
		}
	}
}
