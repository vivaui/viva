package gfx

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
