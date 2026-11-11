package gfx

import "math"

// IsGeometricShape reports whether r is one of the Geometric Shapes a fixed-pitch line draws as
// geometry: the corner triangles of U+25E2 to U+25E5 and the outlined ones of U+25F8 to U+25FA
// and U+25FF. Their ink has to reach the cell's corners, which a font's glyph does not once the
// cell is taller than its line box.
func IsGeometricShape(r rune) bool {
	return (r >= 0x25E2 && r <= 0x25E5) || (r >= 0x25F8 && r <= 0x25FA) || r == 0x25FF
}

// shapeCorner is the corner of the cell a triangle's right angle sits in.
type shapeCorner uint8

const (
	cornerTopLeft shapeCorner = iota
	cornerTopRight
	cornerBottomLeft
	cornerBottomRight
)

var shapeCorners = map[rune]shapeCorner{
	0x25E4: cornerTopLeft,     // ◤
	0x25E5: cornerTopRight,    // ◥
	0x25E3: cornerBottomLeft,  // ◣
	0x25E2: cornerBottomRight, // ◢
	0x25F8: cornerTopLeft,     // ◸
	0x25F9: cornerTopRight,    // ◹
	0x25FA: cornerBottomLeft,  // ◺
	0x25FF: cornerBottomRight, // ◿
}

// geometricShape draws r in the cell: a filled corner triangle is one triangle, so its hypotenuse
// is a straight edge a renderer feathers like any other; an outlined one is the band between that
// triangle and the same one inset by a rule.
func (g *cellGlyph) geometricShape(r rune) {
	corner, ok := shapeCorners[r]
	if !ok {
		return
	}
	if r >= 0x25F8 {
		g.outlineTriangle(corner)
		return
	}
	points := g.triangleCorners(corner)
	g.fill(points[0], points[1], points[2])
}

// triangleCorners is the triangle a corner names: its right angle first, then the two cell
// corners along the edges it sits on.
func (g cellGlyph) triangleCorners(corner shapeCorner) [3]Point {
	width, height := float32(g.width), float32(g.height)
	x, y := float32(0), float32(0)
	if corner == cornerTopRight || corner == cornerBottomRight {
		x = width
	}
	if corner == cornerBottomLeft || corner == cornerBottomRight {
		y = height
	}
	return [3]Point{{X: x, Y: y}, {X: width - x, Y: y}, {X: x, Y: height - y}}
}

// outlineTriangle fills the band between a corner triangle and the same triangle moved in by one
// rule: an outline as wide as a rule, whose ink stops exactly at the cell's edges, the way
// ghostty's inner stroke of the same triangle does. The inner contour is a hole, so the two are
// filled as even-odd.
func (g *cellGlyph) outlineTriangle(corner shapeCorner) {
	points := g.triangleCorners(corner)
	inner := insetTriangle(points, float32(g.rule))
	width, height := float32(g.width), float32(g.height)
	g.fillPath(NewPath(width, height,
		MoveTo(points[0].X, points[0].Y),
		LineTo(points[1].X, points[1].Y),
		LineTo(points[2].X, points[2].Y),
		ClosePath(),
		MoveTo(inner[0].X, inner[0].Y),
		LineTo(inner[1].X, inner[1].Y),
		LineTo(inner[2].X, inner[2].Y),
		ClosePath()), PathFillEvenOdd)
}

// insetTriangle shrinks a triangle so that each of its edges comes in by inset: the hole an
// outline one rule wide leaves. Each corner moves toward the incenter — the point every edge is
// the same distance from — by the share of that distance which spends inset, and moving toward it
// moves every edge in by the same share, so the hole keeps the outline's even width.
func insetTriangle(points [3]Point, inset float32) [3]Point {
	side := func(from, to Point) float32 {
		return float32(math.Hypot(float64(to.X-from.X), float64(to.Y-from.Y)))
	}
	// A corner weighted by the side opposite it is the incenter.
	a, b, c := side(points[1], points[2]), side(points[2], points[0]), side(points[0], points[1])
	perimeter := a + b + c
	centre := Point{
		X: (a*points[0].X + b*points[1].X + c*points[2].X) / perimeter,
		Y: (a*points[0].Y + b*points[1].Y + c*points[2].Y) / perimeter,
	}
	// Twice the area over the perimeter is the distance from the incenter to an edge.
	twiceArea := math.Abs(float64(
		(points[1].X-points[0].X)*(points[2].Y-points[0].Y) - (points[2].X-points[0].X)*(points[1].Y-points[0].Y)))
	scale := max(1-inset*perimeter/float32(twiceArea), 0)
	var result [3]Point
	for index := range points {
		result[index] = Point{
			X: centre.X + scale*(points[index].X-centre.X),
			Y: centre.Y + scale*(points[index].Y-centre.Y),
		}
	}
	return result
}
