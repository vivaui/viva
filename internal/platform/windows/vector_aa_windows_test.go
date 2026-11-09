package windows

import (
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestVectorBoundaryCoversTheOutsideHalfPixel(t *testing.T) {
	for _, scale := range []float64{1, 1.25, 1.5, 1.75, 2, 2.5, 3} {
		for _, reverse := range []bool{false, true} {
			t.Run(fmt.Sprintf("%gx/reverse=%t", scale, reverse), func(t *testing.T) {
				points := [3]scene.Point{{X: 10.75, Y: 4}, {X: 30, Y: 4}, {X: 10.75, Y: 28}}
				for i := range points {
					points[i].X /= float32(scale)
					points[i].Y /= float32(scale)
				}
				if reverse {
					points[1], points[2] = points[2], points[1]
				}
				pixels := clipScene(t, 32, scale, []render.Rectangle{{Kind: scene.KindTriangle, Points: points, EdgeMask: 7, Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}})
				// The pixel's center is outside, but one quarter of its area is inside.
				if got := pixels.RGBAAt(10, 10).R; math.Abs(float64(got)-64) > 3 {
					t.Fatalf("outside coverage = %d, want about 64", got)
				}
				if got := pixels.RGBAAt(9, 10).R; got != 0 {
					t.Fatalf("fringe wider than half a device pixel: %d", got)
				}
			})
		}
	}
}

func vectorCoverageScene() []render.Rectangle {
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(20, 12), gfx.CubicTo(20, 16.42, 16.42, 20, 12, 20), gfx.CubicTo(7.58, 20, 4, 16.42, 4, 12),
		gfx.CubicTo(4, 7.58, 7.58, 4, 12, 4), gfx.CubicTo(16.42, 4, 20, 7.58, 20, 12), gfx.ClosePath(),
		gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
	}}
	var canvas gfx.Canvas
	canvas.SetPixelScale(1.5)
	for i := range 100 {
		canvas.DrawIcon(icon, gfx.Rect{X: float32(i%10)*20 + 2, Y: float32(i/10)*20 + 2, Width: 16, Height: 16}, gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
	}
	rectangles := make([]render.Rectangle, 0, len(canvas.Triangles()))
	for _, triangle := range canvas.Triangles() {
		r := render.Rectangle{Kind: scene.KindTriangle, Color: scene.Color(triangle.Color), EdgeMask: triangle.EdgeMask}
		for i, p := range triangle.Points {
			r.Points[i] = scene.Point{X: p.X, Y: p.Y}
		}
		rectangles = append(rectangles, r)
	}
	return rectangles
}

// Includes GPU completion/readback, rather than timing only command submission.
func BenchmarkVectorCoverageFrame(b *testing.B) {
	offscreen := newOffscreen(b, 204, 204, 1.5)
	if err := offscreen.Set(vectorCoverageScene(), nil); err != nil {
		b.Fatal(err)
	}
	if _, err := offscreen.Image(); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := offscreen.Image(); err != nil {
			b.Fatal(err)
		}
	}
}
