//go:build darwin || windows || linux

package headless

import (
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"
)

// These almost collinear faces came from an ordinary filled icon and from an
// area chart's gradient fill, whose earcut triangles lie along a flat stretch of
// the curve. Translating them changes float32 rounding; a test at the origin
// misses the remote spikes.
func TestVectorSliverCoverageStaysLocal(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	r := newRenderer()
	defer r.close()
	faces := [][3]ui.Point{
		{{X: 830.1327, Y: 356.20184}, {X: 831.76605, Y: 354.5925}, {X: 832.00903, Y: 354.35315}},
		{{X: 830.1322, Y: 356.5373}, {X: 829.31714, Y: 355.7349}, {X: 829.0782, Y: 355.49963}},
		{{X: 1031.0417, Y: 230.069}, {X: 1032, Y: 231.344}, {X: 1032.9583, Y: 232.619}},
		{{X: 500, Y: 300}, {X: 501, Y: 300.001}, {X: 502, Y: 300}},
	}
	for _, scale := range []float64{1, 1.1, 1.25, 1.3, 1.5, 1.75, 2, 2.5, 3} {
		for face, points := range faces {
			t.Run(fmt.Sprintf("%gx/face%d", scale, face), func(t *testing.T) {
				tri := app.Primitive{Kind: app.TrianglePrimitive, Points: points, EdgeMask: 1, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
				pixels, err := r.render(app.Snapshot{Width: 1200, Height: 950, Clear: app.Color{Alpha: 1}, Primitives: []app.Primitive{tri}}, scale)
				if err != nil {
					t.Fatal(err)
				}
				left, top, right, bottom := float64(points[0].X), float64(points[0].Y), float64(points[0].X), float64(points[0].Y)
				for _, p := range points[1:] {
					left, top = math.Min(left, float64(p.X)), math.Min(top, float64(p.Y))
					right, bottom = math.Max(right, float64(p.X)), math.Max(bottom, float64(p.Y))
				}
				for y := 0; y < pixels.Rect.Dy(); y++ {
					for x := 0; x < pixels.Rect.Dx(); x++ {
						px, py := (float64(x)+0.5)/scale, (float64(y)+0.5)/scale
						if px >= left-1/scale && px <= right+1/scale && py >= top-1/scale && py <= bottom+1/scale {
							continue
						}
						if p := pixels.RGBAAt(x, y); p.R > 4 || p.G > 4 || p.B > 4 {
							t.Fatalf("remote coverage at (%d,%d): %v", x, y, p)
						}
					}
				}
			})
		}
	}
}

// Thin and regular faces must agree on shared-edge ownership. Alpha exposes
// overlaps as well as cracks, including after non-integral translations.
func TestVectorSliverSharedEdgesAndGradients(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	r := newRenderer()
	defer r.close()
	for _, scale := range []float64{1, 1.1, 1.25, 1.3, 1.5, 1.75, 2, 2.5, 3} {
		for _, offset := range []ui.Point{{X: 0, Y: 0}, {X: 401.13, Y: 211.29}, {X: -20.25, Y: -20.25}} {
			for _, height := range []float32{2, 32} {
				for _, reverse := range []bool{false, true} {
					t.Run(fmt.Sprintf("%gx/offset%v/height%g/reverse%v", scale, offset, height, reverse), func(t *testing.T) {
						// A trapezoid: a thin upper face and either a thin or regular
						// lower face share the diagonal from a to c.
						a, b := ui.Point{X: 20.5, Y: 20.5}, ui.Point{X: 100.5, Y: 20.5}
						c, d := ui.Point{X: 100.5, Y: 22.5}, ui.Point{X: 20.5, Y: 20.5 + height}
						for _, point := range []*ui.Point{&a, &b, &c, &d} {
							point.X += offset.X
							point.Y += offset.Y
						}
						colorAt := func(p ui.Point) ui.Color {
							x := (p.X - a.X) / (b.X - a.X)
							return ui.Color{Red: 1 - x, Blue: x, Alpha: 0.5}
						}
						tri := func(points [3]ui.Point, mask uint8) app.Primitive {
							if reverse {
								points[1], points[2] = points[2], points[1]
								mask = (mask&1)<<2 | mask&2 | (mask&4)>>2
							}
							return app.Primitive{Kind: app.TrianglePrimitive, Points: points, EdgeMask: mask,
								Color: colorAt(points[0]), Shaded: true, Shade: [2]ui.Color{colorAt(points[1]), colorAt(points[2])}}
						}
						pixels, err := r.render(app.Snapshot{Width: 525, Height: 290, Clear: app.Color{Alpha: 1}, Primitives: []app.Primitive{
							tri([3]ui.Point{a, b, c}, 3), tri([3]ui.Point{a, c, d}, 6),
						}}, scale)
						if err != nil {
							t.Fatal(err)
						}
						checked := 0
						for y := 0; y < pixels.Rect.Dy(); y++ {
							for x := 0; x < pixels.Rect.Dx(); x++ {
								px, py := (float64(x)+0.5)/scale, (float64(y)+0.5)/scale
								bottom := float64(d.Y) + (float64(c.Y)-float64(d.Y))*(px-float64(a.X))/80
								if px <= float64(a.X)+1/scale || px >= float64(b.X)-1/scale || py <= float64(a.Y)+0.5/scale || py >= bottom-0.5/scale {
									continue
								}
								got := pixels.RGBAAt(x, y)
								wantBlue := 127.5 * (px - float64(a.X)) / 80
								if math.Abs(float64(got.B)-wantBlue) > 4 || math.Abs(float64(got.R)-(127.5-wantBlue)) > 4 || got.G > 4 {
									t.Fatalf("shared mesh at (%d,%d) = %v, want red %.1f blue %.1f", x, y, got, 127.5-wantBlue, wantBlue)
								}
								checked++
							}
						}
						if checked == 0 {
							t.Fatal("no interior samples checked")
						}
					})
				}
			}
		}
	}
}

func TestVectorInvalidFacesStayEmpty(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	if runtime.GOOS == "linux" {
		// The Vulkan backend validates its primitives and rejects a non-finite
		// one outright, so nothing reaches the rasterizer to be tested.
		t.Skip("the Vulkan backend rejects invalid primitives instead of drawing nothing")
	}
	r := newRenderer()
	defer r.close()
	for _, points := range [][3]ui.Point{
		{{X: 4, Y: 4}, {X: 4, Y: 4}, {X: 4, Y: 4}},
		{{X: 4, Y: 4}, {X: 12, Y: 12}, {X: 20, Y: 20}},
		{{X: 4, Y: 4}, {X: float32(math.NaN()), Y: 12}, {X: 20, Y: 20}},
		{{X: 4, Y: 4}, {X: 12, Y: float32(math.Inf(1))}, {X: 20, Y: 20}},
		{{X: -1e30, Y: -1e30}, {X: 1e30, Y: -1e30}, {X: 0, Y: 1e30}},
	} {
		for _, mask := range []uint8{0, 1, 7} {
			pixels, err := r.render(app.Snapshot{Width: 41, Height: 37, Clear: app.Color{Alpha: 1}, Primitives: []app.Primitive{{
				Kind: app.TrianglePrimitive, Points: points, EdgeMask: mask, Color: ui.Color{Red: 1, Alpha: 1},
			}}}, 2)
			if err != nil {
				t.Fatal(err)
			}
			for y := 0; y < pixels.Rect.Dy(); y++ {
				for x := 0; x < pixels.Rect.Dx(); x++ {
					if p := pixels.RGBAAt(x, y); p.R != 0 || p.G != 0 || p.B != 0 {
						t.Fatalf("invalid face %v mask %d painted at (%d,%d): %v", points, mask, x, y, p)
					}
				}
			}
		}
	}
}

func TestVectorSliverMasksAndClipping(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	if runtime.GOOS == "linux" {
		// Out-of-range edge masks are a validation error there, not a no-op.
		t.Skip("the Vulkan backend rejects edge masks above 7")
	}
	r := newRenderer()
	defer r.close()
	for _, scale := range []float64{1, 1.5, 3} {
		for _, mask := range []uint8{0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 255} {
			for _, reverse := range []bool{false, true} {
				points := [3]ui.Point{{X: -10, Y: -10}, {X: 90, Y: 50}, {X: 90.01, Y: 50.01}}
				if reverse {
					points[1], points[2] = points[2], points[1]
				}
				clip := ui.Rect{X: 10, Y: 10, Width: 40, Height: 30}
				pixels, err := r.render(app.Snapshot{Width: 100, Height: 75, Clear: app.Color{Alpha: 1}, Primitives: []app.Primitive{{
					Kind: app.TrianglePrimitive, Points: points, EdgeMask: mask, Color: ui.Color{Red: 1, Alpha: 1}, Clipped: true, Clip: clip,
				}}}, scale)
				if err != nil {
					t.Fatal(err)
				}
				painted := false
				for y := 0; y < pixels.Rect.Dy(); y++ {
					for x := 0; x < pixels.Rect.Dx(); x++ {
						p := pixels.RGBAAt(x, y)
						if p.R == 0 {
							continue
						}
						painted = true
						px, py := (float64(x)+0.5)/scale, (float64(y)+0.5)/scale
						if px < 10 || px >= 50 || py < 10 || py >= 40 {
							t.Fatalf("scale %g mask %d reverse %v escaped clip at (%d,%d): %v", scale, mask, reverse, x, y, p)
						}
					}
				}
				// A subpixel face with all three silhouette edges still needs AA;
				// dropping every thin triangle would pass an emptiness-only test.
				if mask&7 == 7 && !painted {
					t.Fatalf("scale %g reverse %v: visible sliver disappeared", scale, reverse)
				}
			}
		}
	}
}

func BenchmarkVectorFrame(b *testing.B) {
	if !rendererAvailable {
		b.Skip("native screenshots need the platform headless renderer")
	}
	for _, mask := range []uint8{1, 7} {
		for _, height := range []float32{0.01, 20} {
			b.Run(fmt.Sprintf("mask%d/height%g", mask, height), func(b *testing.B) {
				r := newRenderer()
				defer r.close()
				snapshot := app.Snapshot{Width: 640, Height: 480, Clear: app.Color{Alpha: 1}}
				for i := 0; i < 256; i++ {
					x, y := float32(i%16)*38, float32(i/16)*28
					snapshot.Primitives = append(snapshot.Primitives, app.Primitive{Kind: app.TrianglePrimitive, EdgeMask: mask,
						Points: [3]ui.Point{{X: x + 2, Y: y + 2}, {X: x + 32, Y: y + 22}, {X: x + 32, Y: y + 22 + height}}, Color: ui.Color{Red: 1, Alpha: 1}})
				}
				if _, err := r.render(snapshot, 2); err != nil {
					b.Fatal(err)
				}
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if _, err := r.render(snapshot, 2); err != nil {
						b.Fatal(err)
					}
				}
			})
		}
	}
}
