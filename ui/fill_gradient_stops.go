package ui

import "math"

// GradientStop is a color at a position along a gradient line: 0 at the start of
// the line and 1 at its end.
type GradientStop struct {
	At    float32
	Color Color
}

// Stop is a GradientStop, written where a gradient is declared:
//
//	canvas.FillGradient(bounds, 12, 135, []ui.GradientStop{ui.Stop(0, from), ui.Stop(1, to)})
func Stop(at float32, color Color) GradientStop { return GradientStop{At: at, Color: color} }

// FillGradient fills the rounded rectangle bounds with a linear gradient through
// stops, the way CSS linear-gradient does. angle is in degrees clockwise from
// pointing up: 0 runs from the bottom edge to the top, 90 from left to right, 180
// from top to bottom. The gradient line passes through the center of bounds and is
// as long as it takes for the two corners the angle points at to take the first
// and the last stop exactly, so a 135 degree gradient on any rectangle starts in
// its top left corner and ends in its bottom right one.
//
// Stops are taken in order; a stop placed before the one ahead of it is moved onto
// it, which makes a hard edge. Before the first stop and after the last the color
// is held. Two stops with the same position give a hard edge, none paint nothing
// and one is a solid fill. Colors between stops are interpolated in straight RGBA,
// as SVG does, except that a stop with no opacity takes the color of the stop it
// meets, so a fade to Color{} is a fade of opacity and not a pass through gray.
//
// The shape is a mesh of triangles whose outer edges are feathered like a path's,
// not the rounded rectangle primitive Rounded draws, so a gradient costs one
// triangle per few degrees of corner and per stop, and no more.
func (canvas *Canvas) FillGradient(bounds Rect, radius, angle float32, stops []GradientStop) {
	if len(stops) == 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	if len(stops) == 1 {
		canvas.Rounded(bounds, radius, stops[0].Color)
		return
	}
	bounds = canvas.snapRect(bounds, radius)
	radius = max(0, min(radius, bounds.Width/2, bounds.Height/2))

	sin, cos := math.Sincos(float64(angle) * math.Pi / 180)
	dx, dy := float32(sin), float32(-cos)
	half := (abs32(bounds.Width*dx) + abs32(bounds.Height*dy)) / 2
	center := Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	axis := newGradientAxis(LinearGradient{
		Start: Point{X: center.X - dx*half, Y: center.Y - dy*half},
		End:   Point{X: center.X + dx*half, Y: center.Y + dy*half},
	}, Rect{}, 1, 1)

	points := canvas.roundedOutline(bounds, radius)
	for index := range points {
		next := points[(index+1)%len(points)]
		canvas.appendGradientTriangle([3]Point{center, points[index], next}, 1<<1, axis, stops)
	}
}

// roundedOutline lists the corners of a rounded rectangle clockwise, flattened so
// an arc is within the tolerance curves are flattened to, in memory the canvas
// keeps. The list is valid until the next call.
func (canvas *Canvas) roundedOutline(bounds Rect, radius float32) []Point {
	points := canvas.fillPoints[:0]
	if radius <= 0 {
		points = append(points,
			Point{X: bounds.X, Y: bounds.Y}, Point{X: bounds.X + bounds.Width, Y: bounds.Y},
			Point{X: bounds.X + bounds.Width, Y: bounds.Y + bounds.Height}, Point{X: bounds.X, Y: bounds.Y + bounds.Height})
		canvas.fillPoints = points
		return points
	}
	segments := arcSegments(float64(radius*max(1, canvas.pixelScale)), math.Pi/2)
	step := math.Pi / 2 / float64(segments)
	centers := [4]Point{
		{X: bounds.X + radius, Y: bounds.Y + radius},
		{X: bounds.X + bounds.Width - radius, Y: bounds.Y + radius},
		{X: bounds.X + bounds.Width - radius, Y: bounds.Y + bounds.Height - radius},
		{X: bounds.X + radius, Y: bounds.Y + bounds.Height - radius},
	}
	for corner, at := range centers {
		start := math.Pi + float64(corner)*math.Pi/2 // left, top, right, bottom of the corner's circle
		for index := 0; index <= segments; index++ {
			sin, cos := math.Sincos(start + float64(index)*step)
			points = append(points, Point{X: at.X + radius*float32(cos), Y: at.Y + radius*float32(sin)})
		}
	}
	canvas.fillPoints = points
	return points
}

// appendGradientTriangle cuts one triangle of the shape where each stop lies, so
// that no piece crosses a stop, and colors each piece from the two stops it lies
// between. Cutting a convex polygon with parallel lines leaves convex pieces, and
// the color along the line is linear within each, so the vertex colors are exact.
func (canvas *Canvas) appendGradientTriangle(points [3]Point, edges uint8, axis gradientAxis, stops []GradientStop) {
	rest := gradientPolygon{count: 3}
	for index := range points {
		rest.corners[index] = gradientVertex{point: points[index], boundary: edges>>index&1 != 0}
	}
	var below gradientPolygon
	low, high := stops[0].At, stops[0].At
	for index := range stops {
		low, high = high, max(high, stops[index].At)
		below, rest = rest.split(axis, high)
		canvas.appendGradientPiece(below, axis, stops, index, low, high)
	}
	canvas.appendGradientPiece(rest, axis, stops, len(stops), high, high)
}

// appendGradientPiece paints the part of the shape between the stops before index
// and the one at index, which lies from low to high along the line: before the
// first stop and after the last it is that stop's color held.
func (canvas *Canvas) appendGradientPiece(piece gradientPolygon, axis gradientAxis, stops []GradientStop, index int, low, high float32) {
	var from, to Color
	held := index == 0 || index == len(stops)
	switch {
	case index == 0:
		from, to = stops[0].Color, stops[0].Color
	case held:
		from, to = stops[index-1].Color, stops[index-1].Color
	default:
		if high <= low {
			return // a hard edge: nothing lies between the two stops
		}
		from, to = stops[index-1].Color, stops[index].Color
	}
	// A stop with no opacity takes the color of its neighbor, so a fade to it
	// changes opacity alone.
	fromRGB, toRGB := from, to
	if from.Alpha <= 0 {
		fromRGB.Red, fromRGB.Green, fromRGB.Blue = to.Red, to.Green, to.Blue
	}
	if to.Alpha <= 0 {
		toRGB.Red, toRGB.Green, toRGB.Blue = from.Red, from.Green, from.Blue
	}
	colorAt := func(point Point) Color {
		if held {
			return from
		}
		u := max(0, min((axis.t(point)-low)/(high-low), 1))
		return Color{
			Red:   Lerp(fromRGB.Red, toRGB.Red, u),
			Green: Lerp(fromRGB.Green, toRGB.Green, u),
			Blue:  Lerp(fromRGB.Blue, toRGB.Blue, u),
			Alpha: Lerp(from.Alpha, to.Alpha, u),
		}
	}
	clip, clipped := canvas.clipSnapshot()
	piece.emitTriangles(func(points [3]Point, edges uint8) {
		triangle := Triangle{
			Points: points, EdgeMask: edges, Layer: canvas.currentLayer(),
			ClipOf: clip, HasClip: clipped, RectsBefore: uint32(len(canvas.rectangles)),
		}
		triangle.Color = colorAt(points[0])
		shade := [2]Color{colorAt(points[1]), colorAt(points[2])}
		if shade[0] != triangle.Color || shade[1] != triangle.Color {
			triangle.Shaded, triangle.Shade = true, shade
		}
		if triangle.visible() {
			canvas.triangles = append(canvas.triangles, triangle)
		}
	})
}
