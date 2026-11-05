package ui

import "math"

// iconClipMaxPoints bounds a clip polygon: a triangle clipped by a rounded
// rectangle or circle has at most three corners plus the shape's own.
const iconClipMaxPoints = 40

// iconClipShape is a convex clip polygon in canvas coordinates, in order.
type iconClipShape struct {
	points [iconClipMaxPoints]Point
	count  int
}

// iconClipVertex is one corner of a clipped triangle. boundary says the edge to
// the next corner lies on the source triangle's outer edge and must be
// feathered; an edge made by the clip is interior.
type iconClipVertex struct {
	point    Point
	boundary bool
	// weights are the vertex's barycentric coordinates in the source triangle,
	// so a gradient is interpolated at a cut's corners as at the source's.
	weights [3]float32
}

// iconClipPolygon is a convex intermediate polygon for one clipping pass.
type iconClipPolygon struct {
	vertices [iconClipMaxPoints]iconClipVertex
	count    int
}

func (polygon *iconClipPolygon) add(point Point, boundary bool, weights [3]float32) {
	if polygon.count < len(polygon.vertices) {
		polygon.vertices[polygon.count] = iconClipVertex{point: point, boundary: boundary, weights: weights}
		polygon.count++
	}
}

// iconClipShapeAt places clip, given in an icon's view box, into the canvas
// through place.
func iconClipShapeAt(clip IconClip, units iconUnits, place iconPlacement) iconClipShape {
	var shape iconClipShape
	bounds := clip.Bounds
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return shape
	}
	radius := min(max(clip.Radius, 0), min(bounds.Width, bounds.Height)/2)
	add := func(point Point) {
		if shape.count < len(shape.points) {
			local := units.point(point)
			shape.points[shape.count] = place.apply(local)
			shape.count++
		}
	}
	if radius <= 0 {
		add(Point{X: bounds.X, Y: bounds.Y})
		add(Point{X: bounds.X + bounds.Width, Y: bounds.Y})
		add(Point{X: bounds.X + bounds.Width, Y: bounds.Y + bounds.Height})
		add(Point{X: bounds.X, Y: bounds.Y + bounds.Height})
		return shape
	}

	// A half-pixel arc chord is below what the eye resolves at the sizes an icon
	// is painted; six steps per corner are enough at 96px and stay cheap at 16.
	const steps = 6
	corners := [...]struct {
		x, y     float64
		from, to float64
	}{
		{float64(bounds.X + bounds.Width - radius), float64(bounds.Y + radius), -math.Pi / 2, 0},
		{float64(bounds.X + bounds.Width - radius), float64(bounds.Y + bounds.Height - radius), 0, math.Pi / 2},
		{float64(bounds.X + radius), float64(bounds.Y + bounds.Height - radius), math.Pi / 2, math.Pi},
		{float64(bounds.X + radius), float64(bounds.Y + radius), math.Pi, 3 * math.Pi / 2},
	}
	for _, corner := range corners {
		for step := 0; step <= steps; step++ {
			angle := corner.from + (corner.to-corner.from)*float64(step)/steps
			add(Point{
				X: float32(corner.x + float64(radius)*math.Cos(angle)),
				Y: float32(corner.y + float64(radius)*math.Sin(angle)),
			})
		}
	}
	return shape
}

// clips reports whether the shape has an area to clip to.
func (shape iconClipShape) clips() bool {
	return shape.count >= 3
}

// signedArea is twice the polygon's signed area. Its sign says which side of a
// directed edge is inside.
func (shape iconClipShape) signedArea() float64 {
	total := 0.0
	for index := 0; index < shape.count; index++ {
		a, b := shape.points[index], shape.points[(index+1)%shape.count]
		total += float64(a.X)*float64(b.Y) - float64(b.X)*float64(a.Y)
	}
	return total
}

// eachTriangle clips one triangle to the shape and emits the pieces. A triangle
// wholly inside comes back unchanged; the pieces tile the part that overlaps.
func (shape iconClipShape) eachTriangle(
	points [3]Point,
	edges uint8,
	emit func([3]Point, uint8, [3][3]float32),
) {
	if !shape.clips() {
		return
	}
	area := shape.signedArea()
	if area == 0 {
		return
	}
	sign := float64(1)
	if area < 0 {
		sign = -1
	}
	current := iconClipPolygon{}
	for index := range points {
		var weights [3]float32
		weights[index] = 1
		current.add(points[index], edges>>index&1 != 0, weights)
	}
	var next iconClipPolygon
	for index := 0; index < shape.count && current.count > 0; index++ {
		a := shape.points[index]
		b := shape.points[(index+1)%shape.count]
		next.count = 0
		for vertex := 0; vertex < current.count; vertex++ {
			from := current.vertices[vertex]
			to := current.vertices[(vertex+1)%current.count]
			fromInside := insideIconClip(a, b, from.point, sign)
			toInside := insideIconClip(a, b, to.point, sign)
			if fromInside {
				next.add(from.point, from.boundary, from.weights)
				if !toInside {
					point, u := iconClipIntersection(a, b, from, to)
					next.add(point, false, iconClipWeights(from.weights, to.weights, u))
				}
			} else if toInside {
				point, u := iconClipIntersection(a, b, from, to)
				next.add(point, from.boundary, iconClipWeights(from.weights, to.weights, u))
			}
		}
		current, next = next, current
	}
	emitIconClipTriangles(current, emit)
}

// iconClipWeights interpolates barycentric weights along from→to.
func iconClipWeights(from, to [3]float32, u float64) (weights [3]float32) {
	for index := range weights {
		weights[index] = float32(float64(from[index]) + (float64(to[index])-float64(from[index]))*u)
	}
	return weights
}

// insideIconClip reports whether point lies on the inside of the directed edge
// a→b. sign makes either winding of the clip polygon work.
func insideIconClip(a, b, point Point, sign float64) bool {
	cross := (float64(b.X)-float64(a.X))*(float64(point.Y)-float64(a.Y)) -
		(float64(b.Y)-float64(a.Y))*(float64(point.X)-float64(a.X))
	return sign*cross >= -1e-5
}

// iconClipIntersection is where the line from→to crosses the clip edge a→b.
func iconClipIntersection(a, b Point, from, to iconClipVertex) (Point, float64) {
	ax, ay := float64(a.X), float64(a.Y)
	bx, by := float64(b.X), float64(b.Y)
	fx, fy := float64(from.point.X), float64(from.point.Y)
	tx, ty := float64(to.point.X), float64(to.point.Y)
	dx, dy := tx-fx, ty-fy
	ex, ey := bx-ax, by-ay
	denominator := dx*ey - dy*ex
	if math.Abs(denominator) < 1e-9 {
		return from.point, 0
	}
	u := ((ax-fx)*ey - (ay-fy)*ex) / denominator
	return Point{X: float32(fx + dx*u), Y: float32(fy + dy*u)}, u
}

// emitIconClipTriangles fans a clipped convex polygon. Slivers and duplicate
// corners a cut leaves on the boundary are skipped.
func emitIconClipTriangles(polygon iconClipPolygon, emit func([3]Point, uint8, [3][3]float32)) {
	count := 0
	for index := 0; index < polygon.count; index++ {
		next := polygon.vertices[(index+1)%polygon.count]
		if samePoint(polygon.vertices[index].point, next.point) {
			continue
		}
		polygon.vertices[count] = polygon.vertices[index]
		count++
	}
	if count < 3 {
		return
	}
	for index := 1; index+1 < count; index++ {
		a, b, c := polygon.vertices[0], polygon.vertices[index], polygon.vertices[index+1]
		cross := (b.point.X-a.point.X)*(c.point.Y-a.point.Y) - (b.point.Y-a.point.Y)*(c.point.X-a.point.X)
		if !clipFiniteArea(a.point, b.point, c.point) || cross > -1e-5 && cross < 1e-5 {
			continue
		}
		var edges uint8
		if index == 1 && a.boundary {
			edges |= 1
		}
		if b.boundary {
			edges |= 2
		}
		if index+2 == count && polygon.vertices[count-1].boundary {
			edges |= 4
		}
		emit([3]Point{a.point, b.point, c.point}, edges, [3][3]float32{a.weights, b.weights, c.weights})
	}
}

func clipFiniteArea(a, b, c Point) bool {
	for _, point := range [...]Point{a, b, c} {
		if math.IsNaN(float64(point.X)) || math.IsNaN(float64(point.Y)) ||
			math.IsInf(float64(point.X), 0) || math.IsInf(float64(point.Y), 0) {
			return false
		}
	}
	return true
}
