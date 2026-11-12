package gfx

import (
	"math"
	"testing"
)

func TestFillLinearGradientClampingClipAndScale(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		var c Canvas
		c.SetPixelScale(scale)
		clip := Rect{X: 2, Y: 3, Width: 100, Height: 80}
		c.Clip(clip)
		c.PushLayer(3)
		c.Fill(Rect{Width: 1, Height: 1}, Color{Alpha: 1})
		g := LinearGradient{Start: Point{X: 20, Y: 0}, End: Point{X: 80, Y: 0}, From: Color{Red: 1, Alpha: 1}, To: Color{Blue: 1, Alpha: 1}}
		c.FillLinearGradient(Rect{X: .2, Y: .3, Width: 100, Height: 60}, g)
		if len(c.Triangles()) == 0 {
			t.Fatal("no gradient")
		}
		for _, tri := range c.Triangles() {
			if !tri.HasClip || tri.ClipOf != clip || tri.Layer != 3 || tri.Order <= c.Rectangles()[0].Order {
				t.Fatalf("lost order/clip %+v", tri)
			}
			for i, p := range tri.Points {
				want := g.At((p.X - 20) / 60)
				got := tri.Color
				if tri.Shaded && i > 0 {
					got = tri.Shade[i-1]
				}
				if math.Abs(float64(got.Red-want.Red)) > 1e-5 || math.Abs(float64(got.Blue-want.Blue)) > 1e-5 {
					t.Fatalf("wrong color at %+v: %+v != %+v", p, got, want)
				}
			}
		}
	}
}

func TestFillLinearGradientZeroLengthAndAllocations(t *testing.T) {
	var c Canvas
	g := LinearGradient{To: Color{Green: 1, Alpha: 1}}
	paint := func() { c.Reset(); c.FillLinearGradient(Rect{Width: 100, Height: 100}, g) }
	paint()
	for _, tri := range c.Triangles() {
		for _, color := range tri.Shade {
			if !tri.Shaded {
				color = tri.Color
			}
			if color != g.To {
				t.Fatal(color)
			}
		}
	}
	if n := testing.AllocsPerRun(100, paint); n != 0 {
		t.Fatal(n)
	}
}
