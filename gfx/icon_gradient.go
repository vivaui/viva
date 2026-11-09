package gfx

// gradientAxis is a LinearGradient placed in canvas coordinates: t(p) is 0 at the
// start, 1 at the end, projected onto the line between them.
type gradientAxis struct {
	origin Point
	dx, dy float32 // t = (p-origin)·(dx,dy) + base
	base   float32 // 1 for a zero-length gradient, which paints its last color
}

// newGradientAxis places gradient, given in view-box units, into target; sx and
// sy are the view-box to target scale along each axis.
func newGradientAxis(gradient LinearGradient, target Rect, sx, sy float32) gradientAxis {
	axis := gradientAxis{origin: Point{X: target.X + gradient.Start.X*sx, Y: target.Y + gradient.Start.Y*sy}}
	vx, vy := (gradient.End.X-gradient.Start.X)*sx, (gradient.End.Y-gradient.Start.Y)*sy
	if length := vx*vx + vy*vy; length > 1e-12 {
		axis.dx, axis.dy = vx/length, vy/length
	} else {
		axis.base = 1
	}
	return axis
}

func (axis gradientAxis) t(point Point) float32 {
	return (point.X-axis.origin.X)*axis.dx + (point.Y-axis.origin.Y)*axis.dy + axis.base
}

// vertexT is the gradient position at each corner, held to [0,1]. A corner a hair
// off an end, as a cut leaves it, is put on it, so a piece beyond an end is flat.
func (axis gradientAxis) vertexT(points [3]Point) [3]float32 {
	var t [3]float32
	for index, point := range points {
		value := axis.t(point)
		switch {
		case value < 1e-4:
			value = 0
		case value > 1-1e-4:
			value = 1
		}
		t[index] = value
	}
	return t
}

// gradientVertex is one corner of a polygon being cut; boundary says the edge
// from it to the next corner lies on the shape's outer boundary.
type gradientVertex struct {
	point    Point
	boundary bool
}

// gradientPolygon is a convex polygon. A triangle cut by two parallel lines never
// has more than five corners.
type gradientPolygon struct {
	corners [8]gradientVertex
	count   int
}

func (polygon *gradientPolygon) add(point Point, boundary bool) {
	if polygon.count < len(polygon.corners) {
		polygon.corners[polygon.count] = gradientVertex{point: point, boundary: boundary}
		polygon.count++
	}
}

// split cuts the polygon along t = level into the part at or below it and the
// part at or above it. Corners on the line belong to both. The new edge along the
// line is interior; pieces of the old edges keep their boundary flag.
func (polygon gradientPolygon) split(axis gradientAxis, level float32) (below, above gradientPolygon) {
	for index := 0; index < polygon.count; index++ {
		current := polygon.corners[index]
		next := polygon.corners[(index+1)%polygon.count]
		currentD, nextD := axis.t(current.point)-level, axis.t(next.point)-level
		belowCurrent, belowNext := currentD <= 0, nextD <= 0
		aboveCurrent, aboveNext := currentD >= 0, nextD >= 0
		var crossing Point
		if belowCurrent != belowNext || aboveCurrent != aboveNext {
			u := currentD / (currentD - nextD)
			crossing = Point{X: current.point.X + (next.point.X-current.point.X)*u, Y: current.point.Y + (next.point.Y-current.point.Y)*u}
		}
		if belowCurrent {
			below.add(current.point, current.boundary)
			if !belowNext {
				below.add(crossing, false)
			}
		} else if belowNext {
			below.add(crossing, current.boundary)
		}
		if aboveCurrent {
			above.add(current.point, current.boundary)
			if !aboveNext {
				above.add(crossing, false)
			}
		} else if aboveNext {
			above.add(crossing, current.boundary)
		}
	}
	return below, above
}

// each cuts one triangle where the gradient starts and ends and calls emit with
// the pieces: one that lies wholly before the start, one between the ends, one
// beyond the end, each a triangle, or several when it has more corners. A triangle
// that does not cross an end comes out whole. The pieces tile the triangle
// exactly.
func (axis gradientAxis) each(points [3]Point, edges uint8, emit func([3]Point, uint8)) {
	t := [3]float32{axis.t(points[0]), axis.t(points[1]), axis.t(points[2])}
	low, high := min(t[0], t[1], t[2]), max(t[0], t[1], t[2])
	if low >= 0 && high <= 1 || high <= 0 || low >= 1 {
		emit(points, edges)
		return
	}
	polygon := gradientPolygon{count: 3}
	for index := range points {
		polygon.corners[index] = gradientVertex{point: points[index], boundary: edges>>index&1 != 0}
	}
	before, rest := polygon.split(axis, 0)
	middle, after := rest.split(axis, 1)
	before.emitTriangles(emit)
	middle.emitTriangles(emit)
	after.emitTriangles(emit)
}

// emitTriangles fans the polygon into triangles, skipping slivers and merging
// corners the cut left in the same place.
func (polygon gradientPolygon) emitTriangles(emit func([3]Point, uint8)) {
	// A corner that coincides with the next one is the cut's duplicate of it; the
	// later one carries the right outgoing edge.
	count := 0
	for index := 0; index < polygon.count; index++ {
		next := polygon.corners[(index+1)%polygon.count]
		if samePoint(polygon.corners[index].point, next.point) {
			continue
		}
		polygon.corners[count] = polygon.corners[index]
		count++
	}
	if count < 3 {
		return
	}
	for index := 1; index+1 < count; index++ {
		a, b, c := polygon.corners[0], polygon.corners[index], polygon.corners[index+1]
		cross := (b.point.X-a.point.X)*(c.point.Y-a.point.Y) - (b.point.Y-a.point.Y)*(c.point.X-a.point.X)
		if cross > -1e-5 && cross < 1e-5 {
			continue
		}
		var edges uint8
		if index == 1 && a.boundary {
			edges |= 1
		}
		if b.boundary {
			edges |= 2
		}
		if index+2 == count && polygon.corners[count-1].boundary {
			edges |= 4
		}
		emit([3]Point{a.point, b.point, c.point}, edges)
	}
}

func samePoint(a, b Point) bool {
	return abs32(a.X-b.X) < 1e-4 && abs32(a.Y-b.Y) < 1e-4
}

// colorize sets the colors of triangle from the gradient at its corners t:
// a flat triangle where they agree, a shaded one where they do not. alpha is the
// icon tint's opacity.
func colorize(triangle *Triangle, gradient *LinearGradient, t [3]float32, alpha float32) {
	triangle.Color = fade(gradient.At(t[0]), alpha)
	triangle.Shaded = t[1] != t[0] || t[2] != t[0]
	if triangle.Shaded {
		triangle.Shade = [2]Color{fade(gradient.At(t[1]), alpha), fade(gradient.At(t[2]), alpha)}
	}
}

func fade(color Color, alpha float32) Color {
	color.Alpha *= alpha
	return color
}

// visible reports whether any corner of triangle paints something.
func (triangle *Triangle) visible() bool {
	return triangle.Color.Alpha > 0 || triangle.Shaded && (triangle.Shade[0].Alpha > 0 || triangle.Shade[1].Alpha > 0)
}

// shadeGradient replaces the triangles from start on, which the canvas painted
// for one gradient layer in a flat color, with the same area cut where the
// gradient starts and ends and colored from it.
func (canvas *Canvas) shadeGradient(start int, axis gradientAxis, gradient *LinearGradient, alpha float32) {
	source := append(canvas.gradientSource[:0], canvas.triangles[start:]...)
	canvas.gradientSource = source
	canvas.triangles = canvas.triangles[:start]
	for _, triangle := range source {
		axis.each(triangle.Points, triangle.EdgeMask, func(points [3]Point, edges uint8) {
			shaded := triangle
			shaded.Points, shaded.EdgeMask = points, edges
			colorize(&shaded, gradient, axis.vertexT(points), alpha)
			if shaded.visible() {
				canvas.triangles = append(canvas.triangles, shaded)
			}
		})
	}
}
