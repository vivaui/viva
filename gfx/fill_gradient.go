package gfx

// FillLinearGradient fills a rectangle with a two-color gradient. Start and End
// use logical canvas coordinates, just like bounds. Colors outside that segment
// are held at its endpoints; a zero-length segment uses To. Shaded mesh vertices
// reuse the existing native, software and web renderers without a bitmap cache.
func (canvas *Canvas) FillLinearGradient(bounds Rect, gradient LinearGradient) {
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return
	}
	bounds = canvas.snapRect(bounds, 0)
	axis := gradientAxis{origin: gradient.Start}
	dx, dy := gradient.End.X-gradient.Start.X, gradient.End.Y-gradient.Start.Y
	if length := dx*dx + dy*dy; length > 1e-12 {
		axis.dx, axis.dy = dx/length, dy/length
	} else {
		axis.base = 1
	}
	a, b := Point{bounds.X, bounds.Y}, Point{bounds.X + bounds.Width, bounds.Y}
	c, d := Point{bounds.X + bounds.Width, bounds.Y + bounds.Height}, Point{bounds.X, bounds.Y + bounds.Height}
	clip, clipped := canvas.clipSnapshot()
	for _, points := range [2][3]Point{{a, b, c}, {a, c, d}} {
		axis.each(points, 0, func(points [3]Point, edges uint8) {
			triangle := Triangle{Points: points, Layer: canvas.currentLayer(), ClipOf: clip, HasClip: clipped, RectsBefore: uint32(len(canvas.rectangles))}
			colorize(&triangle, &gradient, axis.vertexT(points), 1)
			if triangle.visible() {
				canvas.triangles = append(canvas.triangles, triangle)
			}
		})
	}
}
