package gfx

import (
	"math"
	"testing"
)

// pathInkBounds returns the box that the triangles of an open path cover.
func pathInkBounds(t *testing.T, path Path, style PathStyle) Rect {
	t.Helper()
	var canvas Canvas
	canvas.DrawPath(path, Rect{Width: 32, Height: 32}, style)
	triangles := canvas.Triangles()
	if len(triangles) == 0 {
		t.Fatal("the path painted no geometry")
	}
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for _, triangle := range triangles {
		for _, p := range triangle.Points {
			minX = min(minX, p.X)
			minY = min(minY, p.Y)
			maxX = max(maxX, p.X)
			maxY = max(maxY, p.Y)
		}
	}
	return Rect{X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY}
}

// A square cap extends the stroke by half its width past the endpoint, at both
// ends of an open contour. Only the first end used to grow: the second end was
// given the negated outward direction, which put its square inside the stroke
// instead of past it. A round cap hid that, because a disc does not care which
// way it faces.
func TestSquareCapGrowsPastBothEndpoints(t *testing.T) {
	const (
		width  = float32(4)
		half   = width / 2
		x0     = float32(8)
		x1     = float32(24)
		middle = float32(16)
	)
	path := NewPath(32, 32,
		PathCommand{Kind: PathCommandMove, Point: Point{X: x0, Y: middle}},
		PathCommand{Kind: PathCommandLine, Point: Point{X: x1, Y: middle}},
	)
	bounds := pathInkBounds(t, path, PathStyle{
		Stroke:      Color{Alpha: 1},
		StrokeWidth: width,
		Cap:         CapSquare,
	})
	if got, want := bounds.X, x0-half; math.Abs(float64(got-want)) > 1e-3 {
		t.Errorf("the square cap does not extend past the start: ink starts at %v, want %v", got, want)
	}
	if got, want := bounds.X+bounds.Width, x1+half; math.Abs(float64(got-want)) > 1e-3 {
		t.Errorf("the square cap does not extend past the end: ink ends at %v, want %v", got, want)
	}
}

// The same contour with a butt cap ends exactly at the endpoints, which pins the
// measurement above: the difference is the cap, not the stroke width.
func TestButtCapEndsAtBothEndpoints(t *testing.T) {
	const (
		width  = float32(4)
		x0     = float32(8)
		x1     = float32(24)
		middle = float32(16)
	)
	path := NewPath(32, 32,
		PathCommand{Kind: PathCommandMove, Point: Point{X: x0, Y: middle}},
		PathCommand{Kind: PathCommandLine, Point: Point{X: x1, Y: middle}},
	)
	bounds := pathInkBounds(t, path, PathStyle{
		Stroke:      Color{Alpha: 1},
		StrokeWidth: width,
		Cap:         CapButt,
	})
	if got, want := bounds.X, x0; math.Abs(float64(got-want)) > 1e-3 {
		t.Errorf("the butt cap should start at the endpoint: ink starts at %v, want %v", got, want)
	}
	if got, want := bounds.X+bounds.Width, x1; math.Abs(float64(got-want)) > 1e-3 {
		t.Errorf("the butt cap should end at the endpoint: ink ends at %v, want %v", got, want)
	}
}
