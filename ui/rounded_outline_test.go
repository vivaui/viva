package ui

import (
	"math"
	"testing"
)

func TestRoundedOutlineHasNoOverlappingFaces(t *testing.T) {
	for _, radius := range []float32{0, .5, 2, 8, 20, 100} {
		for _, width := range []float32{.5, 1, 4, 12} {
			var canvas Canvas
			canvas.RoundedOutline(Rect{X: 5, Y: 7, Width: 80, Height: 40}, radius, width, Color{Alpha: .5})
			triangles := canvas.Triangles()
			boundary := 0
			for i, triangle := range triangles {
				for edge := uint8(0); edge < 3; edge++ {
					if triangle.EdgeMask&(1<<edge) != 0 {
						boundary++
					}
				}
				for _, other := range triangles[i+1:] {
					if trianglesOverlap(triangle, other) {
						t.Fatalf("radius %g width %g: overlapping faces", radius, width)
					}
				}
			}
			assertBoundaryMasks(t, triangles, boundary)
			r := float64(min(radius, 20))
			inner := max(0, r-float64(width))
			want := 80*40 - (4-math.Pi)*r*r - (float64(80-2*width)*float64(40-2*width) - (4-math.Pi)*inner*inner)
			if got := triangleCoverArea(triangles); math.Abs(got-want) > 15 {
				t.Fatalf("radius %g width %g: area %g, want near %g", radius, width, got, want)
			}
		}
	}
}

func TestRoundedOutlineLeavesTheInteriorUnpainted(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		var canvas Canvas
		canvas.SetPixelScale(scale)
		canvas.RoundedOutline(Rect{X: 10, Y: 10, Width: 200, Height: 40}, 8, 1, Color{Red: 1, Alpha: 1})
		if len(canvas.Triangles()) == 0 {
			t.Fatal("outline has no geometry")
		}
		edge := false
		for _, triangle := range canvas.Triangles() {
			if pointInTriangle(Point{X: 110, Y: 30}, triangle.Points[0], triangle.Points[1], triangle.Points[2]) {
				t.Fatalf("scale %v: border covers the content", scale)
			}
			edge = edge || pointInTriangle(Point{X: 110, Y: 10.5}, triangle.Points[0], triangle.Points[1], triangle.Points[2])
		}
		if !edge {
			t.Fatalf("scale %v: outline is missing its top edge", scale)
		}
	}
}
