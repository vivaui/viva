package ui

import "math"

// iconTrimSteps is how finely a curve is sampled when a trim cuts through it.
// The samples only matter for a cut contour: an untrimmed one keeps its curves.
const iconTrimSteps = 64

// trimIconContour returns the part of a one-contour icon between two fractions
// of its length. The full range returns the source unchanged. A partial range is
// flattened to a polyline in the icon's view box, so both cut ends get the
// source's caps and a closing segment is part of the measured length. The
// canvas owns the scratch slices, so a trim allocates nothing once they have
// grown to their working size.
func (canvas *Canvas) trimIconContour(icon Icon, from, to float32) Icon {
	if from <= 0 && to >= 1 {
		return icon
	}
	points, closed := canvas.trimContourPoints(icon.Commands)
	total := polylineLength(points, closed)
	trimmed := icon
	trimmed.Commands = nil
	if total <= 0 || len(points) < 2 {
		return trimmed
	}
	trimmed.Commands = canvas.trimPolyline(points, closed, from*total, to*total)
	return trimmed
}

// trimContourPoints flattens one contour to the points its length is measured
// on. A closed contour repeats its start point at the end of the trace.
func (canvas *Canvas) trimContourPoints(commands []PathCommand) (points []Point, closed bool) {
	var current, start Point
	started := false
	points = canvas.trimPoints[:0]
	appendPoint := func(point Point) {
		if len(points) == 0 || points[len(points)-1] != point {
			points = append(points, point)
		}
	}
	for _, command := range commands {
		switch command.Kind {
		case PathCommandMove:
			current, start = command.Point, command.Point
			appendPoint(current)
			started = true
		case PathCommandLine:
			if started {
				appendPoint(command.Point)
				current = command.Point
			}
		case PathCommandQuad:
			if started {
				points = appendTrimQuad(points, current, command)
				current = command.Point
			}
		case PathCommandCubic:
			if started {
				points = appendTrimCubic(points, current, command)
				current = command.Point
			}
		case PathCommandClose:
			if started {
				closed = true
				appendPoint(start)
				current = start
			}
		}
	}
	canvas.trimPoints = points
	return points, closed
}

// trimPolyline keeps the part of a closed contour between two arc lengths. The
// contour always includes its closing segment in the length and in the trace.
func (canvas *Canvas) trimPolyline(points []Point, closed bool, start, end float32) []PathCommand {
	if len(points) < 2 || start >= end {
		return nil
	}
	commands := canvas.trimCommands[:0]
	var walked float32
	segments := len(points) - 1
	if closed && points[len(points)-1] != points[0] {
		segments = len(points)
	}
	for index := 0; index < segments; index++ {
		next := (index + 1) % len(points)
		from, to := points[index], points[next]
		length := iconTrimDistance(from, to)
		segmentFrom, segmentTo := walked, walked+length
		walked = segmentTo
		if length <= 0 || segmentTo <= start || segmentFrom >= end {
			continue
		}
		fromT, toT := float32(0), float32(1)
		if start > segmentFrom {
			fromT = (start - segmentFrom) / length
		}
		if end < segmentTo {
			toT = (end - segmentFrom) / length
		}
		a, b := lerpPoint(from, to, fromT), lerpPoint(from, to, toT)
		if len(commands) == 0 {
			commands = append(commands, MoveTo(a.X, a.Y))
		}
		commands = append(commands, LineTo(b.X, b.Y))
	}
	canvas.trimCommands = commands
	return commands
}

// appendTrimQuad samples a quadratic curve, from excluded and its end included.
func appendTrimQuad(points []Point, from Point, command PathCommand) []Point {
	for step := 1; step <= iconTrimSteps; step++ {
		t := float32(step) / iconTrimSteps
		u := 1 - t
		points = append(points, Point{
			X: u*u*from.X + 2*u*t*command.Control1.X + t*t*command.Point.X,
			Y: u*u*from.Y + 2*u*t*command.Control1.Y + t*t*command.Point.Y,
		})
	}
	return points
}

// appendTrimCubic samples a cubic curve, from excluded and its end included.
func appendTrimCubic(points []Point, from Point, command PathCommand) []Point {
	for step := 1; step <= iconTrimSteps; step++ {
		t := float32(step) / iconTrimSteps
		u := 1 - t
		points = append(points, Point{
			X: u*u*u*from.X + 3*u*u*t*command.Control1.X + 3*u*t*t*command.Control2.X + t*t*t*command.Point.X,
			Y: u*u*u*from.Y + 3*u*u*t*command.Control1.Y + 3*u*t*t*command.Control2.Y + t*t*t*command.Point.Y,
		})
	}
	return points
}

func polylineLength(points []Point, closed bool) float32 {
	var total float32
	for index := 1; index < len(points); index++ {
		total += iconTrimDistance(points[index-1], points[index])
	}
	if closed && len(points) > 1 && points[len(points)-1] != points[0] {
		total += iconTrimDistance(points[len(points)-1], points[0])
	}
	return total
}

func lerpPoint(a, b Point, t float32) Point {
	return Point{X: a.X + (b.X-a.X)*t, Y: a.Y + (b.Y-a.Y)*t}
}

func iconTrimDistance(a, b Point) float32 {
	return float32(math.Hypot(float64(b.X-a.X), float64(b.Y-a.Y)))
}
