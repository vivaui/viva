package web

import "github.com/vivaui/viva/gfx"

// shadeAxis turns a shaded triangle into what Canvas 2D can paint, a linear
// gradient: it runs from start, where the color is from, to end, where it is to.
// A color that varies linearly across a triangle is such a gradient exactly when
// its three corner colors lie on one line in color space, which is what the
// canvas's gradients produce; from and to are then the two colors farthest
// apart, so the third lies between them and the gradient's flat ends are never
// reached inside the triangle. For any other three colors it is the nearest such
// line. ok is false when the triangle is effectively flat or degenerate and its
// first color serves.
func shadeAxis(points [3]gfx.Point, colors [3]gfx.Color) (start, end gfx.Point, from, to gfx.Color, ok bool) {
	first, second := 0, 1
	for _, pair := range [...][2]int{{0, 2}, {1, 2}} {
		if colorDistance(colors[pair[0]], colors[pair[1]]) > colorDistance(colors[first], colors[second]) {
			first, second = pair[0], pair[1]
		}
	}
	third := 3 - first - second
	from, to = colors[first], colors[second]
	span := colorDistance(from, to)
	if span < 1e-8 {
		return start, end, colors[0], colors[0], false
	}
	// s is how far the third corner's color lies from from toward to; it is 0 at
	// the first corner and 1 at the second.
	sThird := colorDot(colors[third], from, to) / span
	e1x, e1y := points[second].X-points[first].X, points[second].Y-points[first].Y
	e2x, e2y := points[third].X-points[first].X, points[third].Y-points[first].Y
	area := e1x*e2y - e1y*e2x
	if area > -1e-6 && area < 1e-6 {
		return start, end, colors[0], colors[0], false
	}
	// The gradient of the affine function with those values over the corners.
	gx := (e2y - sThird*e1y) / area
	gy := (sThird*e1x - e2x) / area
	length := gx*gx + gy*gy
	if length < 1e-12 {
		return start, end, colors[0], colors[0], false
	}
	start = points[first]
	end = gfx.Point{X: start.X + gx/length, Y: start.Y + gy/length}
	return start, end, from, to, true
}

// colorDistance is the squared distance between two colors, alpha included.
func colorDistance(a, b gfx.Color) float32 { return colorDot(b, a, b) }

// colorDot is (color-from)·(to-from).
func colorDot(color, from, to gfx.Color) float32 {
	return (color.Red-from.Red)*(to.Red-from.Red) + (color.Green-from.Green)*(to.Green-from.Green) +
		(color.Blue-from.Blue)*(to.Blue-from.Blue) + (color.Alpha-from.Alpha)*(to.Alpha-from.Alpha)
}
