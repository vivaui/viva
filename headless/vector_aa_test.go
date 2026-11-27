//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"image"
	"math"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
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
			point := func(x, y float32) gfx.Point { return gfx.Point{X: x / float32(scale), Y: y / float32(scale)} }
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
			tri := app.Primitive{Kind: app.TrianglePrimitive, EdgeMask: 7, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
				Points: [3]gfx.Point{point(10.75, 4), point(30, 4), point(10.75, 28)}}
			for range 2 {
				pixels := draw(tri)
				near(pixels.RGBAAt(10, 10).R, 64)
				near(pixels.RGBAAt(9, 10).R, 0)
				near(pixels.RGBAAt(11, 10).R, 255)
				tri.Points[1], tri.Points[2] = tri.Points[2], tri.Points[1]
			}
			diagonal := tri
			diagonal.Points = [3]gfx.Point{point(4, 4), point(24.75, 4), point(4, 24.75)}
			pixels := draw(diagonal)
			near(pixels.RGBAAt(14, 14).R, 255*(0.5-0.25/math.Sqrt2))
			near(pixels.RGBAAt(15, 15).R, 0)
			// An acute tip's offset lines meet far away. That miter is geometry,
			// not coverage: it must not draw a spike past the real endpoint.
			needle := tri
			needle.Points = [3]gfx.Point{point(20, 20), point(4, 19.8), point(4, 20.2)}
			pixels = draw(needle)
			for x := 21; x < 30; x++ {
				near(pixels.RGBAAt(x, 20).R, 0)
			}
			// A zero-area face must stay empty, even with boundary feathering.
			needle.Points = [3]gfx.Point{point(4, 4), point(4, 4), point(4, 4)}
			pixels = draw(needle)
			for y := 0; y < pixels.Rect.Dy(); y++ {
				for x := 0; x < pixels.Rect.Dx(); x++ {
					near(pixels.RGBAAt(x, y).R, 0)
				}
			}
			// Expansion must not change alpha or move the original gradient.
			tri.Shaded = true
			tri.Color = gfx.Color{Red: 1, Alpha: 0.5}
			tri.Shade = [2]gfx.Color{{Green: 1, Alpha: 0.5}, {Blue: 1, Alpha: 0.5}}
			pixels = draw(tri)
			b, c := (14.5-10.75)/(30-10.75), (10.5-4)/(28-4)
			got := pixels.RGBAAt(14, 10)
			near(got.R, 255*0.5*(1-b-c))
			near(got.G, 255*0.5*b)
			near(got.B, 255*0.5*c)
			// Two translucent triangles sharing a diagonal must not overlap or crack.
			tri.Shaded = false
			tri.Color = gfx.Color{Red: 1, Alpha: 0.5}
			tri.Points = [3]gfx.Point{point(8.25, 8.25), point(30.25, 8.25), point(30.25, 30.25)}
			tri.EdgeMask = 3
			twin := tri
			twin.Points = [3]gfx.Point{point(8.25, 8.25), point(30.25, 30.25), point(8.25, 30.25)}
			twin.EdgeMask = 6
			pixels = draw(tri, twin)
			for at := 10; at < 29; at++ {
				near(pixels.RGBAAt(at, at).R, 128)
			}
			// Native scissors still clip the expanded silhouette, including its halo.
			tri.Clipped = true
			tri.Clip = gfx.Rect{X: 9 / float32(scale), Y: 9 / float32(scale), Width: 20 / float32(scale), Height: 20 / float32(scale)}
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

// A stroke's internal diagonal must not change the coverage of either long edge,
// even near the ends where the triangle owning a pixel carries the opposite edge.
func TestVectorStrokeQuadCoverageIsUniform(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	r := newRenderer()
	defer r.close()
	for _, scale := range []float64{1, 1.25, 1.5, 2} {
		for _, width := range []float64{1, 1.25, 2, 3} {
			for _, reverse := range []bool{false, true} {
				for _, cap := range []gfx.StrokeCap{gfx.CapButt, gfx.CapRound} {
					t.Run(fmt.Sprintf("%gx/%gpx/reverse=%t/cap=%d", scale, width, reverse, cap), func(t *testing.T) {
						const center = 12.25
						from, to := float32(4/scale), float32(28/scale)
						if reverse {
							from, to = to, from
						}
						var canvas gfx.Canvas
						canvas.SetPixelScale(float32(scale))
						driver, err := app.NewDriver(app.Config{
							Title: "stroke coverage", Width: 32, Height: 32, ClearColor: app.Color{Alpha: 1},
							Paint: func(canvas *gfx.Canvas, size gfx.Size) {
								path := canvas.PathBuilder()
								path.MoveTo(float32(center/scale), from)
								path.LineTo(float32(center/scale), to)
								canvas.DrawPath(path.Path(size.Width, size.Height), gfx.Rect{Width: size.Width, Height: size.Height}, gfx.PathStyle{
									Stroke: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, StrokeWidth: float32(width / scale), Cap: cap,
								})
							},
						}, &canvas)
						if err != nil {
							t.Fatal(err)
						}
						defer driver.Close()
						pixels, err := r.render(driver.Snapshot(), scale)
						if err != nil {
							t.Fatal(err)
						}
						for x := 9; x <= 15; x++ {
							// Coverage is the width of the intersection of this pixel and the stroke.
							coverage := min(float64(x+1), center+width/2) - max(float64(x), center-width/2)
							want := math.Round(255 * max(0, coverage))
							// Both cap styles end at least two device pixels outside this band.
							for y := 6; y < 26; y++ {
								if got := pixels.RGBAAt(x, y).R; math.Abs(float64(got)-want) > 2 {
									t.Fatalf("stroke coverage at (%d,%d) = %d, want %g", x, y, got, want)
								}
							}
						}
					})
				}
			}
		}
	}
}

func TestVectorClippedStrokeCoverage(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	renderer := newRenderer()
	defer renderer.close()
	icon := gfx.Icon{
		ViewBox: 32, Stroke: 1, Cap: gfx.CapButt,
		Commands: []gfx.PathCommand{gfx.MoveTo(12.25, 4), gfx.LineTo(12.25, 28)},
	}
	bounds := gfx.Rect{Width: 32, Height: 32}
	for _, tc := range []struct {
		name         string
		clip         gfx.IconClip
		leftCoverage float64
	}{
		{name: "ends", clip: gfx.IconClip{Bounds: gfx.Rect{Y: 6, Width: 32, Height: 20}}, leftCoverage: 64},
		{name: "ends and side", clip: gfx.IconClip{Bounds: gfx.Rect{X: 12, Y: 6, Width: 2, Height: 20}}},
		{name: "rounded", clip: gfx.IconClip{Bounds: gfx.Rect{Y: 6, Width: 24, Height: 20}, Radius: 10}, leftCoverage: 64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver, err := app.NewDriver(app.Config{
				Title: "clipped stroke coverage", Width: 32, Height: 32, ClearColor: app.Color{Alpha: 1},
				Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
					canvas.DrawIconTransformed(icon, bounds, gfx.Color{Red: 1, Alpha: 1}, gfx.IconTransform{Clip: &tc.clip})
				},
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			defer driver.Close()
			pixels, err := renderer.render(driver.Snapshot(), 1)
			if err != nil {
				t.Fatal(err)
			}
			near := func(x, y int, want float64) {
				t.Helper()
				if got := pixels.RGBAAt(x, y).R; math.Abs(float64(got)-want) > 2 {
					t.Fatalf("stroke coverage at (%d,%d) = %d, want %g", x, y, got, want)
				}
			}
			// The 1px stroke spans x=11.75 to 12.75. The clipping side
			// at x=12 removes the quarter pixel on the left.
			for y := 6; y < 26; y++ {
				near(11, y, tc.leftCoverage)
				near(12, y, 191)
				near(13, y, 0)
			}
			for _, y := range []int{5, 26} {
				near(11, y, 0)
				near(12, y, 0)
			}
		})
	}
}
