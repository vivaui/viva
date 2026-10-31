package ui

import "testing"

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
