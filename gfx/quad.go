package gfx

// Quad is a general quadrilateral: four corners in the order top-left,
// top-right, bottom-left, bottom-right, each with the colour it takes. The
// colours vary linearly across the shape, so one quad can paint a flat panel, a
// tilted sheet or a graded band. Unlike a Rect it need not be axis-aligned.
type Quad struct {
	Points [4]Point
	Tint   [4]Color
}

// quadWinding is the order the four corners go round in: top-left, top-right,
// bottom-right, bottom-left. Quad.Points puts the bottom two the other way
// round so that a quad reads like a rect.
var quadWinding = [4]int{0, 1, 3, 2}

// area is the signed area of the quad, in square logical pixels.
func (quad Quad) area() float32 {
	var sum float32
	for index, at := range quadWinding {
		next := quad.Points[quadWinding[(index+1)%len(quadWinding)]]
		point := quad.Points[at]
		sum += point.X*next.Y - next.X*point.Y
	}
	return sum / 2
}

// paintable reports whether the quad covers a visible area at all.
func (quad Quad) paintable() bool {
	if area := quad.area(); area > -0.5 && area < 0.5 {
		return false
	}
	for _, tint := range quad.Tint {
		if tint.Alpha > 0 {
			return true
		}
	}
	return false
}

// FillQuad fills a general quadrilateral: four corners in the order top-left,
// top-right, bottom-left, bottom-right, each with the colour that corner takes.
// The colours vary linearly across the shape. Unlike Fill it need not be
// axis-aligned; the quad is tessellated into triangles, so its outer edges are
// feathered like a path's instead of landing on pixel edges.
//
// The quad is clipped by Canvas.Clip and drawn on the current layer. A quad
// with no area or with every corner transparent paints nothing.
func (canvas *Canvas) FillQuad(quad Quad) {
	if !quad.paintable() {
		return
	}
	points, tints := quad.Points, quad.Tint
	// Two triangles share the diagonal. The first keeps the top and left edges
	// on the outline, the second the right and bottom edges; the diagonal stays
	// fully covered so the seam does not show.
	for _, half := range [2][3]int{{0, 1, 2}, {1, 3, 2}} {
		triangle := Triangle{
			Points: [3]Point{points[half[0]], points[half[1]], points[half[2]]},
		}
		colors := [3]Color{tints[half[0]], tints[half[1]], tints[half[2]]}
		triangle.Color = colors[0]
		triangle.Shade = [2]Color{colors[1], colors[2]}
		triangle.Shaded = colors[0] != colors[1] || colors[0] != colors[2]
		if half[0] == 0 {
			triangle.EdgeMask = 1<<0 | 1<<2
		} else {
			triangle.EdgeMask = 1<<0 | 1<<1
		}
		canvas.AppendTriangle(triangle)
	}
}

// LabelQuad appends a system-font line drawn into quad instead of its
// axis-aligned bounds, with the corner tints multiplied into the pixels at each
// corner. The platform rasterises the line at its Bounds size, so give Bounds
// the rectangle the line occupies untransformed, and let the quad place that
// picture. A quad is what tilts, foreshortens or shades a line drawn on a
// surface that is not facing the reader square on, such as a page being turned.
//
// One label is one straight line of text: a quad bends a line only as far as
// its four corners describe, so give a strongly curved surface one quad per
// line rather than a single quad for the page. A quad line is rasterised as a
// whole line and has no fixed-pitch grid runs; TextStyle.Mono still selects the
// fixed-pitch face.
func (canvas *Canvas) LabelQuad(value string, bounds Rect, quad Quad, style TextStyle) {
	canvas.LabelQuadAligned(value, bounds, quad, style, AlignStart)
}

// LabelQuadAligned appends a system-font line drawn into quad with explicit
// horizontal placement inside bounds, like LabelAligned inside a quad.
func (canvas *Canvas) LabelQuadAligned(value string, bounds Rect, quad Quad, style TextStyle, alignment Alignment) {
	if !quad.paintable() {
		return
	}
	originalX, originalWidth := bounds.X, bounds.Width
	value, bounds.X = canvas.placeLabel(value, bounds, style, alignment)
	if inset := bounds.X - originalX; inset > 0 && originalWidth > 0 {
		// The picture starts where the aligned line starts, so move the left
		// edge of the quad with it.
		frac := min(inset/originalWidth, 1)
		quad.Points[0] = lerpPoint(quad.Points[0], quad.Points[1], frac)
		quad.Points[2] = lerpPoint(quad.Points[2], quad.Points[3], frac)
	}
	text := Text{
		Value: value, Bounds: bounds, Style: style, Layer: canvas.currentLayer(),
		Quad: quad.Points, Tint: quad.Tint, HasQuad: true,
	}
	text.ClipOf, text.HasClip = canvas.clipSnapshot()
	canvas.texts = append(canvas.texts, text)
}
