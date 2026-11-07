//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// The same pixel contract runs on D3D11, Vulkan and Metal (with vivahead).
func TestVectorCoverageAtDeviceScales(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	r := newRenderer()
	defer r.close()
	for _, scale := range []float64{1, 1.1, 1.25, 1.3, 1.5, 1.75, 2, 2.5, 3, 1.25} {
		t.Run(fmt.Sprintf("%gx", scale), func(t *testing.T) {
			point := func(x, y float32) ui.Point { return ui.Point{X: x / float32(scale), Y: y / float32(scale)} }
			draw := func(primitives ...app.Primitive) *image.RGBA {
				t.Helper()
				// Unequal, odd dimensions exercise backing-size rounding independently
				// on both axes; an integral 40*scale used to hide fractional-DPI drift.
				pixels, err := r.render(app.Snapshot{Width: 41, Height: 37, Clear: app.Color{Alpha: 1}, Primitives: primitives}, scale)
				if err != nil {
					t.Fatal(err)
				}
				return pixels
			}
			near := func(got uint8, want float64) {
				t.Helper()
				if math.Abs(float64(got)-want) > 4 {
					t.Fatalf("channel = %d, want about %.1f", got, want)
				}
			}
			tri := app.Primitive{Kind: app.TrianglePrimitive, EdgeMask: 7, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
				Points: [3]ui.Point{point(10.75, 4), point(30, 4), point(10.75, 28)}}
			for range 2 {
				pixels := draw(tri)
				near(pixels.RGBAAt(10, 10).R, 64)
				near(pixels.RGBAAt(9, 10).R, 0)
				near(pixels.RGBAAt(11, 10).R, 255)
				tri.Points[1], tri.Points[2] = tri.Points[2], tri.Points[1]
			}
			diagonal := tri
			diagonal.Points = [3]ui.Point{point(4, 4), point(24.75, 4), point(4, 24.75)}
			pixels := draw(diagonal)
			near(pixels.RGBAAt(14, 14).R, 255*(0.5-0.25/math.Sqrt2))
			near(pixels.RGBAAt(15, 15).R, 0)
			// An acute tip's offset lines meet far away. That miter is geometry,
			// not coverage: it must not draw a spike past the real endpoint.
			needle := tri
			needle.Points = [3]ui.Point{point(20, 20), point(4, 19.8), point(4, 20.2)}
			pixels = draw(needle)
			for x := 21; x < 30; x++ {
				near(pixels.RGBAAt(x, 20).R, 0)
			}
			// A zero-area face must stay empty, even with boundary feathering.
			needle.Points = [3]ui.Point{point(4, 4), point(4, 4), point(4, 4)}
			pixels = draw(needle)
			for y := 0; y < pixels.Rect.Dy(); y++ {
				for x := 0; x < pixels.Rect.Dx(); x++ {
					near(pixels.RGBAAt(x, y).R, 0)
				}
			}
			// Expansion must not change alpha or move the original gradient.
			tri.Shaded = true
			tri.Color = ui.Color{Red: 1, Alpha: 0.5}
			tri.Shade = [2]ui.Color{{Green: 1, Alpha: 0.5}, {Blue: 1, Alpha: 0.5}}
			pixels = draw(tri)
			b, c := (14.5-10.75)/(30-10.75), (10.5-4)/(28-4)
			got := pixels.RGBAAt(14, 10)
			near(got.R, 255*0.5*(1-b-c))
			near(got.G, 255*0.5*b)
			near(got.B, 255*0.5*c)
			// Two translucent triangles sharing a diagonal must not overlap or crack.
			tri.Shaded = false
			tri.Color = ui.Color{Red: 1, Alpha: 0.5}
			tri.Points = [3]ui.Point{point(8.25, 8.25), point(30.25, 8.25), point(30.25, 30.25)}
			tri.EdgeMask = 3
			twin := tri
			twin.Points = [3]ui.Point{point(8.25, 8.25), point(30.25, 30.25), point(8.25, 30.25)}
			twin.EdgeMask = 6
			pixels = draw(tri, twin)
			for at := 10; at < 29; at++ {
				near(pixels.RGBAAt(at, at).R, 128)
			}
			// Native scissors still clip the expanded silhouette, including its halo.
			tri.Clipped = true
			tri.Clip = ui.Rect{X: 9 / float32(scale), Y: 9 / float32(scale), Width: 20 / float32(scale), Height: 20 / float32(scale)}
			pixels = draw(tri)
			for y := 0; y < pixels.Rect.Dy(); y++ {
				for x := 0; x < pixels.Rect.Dx(); x++ {
					if x < 9 || y < 9 || x >= 29 || y >= 29 {
						near(pixels.RGBAAt(x, y).R, 0)
					}
				}
			}
		})
	}
}
