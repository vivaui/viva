package gfx

import "math"

// PathCommandKind identifies one immutable path command.
type PathCommandKind uint8

const (
	PathCommandMove PathCommandKind = iota + 1
	PathCommandLine
	PathCommandQuad
	PathCommandCubic
	PathCommandClose
)

// PathCommand is one explicit path instruction in view-box coordinates.
// Quad commands use Control1; cubic commands use both control points.
type PathCommand struct {
	Kind     PathCommandKind
	Point    Point
	Control1 Point
	Control2 Point
}

// MoveTo begins a new path contour.
func MoveTo(x, y float32) PathCommand {
	return PathCommand{Kind: PathCommandMove, Point: Point{X: x, Y: y}}
}

// LineTo appends one straight segment to the current contour.
func LineTo(x, y float32) PathCommand {
	return PathCommand{Kind: PathCommandLine, Point: Point{X: x, Y: y}}
}

// QuadTo appends one quadratic curve to the current contour.
func QuadTo(cx, cy, x, y float32) PathCommand {
	return PathCommand{Kind: PathCommandQuad, Point: Point{X: x, Y: y}, Control1: Point{X: cx, Y: cy}}
}

// CubicTo appends one cubic curve to the current contour.
func CubicTo(c1x, c1y, c2x, c2y, x, y float32) PathCommand {
	return PathCommand{
		Kind:     PathCommandCubic,
		Point:    Point{X: x, Y: y},
		Control1: Point{X: c1x, Y: c1y},
		Control2: Point{X: c2x, Y: c2y},
	}
}

// ClosePath closes the current contour.
func ClosePath() PathCommand {
	return PathCommand{Kind: PathCommandClose}
}

// PathFillRule selects how overlapping contour coverage combines.
type PathFillRule uint8

const (
	// PathFillNonZero fills where the winding number is nonzero.
	PathFillNonZero PathFillRule = iota
	// PathFillEvenOdd fills where the contour crossing count is odd.
	PathFillEvenOdd
)

// StrokeCap selects the shape at open contour endpoints.
type StrokeCap uint8

const (
	// CapButt ends the stroke exactly at the endpoint.
	CapButt StrokeCap = iota
	// CapRound ends the stroke with a half disc.
	CapRound
	// CapSquare extends the stroke by half the width.
	CapSquare
)

// StrokeJoin selects the corner shape between stroke segments.
type StrokeJoin uint8

const (
	// JoinMiter extends the outer corner to the offset line intersection,
	// falling back to a bevel beyond the miter limit of 4.
	JoinMiter StrokeJoin = iota
	// JoinRound fills the outer corner with an arc.
	JoinRound
	// JoinBevel cuts the outer corner with a straight edge.
	JoinBevel
)

// PathStyle describes a path fill and/or stroke. StrokeWidth is measured in
// logical pixels after the view-box transform. Zero-alpha colors are skipped.
//
// FillGradient, when set, replaces Fill: it is a linear gradient in the path's
// view-box units, held flat beyond both ends. It only colors the fill; the stroke
// stays the solid Stroke color.
type PathStyle struct {
	Fill         Color
	FillGradient *LinearGradient
	Stroke       Color
	StrokeWidth  float32
	// Dashed breaks the stroke into equal dashes and gaps, continuing around
	// corners and closed contours without closing a short gap.
	Dashed   bool
	FillRule PathFillRule
	Cap      StrokeCap
	Join     StrokeJoin
}

// Path is immutable vector geometry in a fixed view box. NewPath copies the
// commands once; painting the path does not mutate or allocate it.
type Path struct {
	width    float32
	height   float32
	commands []PathCommand
}

// NewPath creates immutable geometry. Width and height define its view box.
func NewPath(width, height float32, commands ...PathCommand) Path {
	if width <= 0 || height <= 0 || len(commands) == 0 {
		return Path{}
	}
	return Path{width: width, height: height, commands: append([]PathCommand(nil), commands...)}
}

// Size returns the path view box.
func (path Path) Size() Size {
	return Size{Width: path.width, Height: path.height}
}

// flattenTolerance is the maximum curve deviation in destination pixels.
const flattenTolerance = 0.1

// minFlattenTolerance bounds the tolerance from below for small paths: a
// fraction of a device pixel at 2x.
const minFlattenTolerance = 0.06

// miterLimit follows the SVG default: the miter length may reach four times
// the half width before falling back to a bevel.
const miterLimit = 4.0

// DrawPath tessellates immutable path geometry into retained triangles.
// Fills support concave contours and holes under both fill rules; strokes
// tessellate into non-overlapping outlines so translucent stroke colors
// composite without seams.
func (canvas *Canvas) DrawPath(path Path, bounds Rect, style PathStyle) {
	canvas.triangles = canvas.PathTriangles(canvas.triangles, path, bounds, style)
}

// PathTriangles appends to dst the triangles DrawPath would add for the same path, bounds
// and style, without painting them. A line that has to draw its geometry in its own place in
// the paint order — a fixed-pitch line's cell glyph strokes, which go before its text — asks
// for the triangles and places them itself.
func (canvas *Canvas) PathTriangles(dst []Triangle, path Path, bounds Rect, style PathStyle) []Triangle {
	// The tessellator appends to the canvas's own list; lend it dst for the call.
	saved := canvas.triangles
	canvas.triangles = dst
	canvas.drawPath(path, bounds, style)
	dst = canvas.triangles
	canvas.triangles = saved
	return dst
}

// drawPath tessellates path into canvas.triangles; it is DrawPath's body.
func (canvas *Canvas) drawPath(path Path, bounds Rect, style PathStyle) {
	if path.width <= 0 || path.height <= 0 || bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	fill := style.Fill
	if style.FillGradient != nil {
		// The gradient recolors the flat fill below; the flat color only needs
		// to be visible so the fill is tessellated at all.
		fill = style.FillGradient.From
		fill.Alpha = max(style.FillGradient.From.Alpha, style.FillGradient.To.Alpha)
	}
	if fill.Alpha <= 0 && (style.Stroke.Alpha <= 0 || style.StrokeWidth <= 0) {
		return
	}
	// The flatten tolerance scales with the display size but never drops
	// below minFlattenTolerance: small curves such as an icon's circles
	// flatten to visually identical polygons with far fewer segments, yet a
	// coarser floor shows as facets once the stroke is only a pixel or two.
	tolerance := flattenTolerance * max32(scaleFactor(bounds.Width, path.width), scaleFactor(bounds.Height, path.height))
	if tolerance < minFlattenTolerance {
		tolerance = minFlattenTolerance
	}
	sx := bounds.Width / path.width
	sy := bounds.Height / path.height
	transform := func(point Point) Point {
		return Point{X: bounds.X + point.X*sx, Y: bounds.Y + point.Y*sy}
	}

	canvas.pathPoints = canvas.pathPoints[:0]
	canvas.contourEnds = canvas.contourEnds[:0]
	canvas.contourClosed = canvas.contourClosed[:0]
	inContour := false
	for _, command := range path.commands {
		switch command.Kind {
		case PathCommandMove:
			if inContour {
				canvas.finishContour()
			}
			canvas.pathPoints = append(canvas.pathPoints, transform(command.Point))
			inContour = true
		case PathCommandLine:
			if inContour {
				canvas.pathPoints = append(canvas.pathPoints, transform(command.Point))
			}
		case PathCommandQuad:
			if inContour {
				control := transform(command.Control1)
				end := transform(command.Point)
				start := canvas.pathPoints[len(canvas.pathPoints)-1]
				canvas.flattenQuad(start, control, end, tolerance, 0)
			}
		case PathCommandCubic:
			if inContour {
				control1 := transform(command.Control1)
				control2 := transform(command.Control2)
				end := transform(command.Point)
				start := canvas.pathPoints[len(canvas.pathPoints)-1]
				canvas.flattenCubic(start, control1, control2, end, tolerance, 0)
			}
		case PathCommandClose:
			if inContour {
				canvas.finishContour()
				canvas.contourClosed[len(canvas.contourClosed)-1] = true
				inContour = false
			}
		}
	}
	if inContour {
		canvas.finishContour()
	}
	if len(canvas.contourEnds) == 0 {
		return
	}

	if fill.Alpha > 0 {
		start := len(canvas.triangles)
		canvas.fillContours(style.FillRule, fill)
		if style.FillGradient != nil {
			canvas.shadeGradient(start, newGradientAxis(*style.FillGradient, bounds, sx, sy), style.FillGradient, 1)
		}
	}
	if style.Stroke.Alpha > 0 && style.StrokeWidth > 0 {
		half := style.StrokeWidth * 0.5
		opaque := style.Stroke.Alpha >= 0.999
		if style.Dashed {
			canvas.dashStrokeContours(opaque, half, style.Cap, style.Join, style.Stroke)
		} else {
			start := int32(0)
			for index, end := range canvas.contourEnds {
				points := canvas.pathPoints[start:end]
				canvas.strokeFlattenedContour(points, canvas.contourClosed[index], opaque, half, style.Cap, style.Join, style.Stroke)
				start = end
			}
		}
	}
}

// strokeFlattenedContour gives one flattened contour the cap and join it was asked for.
func (canvas *Canvas) strokeFlattenedContour(points []Point, closed, opaque bool, half float32, cap StrokeCap, join StrokeJoin, color Color) {
	if len(points) >= 2 {
		if opaque {
			// Opaque strokes take the overlapping decomposition:
			// segment quads plus round join discs. Overlap is
			// invisible at full alpha and the geometry stays
			// robust when contour features shrink below the
			// stroke width, where offset outlines self-intersect.
			canvas.strokeContourOverlapping(points, closed, half, cap, join, color)
		} else {
			canvas.strokeContour(points, closed, half, cap, join, color)
		}
	} else if len(points) == 1 && cap == CapRound {
		canvas.appendDisc(points[0], half, color)
	}
}

// dashStrokeContours splits flattened contours into equal dashes and gaps,
// keeping the 6:4 dash-to-gap ratio of the dashed box-drawing glyphs. On a
// closed contour the repetition is fitted to the perimeter — the same number of
// periods however long the outline is, each a whole pattern cycle — so the
// pattern closes exactly at its starting corner instead of leaving a partial
// dash. Arc-length phase continues across corners and curves; the outline of
// every dash is cut at the phase boundary itself, so a translucent stroke
// stays seam-free.
func (canvas *Canvas) dashStrokeContours(opaque bool, half float32, cap StrokeCap, join StrokeJoin, color Color) {
	const (
		dashShare = 6
		gapShare  = 4
	)
	// dashUnitCut is the arc length one full period is aimed at, in stroke
	// widths: ten times the width, as a dashed UI border reads at any size.
	aimedPeriod := half * 2 * 10
	sourceStart := int32(0)
	for contour, end := range canvas.contourEnds {
		source := canvas.pathPoints[sourceStart:end]
		sourceStart = end
		if len(source) < 2 {
			continue
		}
		closed := canvas.contourClosed[contour]
		last := len(source)
		if !closed {
			last--
		}
		var length float32
		for index := 0; index < last; index++ {
			length += distance(source[index], source[(index+1)%len(source)])
		}
		if length <= 0 {
			continue
		}
		dash, gap := aimedPeriod*dashShare/10, aimedPeriod*gapShare/10
		if closed {
			// A whole number of periods closes the contour: the pattern's phase
			// returns to its start exactly as the point does.
			periods := max(int(length/aimedPeriod+0.5), 1)
			dash, gap = length/float32(periods)*dashShare/10, length/float32(periods)*gapShare/10
		}
		canvas.dashPoints = canvas.dashPoints[:0]
		canvas.dashPoints = append(canvas.dashPoints, source[0])
		on, remaining := true, dash
		index, at := 0, float32(0)
		for index < last {
			from, to := source[index], source[(index+1)%len(source)]
			segmentLength := distance(from, to)
			if segmentLength <= 0 {
				index++
				continue
			}
			for remaining < segmentLength-at {
				cut := from
				along := at + remaining
				if along > 0 {
					fraction := min(along/segmentLength, 1)
					cut = Point{X: from.X + (to.X-from.X)*fraction, Y: from.Y + (to.Y-from.Y)*fraction}
				}
				if on {
					canvas.dashPoints = append(canvas.dashPoints, cut)
					canvas.strokeFlattenedContour(canvas.dashPoints, false, opaque, half, cap, join, color)
					canvas.dashPoints = canvas.dashPoints[:0]
				} else {
					canvas.dashPoints = append(canvas.dashPoints, cut)
				}
				on, remaining = !on, gap
				if on {
					remaining = dash
				}
				at = along
			}
			remaining -= segmentLength - at
			at = 0
			if on {
				canvas.dashPoints = append(canvas.dashPoints, to)
			}
			index++
		}
		if on && len(canvas.dashPoints) >= 2 {
			// The contour ends inside a dash. A closed one whose whole periods
			// land on source[0] never reaches here; an open one, or a rounding
			// remainder, keeps the dash open rather than bridging it.
			canvas.strokeFlattenedContour(canvas.dashPoints, false, opaque, half, cap, join, color)
		}
		canvas.dashPoints = canvas.dashPoints[:0]
	}
}

const triangleAllEdges = uint8(1<<3 - 1)

func canonicalEdge(a, b Point) [4]uint32 {
	ax, ay := pointBits(a)
	bx, by := pointBits(b)
	if bx < ax || (bx == ax && by < ay) {
		ax, bx = bx, ax
		ay, by = by, ay
	}
	return [4]uint32{ax, ay, bx, by}
}

func pointBits(point Point) (uint32, uint32) {
	// Treat signed zero as one coordinate so otherwise identical edges match.
	if point.X == 0 {
		point.X = 0
	}
	if point.Y == 0 {
		point.Y = 0
	}
	return math.Float32bits(point.X), math.Float32bits(point.Y)
}

// strokeContourOverlapping tessellates one contour's stroke as per-segment
// quads plus join wedges. Triangles overlap at joins and caps, which is
// only correct for opaque colors; translucent strokes must use
// strokeContour. Points that lie within strokeMergeEpsilon of the line
// between their neighbors are merged first: that only removes vertices that
// change nothing visible, so the flattened curve keeps its accuracy.
func (canvas *Canvas) strokeContourOverlapping(points []Point, closed bool, half float32, cap StrokeCap, join StrokeJoin, color Color) {
	canvas.strokeSideA = mergeCollinear(canvas.strokeSideA[:0], points, closed, strokeMergeEpsilon)
	points = canvas.strokeSideA
	count := len(points)
	if count < 2 {
		if count == 1 && cap == CapRound {
			canvas.appendDisc(points[0], half, color)
		}
		return
	}
	edges := count - 1
	if closed {
		edges = count
	}
	for index := 0; index < edges; index++ {
		from := points[index]
		to := points[(index+1)%count]
		canvas.appendStrokeQuad(
			from, to, half, color,
			!closed && index == 0 && cap == CapButt,
			!closed && index == edges-1 && cap == CapButt,
		)
	}
	for index := 0; index < count; index++ {
		if !closed && (index == 0 || index == count-1) {
			continue
		}
		previous := points[(index-1+count)%count]
		next := points[(index+1)%count]
		if distance(previous, points[index]) < 1e-4 || distance(points[index], next) < 1e-4 {
			continue
		}
		canvas.appendStrokeJoin(previous, points[index], next, half, join, color)
	}
	if !closed {
		first, last := points[0], points[count-1]
		intoFirst := direction(points[1], first)
		outLast := direction(points[count-2], last)
		canvas.appendCapGeometry(first, intoFirst, half, cap, color)
		canvas.appendCapGeometry(last, negate(outLast), half, cap, color)
	}
}

// strokeMergeEpsilon is how far, in logical pixels, a vertex may sit from the
// line through its neighbors and still be dropped from an opaque stroke.
// It must stay far below the stroke width: a width-relative threshold (the
// stroke used half the half-width) simplified smooth curves into visible
// polygons.
const strokeMergeEpsilon = 0.02

// roundJoinFlat is the largest gap, in logical pixels, between a round join's
// arc and its chord that is drawn as the chord alone: smaller than that no
// pixel can tell them apart, and the join costs one triangle, not a fan.
const roundJoinFlat = 0.02

// appendStrokeJoin fills the wedge that two stroke quads leave open on the
// outer side of the corner at vertex. Inner sides overlap and need nothing.
// Every wedge feathers only its outer boundary, so it joins the quads
// seamlessly whatever the corner's angle.
func (canvas *Canvas) appendStrokeJoin(previous, vertex, next Point, half float32, join StrokeJoin, color Color) {
	in := direction(previous, vertex)
	out := direction(vertex, next)
	cross := in.X*out.Y - in.Y*out.X
	dot := in.X*out.X + in.Y*out.Y
	if abs32(cross) < 1e-6 && dot > 0 {
		return // straight on: the quads already meet
	}
	// The outer side is the one the path turns away from.
	side := float32(1)
	if cross > 0 {
		side = -1
	}
	endA := Point{X: vertex.X - in.Y*half*side, Y: vertex.Y + in.X*half*side}
	endB := Point{X: vertex.X - out.Y*half*side, Y: vertex.Y + out.X*half*side}

	switch join {
	case JoinRound:
		turn := math.Atan2(float64(abs32(cross)), float64(dot))
		if dot < -0.999 {
			canvas.appendDisc(vertex, half, color) // a reversal: a round cap
			return
		}
		if float64(half)*(1-math.Cos(turn/2)) > roundJoinFlat {
			arc := appendJoinArc(canvas.strokeSideB[:0], vertex, endA, endB)
			canvas.strokeSideB = arc
			previousPoint := endA
			for _, point := range arc {
				canvas.appendTriangleEdges(vertex, previousPoint, point, color, 1<<1)
				previousPoint = point
			}
			canvas.appendTriangleEdges(vertex, previousPoint, endB, color, 1<<1)
			return
		}
	case JoinMiter:
		if tip, ok := intersectLines(endA, in, endB, out); ok && distance(tip, vertex) <= miterLimit*half {
			canvas.appendTriangleEdges(vertex, endA, tip, color, 1<<1)
			canvas.appendTriangleEdges(vertex, tip, endB, color, 1<<1)
			return
		}
	}
	canvas.appendTriangleEdges(vertex, endA, endB, color, 1<<1)
}

// mergeCollinear drops points whose neighbors see through them: a point
// closer than epsilon to the line between its neighbors adds no shape.
func mergeCollinear(destination []Point, points []Point, closed bool, epsilon float32) []Point {
	for index, point := range points {
		if len(destination) == 0 {
			destination = append(destination, point)
			continue
		}
		previous := destination[len(destination)-1]
		var next Point
		if index+1 < len(points) {
			next = points[index+1]
		} else if closed && len(points) > 1 {
			next = points[0]
		} else {
			destination = append(destination, point)
			continue
		}
		if distancePointLine(point, previous, next) > epsilon || distance(previous, next) < 1e-4 {
			destination = append(destination, point)
		}
	}
	if closed && len(destination) > 1 && distance(destination[0], destination[len(destination)-1]) < 1e-4 {
		destination = destination[:len(destination)-1]
	}
	return destination
}

// appendStrokeQuad adds one offset rectangle for the segment from→to. The long
// sides are always visible union boundaries. A cross edge is a boundary only
// at an open butt cap; joins and explicit cap geometry cover every other one.
func (canvas *Canvas) appendStrokeQuad(from, to Point, half float32, color Color, startCap, endCap bool) {
	dir := direction(from, to)
	normal := Point{X: -dir.Y * half, Y: dir.X * half}
	a := Point{X: from.X + normal.X, Y: from.Y + normal.Y}
	b := Point{X: to.X + normal.X, Y: to.Y + normal.Y}
	c := Point{X: to.X - normal.X, Y: to.Y - normal.Y}
	d := Point{X: from.X - normal.X, Y: from.Y - normal.Y}
	firstMask := uint8(1 << 0)
	secondMask := uint8(1 << 2)
	if startCap {
		firstMask |= 1 << 2
	}
	if endCap {
		secondMask |= 1 << 1
	}
	canvas.appendTriangleEdges(a, b, d, color, firstMask)
	canvas.appendTriangleEdges(d, b, c, color, secondMask)
}

// appendCapGeometry adds the cap at endpoint p, with `outward` pointing
// away from the contour.
func (canvas *Canvas) appendCapGeometry(p, outward Point, half float32, cap StrokeCap, color Color) {
	switch cap {
	case CapRound:
		canvas.appendDisc(p, half, color)
	case CapSquare:
		center := Point{X: p.X + outward.X*half, Y: p.Y + outward.Y*half}
		side := Point{X: -outward.Y * half, Y: outward.X * half}
		a := Point{X: p.X + side.X, Y: p.Y + side.Y}
		b := Point{X: center.X + side.X, Y: center.Y + side.Y}
		c := Point{X: center.X - side.X, Y: center.Y - side.Y}
		d := Point{X: p.X - side.X, Y: p.Y - side.Y}
		canvas.appendTriangleEdges(a, b, d, color, 1<<0)
		canvas.appendTriangleEdges(d, b, c, color, 1<<1|1<<2)
	}
}

func scaleFactor(destination, source float32) float32 {
	if source <= 0 {
		return 1
	}
	return destination / source
}

// finishContour records the end of the current open contour, dropping
// contours that never grew past their move point.
func (canvas *Canvas) finishContour() {
	start := int32(0)
	if len(canvas.contourEnds) > 0 {
		start = canvas.contourEnds[len(canvas.contourEnds)-1]
	}
	if int32(len(canvas.pathPoints))-start < 2 {
		canvas.pathPoints = canvas.pathPoints[:start]
		return
	}
	canvas.contourEnds = append(canvas.contourEnds, int32(len(canvas.pathPoints)))
	canvas.contourClosed = append(canvas.contourClosed, false)
}

// flattenQuad appends the flattened interior and endpoint of one quadratic
// curve using adaptive subdivision.
func (canvas *Canvas) flattenQuad(start, control, end Point, tolerance float32, depth int) {
	deviation := 0.5 * distancePointLine(control, start, end)
	if deviation <= tolerance || depth >= 14 {
		canvas.pathPoints = append(canvas.pathPoints, end)
		return
	}
	depth++
	left := Point{X: (start.X + control.X) * 0.5, Y: (start.Y + control.Y) * 0.5}
	right := Point{X: (control.X + end.X) * 0.5, Y: (control.Y + end.Y) * 0.5}
	middle := Point{X: (left.X + right.X) * 0.5, Y: (left.Y + right.Y) * 0.5}
	canvas.flattenQuad(start, left, middle, tolerance, depth)
	canvas.flattenQuad(middle, right, end, tolerance, depth)
}

// flattenCubic appends the flattened interior and endpoint of one cubic
// curve using adaptive subdivision.
func (canvas *Canvas) flattenCubic(start, control1, control2, end Point, tolerance float32, depth int) {
	deviation := 0.75 * max32(distancePointLine(control1, start, end), distancePointLine(control2, start, end))
	if deviation <= tolerance || depth >= 14 {
		canvas.pathPoints = append(canvas.pathPoints, end)
		return
	}
	depth++
	ab := Point{X: (start.X + control1.X) * 0.5, Y: (start.Y + control1.Y) * 0.5}
	bc := Point{X: (control1.X + control2.X) * 0.5, Y: (control1.Y + control2.Y) * 0.5}
	cd := Point{X: (control2.X + end.X) * 0.5, Y: (control2.Y + end.Y) * 0.5}
	abc := Point{X: (ab.X + bc.X) * 0.5, Y: (ab.Y + bc.Y) * 0.5}
	bcd := Point{X: (bc.X + cd.X) * 0.5, Y: (bc.Y + cd.Y) * 0.5}
	middle := Point{X: (abc.X + bcd.X) * 0.5, Y: (abc.Y + bcd.Y) * 0.5}
	canvas.flattenCubic(start, ab, abc, middle, tolerance, depth)
	canvas.flattenCubic(middle, bcd, cd, end, tolerance, depth)
}

func distancePointLine(point, a, b Point) float32 {
	dx, dy := b.X-a.X, b.Y-a.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return float32(math.Hypot(float64(point.X-a.X), float64(point.Y-a.Y)))
	}
	return abs32(dx*(point.Y-a.Y)-dy*(point.X-a.X)) / length
}

func abs32(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// strokeContour tessellates one contour's stroke. Open contours build a
// single outline ring from the two offset sides plus caps; closed contours
// fill the region between the two offset loops, reusing the hole-aware fill
// triangulation.
func (canvas *Canvas) strokeContour(points []Point, closed bool, half float32, cap StrokeCap, join StrokeJoin, color Color) {
	if half <= 0 {
		return
	}
	canvas.strokeSideA = canvas.offsetSide(canvas.strokeSideA[:0], points, closed, half, 1, join)
	canvas.strokeSideB = canvas.offsetSide(canvas.strokeSideB[:0], points, closed, half, -1, join)
	if closed {
		// The stroke band is one ring with a hole: whichever offset side
		// encloses more area is the outer boundary; the other, reversed,
		// becomes the hole.
		outer, inner := canvas.strokeSideA, canvas.strokeSideB
		if abs32(float32(polygonArea(canvas.strokeSideA))) < abs32(float32(polygonArea(canvas.strokeSideB))) {
			outer, inner = canvas.strokeSideB, canvas.strokeSideA
		}
		canvas.ringPoints = canvas.ringPoints[:0]
		canvas.ringEnds = canvas.ringEnds[:0]
		canvas.ringPoints = appendRing(canvas.ringPoints, outer)
		canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
		for index := len(inner) - 1; index >= 0; index-- {
			canvas.ringPoints = append(canvas.ringPoints, inner[index])
		}
		canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
		canvas.earClipRings(color)
		return
	}

	last := len(points) - 1
	outwardEnd := direction(points[last-1], points[last])
	outwardStart := direction(points[1], points[0])
	canvas.strokeRing = canvas.strokeRing[:0]
	canvas.strokeRing = appendRing(canvas.strokeRing, canvas.strokeSideA)
	canvas.strokeRing = canvas.appendCap(canvas.strokeRing, points[last], outwardEnd, canvas.strokeSideA[len(canvas.strokeSideA)-1], canvas.strokeSideB[len(canvas.strokeSideB)-1], half, cap)
	for index := len(canvas.strokeSideB) - 1; index >= 0; index-- {
		canvas.strokeRing = append(canvas.strokeRing, canvas.strokeSideB[index])
	}
	canvas.strokeRing = canvas.appendCap(canvas.strokeRing, points[0], outwardStart, canvas.strokeSideB[0], canvas.strokeSideA[0], half, cap)
	canvas.ringPoints = canvas.ringPoints[:0]
	canvas.ringEnds = canvas.ringEnds[:0]
	canvas.ringPoints = appendRing(canvas.ringPoints, canvas.strokeRing)
	canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
	canvas.earClipRings(color)
}

func negate(point Point) Point {
	return Point{X: -point.X, Y: -point.Y}
}

func direction(from, to Point) Point {
	dx, dy := to.X-from.X, to.Y-from.Y
	length := float32(math.Hypot(float64(dx), float64(dy)))
	if length == 0 {
		return Point{}
	}
	return Point{X: dx / length, Y: dy / length}
}

// appendRing appends source points to dst, skipping a leading point that
// duplicates the dst tail so joined rings stay free of coincident pairs.
func appendRing(destination []Point, source []Point) []Point {
	for index, point := range source {
		if index == 0 && len(destination) > 0 {
			previous := destination[len(destination)-1]
			if abs32(point.X-previous.X) < 1e-4 && abs32(point.Y-previous.Y) < 1e-4 {
				continue
			}
		}
		destination = append(destination, point)
	}
	return destination
}

// offsetSide appends one offset polyline for `side` (+1 or -1 relative to
// the travel direction) to dst. Inner corners clip at offset line
// intersections; outer corners add the selected join geometry.
func (canvas *Canvas) offsetSide(dst []Point, points []Point, closed bool, half float32, side int, join StrokeJoin) []Point {
	count := len(points)
	edges := count - 1
	if closed {
		edges = count
	}
	if edges < 1 {
		return dst
	}
	if cap(canvas.edgeDir) < edges {
		canvas.edgeDir = make([]Point, edges)
		canvas.edgeNormal = make([]Point, edges)
	}
	dir := canvas.edgeDir[:edges]
	normal := canvas.edgeNormal[:edges]
	for index := 0; index < edges; index++ {
		dir[index] = direction(points[index], points[(index+1)%count])
		normal[index] = Point{
			X: -dir[index].Y * half * float32(side),
			Y: dir[index].X * half * float32(side),
		}
	}
	offset := func(vertex, edge int) Point {
		return Point{X: points[vertex].X + normal[edge].X, Y: points[vertex].Y + normal[edge].Y}
	}
	dst = append(dst, offset(0, 0))
	for index := 0; index < edges; index++ {
		vertex := (index + 1) % count
		endA := offset(vertex, index)
		if index == edges-1 && !closed {
			dst = append(dst, endA)
			break
		}
		next := (index + 1) % edges
		endB := offset(vertex, next)
		cross := dir[index].X*dir[next].Y - dir[index].Y*dir[next].X
		dot := dir[index].X*dir[next].X + dir[index].Y*dir[next].Y
		if abs32(cross) < 1e-6 && dot > 0 {
			// Straight continuation: endA equals endB; drop the duplicate.
			if distance(dst[len(dst)-1], endB) > 1e-4 {
				dst = append(dst, endB)
			}
			continue
		}
		if float32(side)*cross < 0 {
			// Outer corner: endA, join geometry, then endB. The offset
			// side bends away from the turn when side opposes the turn.
			dst = append(dst, endA)
			switch join {
			case JoinRound:
				dst = appendJoinArc(dst, points[vertex], endA, endB)
			case JoinMiter:
				if tip, ok := intersectLines(endA, dir[index], endB, dir[next]); ok &&
					distance(tip, points[vertex]) <= miterLimit*half {
					dst = append(dst, tip)
				}
			default:
				// Bevel: the direct edge from endA to endB closes the corner.
			}
			dst = append(dst, endB)
		} else if tip, ok := intersectLines(endA, dir[index], endB, dir[next]); ok &&
			distance(tip, points[vertex]) <= miterLimit*half {
			// Inner corner: clip to the offset line intersection.
			if distance(dst[len(dst)-1], tip) > 1e-4 {
				dst = append(dst, tip)
			}
		} else {
			dst = append(dst, endB)
		}
	}
	return dst
}

// appendJoinArc appends the intermediate arc points of one round join,
// sweeping the short way from `from` to `to` around center.
func appendJoinArc(dst []Point, center, from, to Point) []Point {
	radius := math.Hypot(float64(from.X-center.X), float64(from.Y-center.Y))
	start := math.Atan2(float64(from.Y-center.Y), float64(from.X-center.X))
	end := math.Atan2(float64(to.Y-center.Y), float64(to.X-center.X))
	return appendArcSweep(dst, center, radius, start, end, math.NaN())
}

// appendCap appends the cap geometry between the two offset side endpoints
// to dst, traveling around the endpoint in ring order.
func (canvas *Canvas) appendCap(dst []Point, center, outward, from, to Point, half float32, cap StrokeCap) []Point {
	switch cap {
	case CapRound:
		radius := math.Hypot(float64(from.X-center.X), float64(from.Y-center.Y))
		start := math.Atan2(float64(from.Y-center.Y), float64(from.X-center.X))
		end := math.Atan2(float64(to.Y-center.Y), float64(to.X-center.X))
		through := math.Atan2(float64(outward.Y), float64(outward.X))
		return appendArcSweep(dst, center, radius, start, end, through)
	case CapSquare:
		dst = append(dst, Point{X: from.X + outward.X*half, Y: from.Y + outward.Y*half})
		dst = append(dst, Point{X: to.X + outward.X*half, Y: to.Y + outward.Y*half})
		return dst
	default:
		return dst
	}
}

// appendArcSweep appends interior arc points from the start to the end
// angle. A non-NaN through angle selects the half that passes it (round
// caps); otherwise the sweep takes the short way (round joins).
func appendArcSweep(dst []Point, center Point, radius, start, end, through float64) []Point {
	if radius <= 0 {
		return dst
	}
	sweep := end - start
	for sweep > math.Pi {
		sweep -= 2 * math.Pi
	}
	for sweep < -math.Pi {
		sweep += 2 * math.Pi
	}
	if !math.IsNaN(through) {
		mid := start + sweep/2
		if math.Cos(mid-through) < 0 {
			if sweep > 0 {
				sweep -= 2 * math.Pi
			} else {
				sweep += 2 * math.Pi
			}
		}
	}
	segments := arcSegments(radius, sweep)
	for index := 1; index < segments; index++ {
		angle := start + sweep*float64(index)/float64(segments)
		dst = append(dst, Point{
			X: center.X + float32(radius*math.Cos(angle)),
			Y: center.Y + float32(radius*math.Sin(angle)),
		})
	}
	return dst
}

// arcSegments bounds the arc chord error by minFlattenTolerance, the same
// accuracy curves are flattened to at icon sizes.
func arcSegments(radius, sweep float64) int {
	if radius <= minFlattenTolerance {
		return 1
	}
	step := 2 * math.Acos(math.Max(-1, math.Min(1, 1-minFlattenTolerance/radius)))
	if step <= 0 {
		return 1
	}
	segments := int(math.Ceil(math.Abs(sweep) / step))
	if segments > 64 {
		return 64
	}
	if segments < 1 {
		return 1
	}
	return segments
}

// appendDisc tessellates one full disc, used for zero-length round strokes.
func (canvas *Canvas) appendDisc(center Point, half float32, color Color) {
	segments := max(arcSegments(float64(half), 2*math.Pi), 12)
	previous := Point{X: center.X + half, Y: center.Y}
	for index := 1; index <= segments; index++ {
		angle := float64(index) * 2 * math.Pi / float64(segments)
		next := Point{
			X: center.X + half*float32(math.Cos(angle)),
			Y: center.Y + half*float32(math.Sin(angle)),
		}
		canvas.appendTriangleEdges(center, previous, next, color, 1<<1)
		previous = next
	}
}

func distance(a, b Point) float32 {
	return float32(math.Hypot(float64(a.X-b.X), float64(a.Y-b.Y)))
}

// intersectLines solves a + t*d0 == b + s*d1.
func intersectLines(a Point, d0 Point, b Point, d1 Point) (Point, bool) {
	denominator := d0.X*d1.Y - d0.Y*d1.X
	if abs32(denominator) < 1e-9 {
		return Point{}, false
	}
	dx, dy := b.X-a.X, b.Y-a.Y
	t := (dx*d1.Y - dy*d1.X) / denominator
	return Point{X: a.X + t*d0.X, Y: a.Y + t*d0.Y}, true
}

func (canvas *Canvas) appendTriangle(a, b, c Point, color Color) {
	canvas.appendTriangleEdges(a, b, c, color, 0)
}

func (canvas *Canvas) appendTriangleEdges(a, b, c Point, color Color, edgeMask uint8) {
	triangle := Triangle{
		Points:   [3]Point{a, b, c},
		Color:    color,
		Layer:    canvas.currentLayer(),
		EdgeMask: edgeMask,
		Order:    canvas.nextOrder(),
	}
	triangle.ClipOf, triangle.HasClip = canvas.clipSnapshot()
	canvas.triangles = append(canvas.triangles, triangle)
}
