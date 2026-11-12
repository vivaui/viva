package gfx

import (
	"fmt"
	"testing"
)

func TestFillQuadFollowsConcaveOutline(t *testing.T) {
	for corner := range 4 {
		for _, reversed := range []bool{false, true} {
			t.Run(fmt.Sprintf("corner=%d/reversed=%t", corner, reversed), func(t *testing.T) {
				quad := Quad{Points: [4]Point{{}, {X: 100}, {Y: 100}, {X: 100, Y: 100}}}
				quad.Points[corner] = Point{X: 80, Y: 80}
				if corner&1 != 0 {
					quad.Points[corner].X = 20
				}
				if corner&2 != 0 {
					quad.Points[corner].Y = 20
				}
				if reversed {
					quad.Points[1], quad.Points[2] = quad.Points[2], quad.Points[1]
				}
				for index := range quad.Tint {
					quad.Tint[index] = Color{Alpha: 0.5}
				}
				var canvas Canvas
				canvas.FillQuad(quad)
				for row := 0; row < 100; row += 3 {
					for column := 0; column < 100; column += 3 {
						point := Point{X: float32(column) + 0.37, Y: float32(row) + 0.61}
						inside := false
						for index, first := range quadWinding {
							second := quadWinding[(index+1)%len(quadWinding)]
							start, end := quad.Points[first], quad.Points[second]
							if (start.Y > point.Y) != (end.Y > point.Y) && point.X < (end.X-start.X)*(point.Y-start.Y)/(end.Y-start.Y)+start.X {
								inside = !inside
							}
						}
						coverage := 0
						for _, triangle := range canvas.Triangles() {
							if pointInTriangle(point, triangle.Points[0], triangle.Points[1], triangle.Points[2]) {
								coverage++
							}
						}
						if (coverage == 1) != inside || coverage > 1 {
							t.Fatalf("point %+v: outline inside=%t, fill coverage=%d", point, inside, coverage)
						}
					}
				}
			})
		}
	}
}

func BenchmarkFillQuad(b *testing.B) {
	for _, concave := range []bool{false, true} {
		b.Run(fmt.Sprintf("concave=%t", concave), func(b *testing.B) {
			quad := Quad{Points: [4]Point{{}, {X: 100}, {Y: 100}, {X: 100, Y: 100}}}
			if concave {
				quad.Points[3] = Point{X: 20, Y: 20}
			}
			for index := range quad.Tint {
				quad.Tint[index] = Color{Alpha: 0.5}
			}
			var canvas Canvas
			canvas.FillQuad(quad)
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				canvas.Reset()
				canvas.FillQuad(quad)
			}
		})
	}
}
