package gfx

// AppendTriangle paints triangle at the canvas's current position, stamping it
// with the active layer and clip and with how many rectangles were painted
// before it. The last is what interleaves rectangles and triangles back into
// paint order when they are replayed (see Canvas.Triangles). A fully
// transparent triangle paints nothing and is dropped.
func (canvas *Canvas) AppendTriangle(triangle Triangle) {
	triangle.Layer = canvas.currentLayer()
	triangle.ClipOf, triangle.HasClip = canvas.clipSnapshot()
	triangle.RectsBefore = uint32(len(canvas.rectangles))
	if triangle.visible() {
		canvas.triangles = append(canvas.triangles, triangle)
	}
}
