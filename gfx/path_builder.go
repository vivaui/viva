package gfx

import "math"

// PathBuilder assembles path geometry in a reusable command buffer, so a path
// derived from data every frame (a chart series, a sparkline) costs no heap
// allocation once the buffer has grown to its working size. The zero value is
// ready to use.
//
// Path returns a view of the buffer rather than a copy: it stays valid until the
// next Reset, and DrawPath never retains it. Use NewPath for geometry that
// outlives the builder.
type PathBuilder struct {
	commands []PathCommand
}

// Reset empties the builder and keeps its capacity.
func (builder *PathBuilder) Reset() {
	builder.commands = builder.commands[:0]
}

// Len is the number of commands recorded since the last Reset.
func (builder *PathBuilder) Len() int {
	return len(builder.commands)
}

// MoveTo begins a new contour.
func (builder *PathBuilder) MoveTo(x, y float32) {
	builder.commands = append(builder.commands, MoveTo(x, y))
}

// LineTo appends a straight segment to the current contour.
func (builder *PathBuilder) LineTo(x, y float32) {
	builder.commands = append(builder.commands, LineTo(x, y))
}

// QuadTo appends a quadratic curve to the current contour.
func (builder *PathBuilder) QuadTo(cx, cy, x, y float32) {
	builder.commands = append(builder.commands, QuadTo(cx, cy, x, y))
}

// CubicTo appends a cubic curve to the current contour.
func (builder *PathBuilder) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	builder.commands = append(builder.commands, CubicTo(c1x, c1y, c2x, c2y, x, y))
}

// Arc begins a contour at the start angle of the circle of radius around (cx, cy) and
// follows it clockwise (y down) by sweep degrees, as cubic curves of at most a quarter turn
// each. Angles are in degrees from the positive x axis, so -90 is twelve o'clock; a sweep of
// 360 or more makes a full circle, one of zero or less adds nothing.
func (builder *PathBuilder) Arc(cx, cy, radius, start, sweep float32) {
	if radius <= 0 || sweep <= 0 {
		return
	}
	sweep = min(sweep, 360)
	const toRadians = math.Pi / 180
	segments := int(math.Ceil(float64(sweep) / 90))
	step := float64(sweep) / float64(segments) * toRadians
	// The control points of a quarter-turn-or-less arc: 4/3 tan(step/4) of the radius.
	k := 4.0 / 3.0 * math.Tan(step/4) * float64(radius)
	angle := float64(start) * toRadians
	point := func(a float64) (x, y float64) {
		return float64(cx) + float64(radius)*math.Cos(a), float64(cy) + float64(radius)*math.Sin(a)
	}
	x0, y0 := point(angle)
	builder.MoveTo(float32(x0), float32(y0))
	for range segments {
		next := angle + step
		x1, y1 := point(next)
		builder.CubicTo(
			float32(x0-k*math.Sin(angle)), float32(y0+k*math.Cos(angle)),
			float32(x1+k*math.Sin(next)), float32(y1-k*math.Cos(next)),
			float32(x1), float32(y1))
		angle, x0, y0 = next, x1, y1
	}
}

// Close closes the current contour.
func (builder *PathBuilder) Close() {
	builder.commands = append(builder.commands, ClosePath())
}

// Path returns the recorded geometry in a width by height view box, or the empty
// path when the box is degenerate or nothing was recorded. The result aliases
// the builder's buffer and is invalidated by Reset and by further commands.
func (builder *PathBuilder) Path(width, height float32) Path {
	if width <= 0 || height <= 0 || len(builder.commands) == 0 {
		return Path{}
	}
	return Path{width: width, height: height, commands: builder.commands}
}

// PathBuilder returns the canvas's own builder, reset and ready. It is shared:
// the geometry from one call is invalid once the next call is made, so draw a
// path before building another. Code that needs several paths alive at once
// keeps its own PathBuilder values.
func (canvas *Canvas) PathBuilder() *PathBuilder {
	canvas.pathBuilder.Reset()
	return &canvas.pathBuilder
}
