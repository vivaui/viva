// Package framebuild reconciles the immediate canvas paint with the retained
// scene, the render store and the platform's text labels. It is the
// platform-neutral half of a frame: the native and web shells keep input,
// focus, accessibility, text input, native views and the cursor.
package framebuild

import (
	"math"
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

// Builder reconciles immediate canvas paints with the retained scene and the
// native label list, so an unchanged repaint performs no renderer work.
//
// A Builder owns the retained scene, render store and label state for one
// canvas. It is not safe for concurrent use: one goroutine calls Build once
// per frame, after the application's paint function has finished writing the
// canvas. The slices of the returned frame alias the builder's state and are
// valid until the next Build.
type Builder struct {
	canvas    *gfx.Canvas
	graph     *scene.Scene
	renderer  render.Store
	changes   []scene.Change
	resyncs   int        // times the renderer had to be rebuilt after a failed batch
	shapeIDs  []scene.ID // one scene node per entry of order, in paint order
	labels    []platform.TextLabel
	lastTexts []gfx.Text
	lastRuns  []gfx.TextRun // the runs lastTexts refer to, copied out of the application's memory
	// lastFontGeneration is the canvas's font generation at the labels last sent: text
	// that has not changed still needs sending when the fonts have, so the renderer
	// draws it again.
	lastFontGeneration uint32
	lastImages         []gfx.Image
	// labelImage names, for each label of this frame, the image it was drawn
	// from (an index into clippedImages, -1 for a text label); lastLabelImage is
	// the same for the labels last sent. A bitmap needs its pixels again unless
	// the label in its place showed the same picture: the platform hands a
	// label its predecessor's texture when both are at the same index and carry
	// the same content.
	labelImage       []int32
	lastLabelImage   []int32
	lastTextBefores  []uint32 // clippedTextBefores last sent
	lastImageBefores []uint32 // clippedImageBefores last sent
	stateBySlot      []instanceState
	// layerShift is 1 while the frame has a backdrop, which takes platform layer
	// zero; every canvas layer then travels one higher. It is 0 otherwise.
	layerShift uint16
	// cellReplace remembers the font-ready form of lines that hold cell glyphs;
	// a frame that changed one row of a grid-shaped page sends every row again,
	// and the rows that did not change rebuild no string and runs.
	cellReplace   []cellReplaceMemo
	clipRects     [4]gfx.Rect
	clipped       []gfx.Rectangle
	clippedPaths  []gfx.Triangle
	clippedSoft   []gfx.Shadow
	order         []shapeRef      // clipped, clippedPaths and clippedSoft interleaved in paint order
	blocks        []gfx.Rectangle // scratch for appendCellGlyphs
	strokes       []gfx.Triangle  // scratch for appendCellGlyphs
	clippedTexts  []gfx.Text
	clippedImages []gfx.Image
	// clippedTextBefores and clippedImageBefores are how many shapes the frame
	// paints before each of those labels: the place a renderer puts the label
	// back among the shapes.
	clippedTextBefores  []uint32
	clippedImageBefores []uint32
}

// New returns a builder for the canvas's current primitives.
func New(canvas *gfx.Canvas) *Builder {
	return &Builder{
		canvas:   canvas,
		graph:    scene.NewWithCapacity(len(canvas.Rectangles()) + len(canvas.Triangles()) + len(canvas.Shadows()) + 1),
		shapeIDs: make([]scene.ID, 0, len(canvas.Rectangles())+len(canvas.Triangles())+len(canvas.Shadows())),
		labels:   make([]platform.TextLabel, 0, len(canvas.Texts())+len(canvas.Images())+1),
	}
}

// Build reconciles the canvas with the scene and the label list and returns the
// resulting renderer frame.
func (builder *Builder) Build() (platform.Frame, error) {
	builder.applyClipping()
	builder.syncShapes()
	builder.changes = builder.graph.DrainChanges(builder.changes[:0])
	update, err := builder.renderer.Apply(builder.changes)
	if err != nil {
		// The scene has already absorbed this frame, so the failed batch will
		// never be sent again: leaving the store as it is would freeze every
		// node it skipped at its old value for good. Rebuild both from the
		// canvas instead.
		if update, err = builder.Resync(); err != nil {
			return platform.Frame{}, err
		}
	}
	if builder.stampState() && !update.Rebuild {
		update = render.Update{Count: len(builder.renderer.Rectangles()), Rebuild: true}
	}

	frame := platform.Frame{Update: update, Rectangles: builder.renderer.Rectangles()}
	if !builder.sameAsLastContent() {
		builder.syncLabels()
		builder.saveContent()
		frame.TextLabels = builder.labels
	}
	return frame, nil
}

// Rectangles returns the retained render instances in paint order.
func (builder *Builder) Rectangles() []render.Rectangle {
	return builder.renderer.Rectangles()
}

// Labels returns the labels of the last frame whose content changed.
func (builder *Builder) Labels() []platform.TextLabel {
	return builder.labels
}

// Graph returns the retained scene.
func (builder *Builder) Graph() *scene.Scene {
	return builder.graph
}

// Store returns the retained render store.
func (builder *Builder) Store() *render.Store {
	return &builder.renderer
}

// Resyncs reports how many times the render store had to be rebuilt after a
// failed batch.
func (builder *Builder) Resyncs() int {
	return builder.resyncs
}

// PlatformLayer is the layer a canvas layer takes on the platform. The topmost
// layer stays topmost.
func (builder *Builder) PlatformLayer(layer uint16) uint16 {
	if builder.layerShift == 0 || layer == math.MaxUint16 {
		return layer
	}
	return layer + builder.layerShift
}

// MarkInitialBitmapUploads makes every bitmap label of the initial frame carry
// its pixels, and remembers that frame as the content last sent.
func (builder *Builder) MarkInitialBitmapUploads() {
	for index := range builder.labels {
		builder.labels[index].BitmapUpload = len(builder.labels[index].Bitmap) > 0
	}
	builder.saveContent()
}

// Trim gives back the backing arrays of the builder's per-frame buffers when
// they are far larger than the current frame, and rebuilds the scene and store
// when a large page was replaced by a small one.
func (builder *Builder) Trim() {
	// A scene keeps its free slots for reuse. That is right for a page that
	// stays large, but after a large page is replaced by a small one the slots
	// are pure retention. Rebuild both from the small frame instead of carrying
	// the peak for the rest of the session.
	const trimAt, trimRatio = 4096, 4
	if slots := builder.graph.Slots(); slots > trimAt && builder.graph.Len()*trimRatio < slots {
		if _, err := builder.Resync(); err == nil {
			builder.stateBySlot = nil
		}
		// A page the scene rebuilt wholesale shows other lines; its memoized
		// replacements are stale retention.
		builder.cellReplace = builder.cellReplace[:0]
	}
	builder.shapeIDs = TrimCapacity(builder.shapeIDs)
	builder.labels = TrimCapacity(builder.labels)
	builder.lastTexts = TrimCapacity(builder.lastTexts)
	builder.lastRuns = TrimCapacity(builder.lastRuns)
	builder.lastImages = TrimCapacity(builder.lastImages)
	builder.labelImage = TrimCapacity(builder.labelImage)
	builder.lastLabelImage = TrimCapacity(builder.lastLabelImage)
	builder.clipped = TrimCapacity(builder.clipped)
	builder.clippedPaths = TrimCapacity(builder.clippedPaths)
	builder.clippedSoft = TrimCapacity(builder.clippedSoft)
	builder.order = TrimCapacity(builder.order)
	builder.blocks = TrimCapacity(builder.blocks)
	builder.strokes = TrimCapacity(builder.strokes)
	builder.clippedTexts = TrimCapacity(builder.clippedTexts)
	builder.clippedImages = TrimCapacity(builder.clippedImages)
	builder.clippedTextBefores = TrimCapacity(builder.clippedTextBefores)
	builder.clippedImageBefores = TrimCapacity(builder.clippedImageBefores)
	builder.lastTextBefores = TrimCapacity(builder.lastTextBefores)
	builder.lastImageBefores = TrimCapacity(builder.lastImageBefores)
	builder.changes = TrimCapacity(builder.changes)
}

// TrimCapacity gives back a slice's backing array when it is far larger than
// its contents. The contents are kept, so a frame already returned remains
// valid; its old array is released.
func TrimCapacity[T any](items []T) []T {
	const minCapacity = 4096
	const ratio = 4
	if cap(items) <= minCapacity || len(items)*ratio >= cap(items) {
		return items
	}
	next := make([]T, len(items), max(len(items)*2, minCapacity/64))
	copy(next, items)
	return next
}

// Resync discards the retained scene and render store and rebuilds both from
// the current canvas, returning the full-rebuild update.
func (builder *Builder) Resync() (render.Update, error) {
	builder.resyncs++
	builder.graph = scene.NewWithCapacity(len(builder.clipped) + len(builder.clippedPaths) + len(builder.clippedSoft) + 1)
	builder.renderer = render.Store{}
	builder.shapeIDs = builder.shapeIDs[:0]
	builder.syncShapes()
	builder.changes = builder.graph.DrainChanges(builder.changes[:0])
	return builder.renderer.Apply(builder.changes)
}

// Changes returns the scene changes of the last build.
func (builder *Builder) Changes() []scene.Change {
	return builder.changes
}

// ClippedRectangles returns the rectangles left by the canvas clip, in paint order.
func (builder *Builder) ClippedRectangles() []gfx.Rectangle {
	return builder.clipped
}

// ClippedTriangles returns the triangles left by the canvas clip, in paint order.
func (builder *Builder) ClippedTriangles() []gfx.Triangle {
	return builder.clippedPaths
}

// ClippedTexts returns the texts left by the canvas clip, in paint order.
func (builder *Builder) ClippedTexts() []gfx.Text {
	return builder.clippedTexts
}

// ClippedImages returns the images left by the canvas clip, in paint order.
func (builder *Builder) ClippedImages() []gfx.Image {
	return builder.clippedImages
}

// OrderLen reports how many shapes the frame paints.
func (builder *Builder) OrderLen() int {
	return len(builder.order)
}

// LabelImages names the image each label was drawn from, -1 for text.
func (builder *Builder) LabelImages() []int32 {
	return builder.labelImage
}

// LastLabelImages is LabelImages for the labels last sent.
func (builder *Builder) LastLabelImages() []int32 {
	return builder.lastLabelImage
}

// LastImages is the image list the labels last sent were built from.
func (builder *Builder) LastImages() []gfx.Image {
	return builder.lastImages
}

// shapeRef names one clipped shape: an entry of builder.clippedPaths when
// triangle is set, of builder.clippedSoft when shadow is set, of builder.clipped
// otherwise.
type shapeRef struct {
	index    int32
	triangle bool
	shadow   bool
}

// applyClipping drops every shape, image and text its canvas clip hides and
// notes, on the others, whether the clip cuts them: a shape left with HasClip
// set is drawn through a scissor rectangle by the renderer, which is how a
// partial shape is cut, so no geometry is cut here. It also merges rectangles,
// triangles, shadows, texts and images back into the order they were painted
// in: each primitive carries the paint order the canvas gave it.
func (builder *Builder) applyClipping() {
	builder.clipped = builder.clipped[:0]
	builder.clippedPaths = builder.clippedPaths[:0]
	builder.clippedSoft = builder.clippedSoft[:0]
	builder.clippedTexts = builder.clippedTexts[:0]
	builder.clippedImages = builder.clippedImages[:0]
	builder.clippedTextBefores = builder.clippedTextBefores[:0]
	builder.clippedImageBefores = builder.clippedImageBefores[:0]
	builder.order = builder.order[:0]
	builder.layerShift = 0
	rectangles := builder.canvas.Rectangles()
	triangles := builder.canvas.Triangles()
	shadows := builder.canvas.Shadows()
	texts := builder.canvas.Texts()
	images := builder.canvas.Images()
	// Walk the five lists as one paint order. A primitive with a smaller Order
	// is painted first; on a tie (a border ring shares its backing rectangle's
	// order, cell glyphs share their line's) shapes come before images and
	// images before text.
	ri, ti, si, xi, ii := 0, 0, 0, 0, 0
	for ri < len(rectangles) || ti < len(triangles) || si < len(shadows) || xi < len(texts) || ii < len(images) {
		kind, best := 0, ^uint32(0)
		if ri < len(rectangles) && rectangles[ri].Order < best {
			kind, best = 1, rectangles[ri].Order
		}
		if ti < len(triangles) && triangles[ti].Order < best {
			kind, best = 2, triangles[ti].Order
		}
		if si < len(shadows) && shadows[si].Order < best {
			kind, best = 3, shadows[si].Order
		}
		if ii < len(images) && images[ii].Order < best {
			kind, best = 4, images[ii].Order
		}
		if xi < len(texts) && texts[xi].Order < best {
			kind = 5
		}
		switch kind {
		case 1:
			builder.appendRectangle(rectangles[ri])
			ri++
		case 2:
			builder.appendTriangle(triangles[ti])
			ti++
		case 3:
			builder.appendShadow(shadows[si])
			si++
		case 4:
			builder.appendImage(images[ii])
			ii++
		case 5:
			builder.appendText(texts[xi])
			xi++
		}
	}
}

// appendImage adds image to the frame's label list unless its clip hides it or
// it has no area, cutting its source to the visible part. A backdrop is kept
// apart: its layer moves the rest of the frame above it.
func (builder *Builder) appendImage(image gfx.Image) {
	if RoundPixel(image.Bounds.Width) <= 0 || RoundPixel(image.Bounds.Height) <= 0 {
		return
	}
	if image.Backdrop {
		// A backdrop is beneath every layer, before any shape: it carries no
		// place among them.
		builder.layerShift = 1
		builder.clippedImages = append(builder.clippedImages, image)
		builder.clippedImageBefores = append(builder.clippedImageBefores, 0)
		return
	}
	if !image.HasClip {
		builder.clippedImages = append(builder.clippedImages, image)
		builder.clippedImageBefores = append(builder.clippedImageBefores, uint32(len(builder.order)))
		return
	}
	visible, ok := intersectVisible(image.Bounds, image.ClipOf)
	if !ok {
		return
	}
	if image.DevicePixels {
		// The clip may end between device pixels; the picture is cut at one.
		scale := builder.canvas.PixelScale()
		if visible = deviceBounds(visible, scale); visible.Width <= 0 || visible.Height <= 0 {
			return
		}
	}
	if visible != image.Bounds {
		sx := image.Source.Width / image.Bounds.Width
		sy := image.Source.Height / image.Bounds.Height
		image.Source.X += (visible.X - image.Bounds.X) * sx
		image.Source.Y += (visible.Y - image.Bounds.Y) * sy
		image.Source.Width = visible.Width * sx
		image.Source.Height = visible.Height * sy
		image.Bounds = visible
		if image.DevicePixels {
			image.Source = wholePixels(image.Source)
		}
	}
	builder.clippedImages = append(builder.clippedImages, image)
	builder.clippedImageBefores = append(builder.clippedImageBefores, uint32(len(builder.order)))
}

// appendText adds text to the frame's label list and paints the cell glyph
// geometry that belongs to it, unless the text paints nothing.
func (builder *Builder) appendText(text gfx.Text) {
	// A label with no area or no font size paints nothing, and the native label
	// backends require positive values. Transient layouts (a closing drawer, a
	// pane squeezed to nothing) can produce one; drop it here rather than failing
	// the whole frame.
	if RoundPixel(text.Bounds.Width) <= 0 || RoundPixel(text.Bounds.Height) <= 0 || text.Style.Size <= 0 {
		return
	}
	if text.HasClip && !textClipVisible(text) {
		return
	}
	builder.clippedTexts = append(builder.clippedTexts, text)
	if text.RunCount > 0 {
		builder.appendCellGlyphs(text)
	}
	// The cell glyph geometry is part of the line: it is painted before the
	// glyphs, so the label comes after the shapes just appended.
	builder.clippedTextBefores = append(builder.clippedTextBefores, uint32(len(builder.order)))
}

// appendShadow puts shadow in the frame's order after cutting it to its clip,
// and drops it when nothing of it is inside.
func (builder *Builder) appendShadow(shadow gfx.Shadow) {
	if shadow.HasClip {
		visible, cut := clipVisibility(shadowBounds(shadow), shadow.ClipOf)
		if !visible {
			return
		}
		shadow.HasClip = cut
	}
	builder.order = append(builder.order, shapeRef{index: int32(len(builder.clippedSoft)), shadow: true})
	builder.clippedSoft = append(builder.clippedSoft, shadow)
}

// appendCellGlyphs paints the geometry of a fixed-pitch line's cell glyphs — the Block Elements,
// Box Drawing, Braille, the corner shapes of Geometric Shapes, the Powerline separators and the
// Symbols for Legacy Computing — after the canvas's own, as the line's text is drawn after them
// (see gfx.Canvas.CellGlyphGeometry).
func (builder *Builder) appendCellGlyphs(text gfx.Text) {
	shapes := builder.canvas.CellGlyphGeometry(gfx.CellGlyphShapes{
		Rectangles: builder.blocks[:0],
		Triangles:  builder.strokes[:0],
	}, text)
	builder.blocks, builder.strokes = shapes.Rectangles, shapes.Triangles
	for _, rectangle := range builder.blocks {
		builder.appendRectangle(rectangle)
	}
	for _, triangle := range builder.strokes {
		builder.appendTriangle(triangle)
	}
}

// appendRectangle puts rectangle in the frame's order after cutting it to its clip, and drops it
// when nothing of it is inside.
func (builder *Builder) appendRectangle(rectangle gfx.Rectangle) {
	if rectangle.HasClip {
		visible, cut := clipVisibility(rectangle.Bounds, rectangle.ClipOf)
		if !visible {
			return
		}
		rectangle.HasClip = cut
	}
	builder.order = append(builder.order, shapeRef{index: int32(len(builder.clipped))})
	builder.clipped = append(builder.clipped, rectangle)
}

// appendTriangle is appendRectangle for the triangles a renderer feathers: their scissor keeps the
// half device pixel the edge reaches past the silhouette.
func (builder *Builder) appendTriangle(triangle gfx.Triangle) {
	if triangle.HasClip {
		bounds := triangleBounds(triangle.Points)
		if triangle.EdgeMask != 0 {
			half := 0.5 / builder.canvas.PixelScale()
			bounds.X -= half
			bounds.Y -= half
			bounds.Width += 2 * half
			bounds.Height += 2 * half
		}
		visible, cut := clipVisibility(bounds, triangle.ClipOf)
		if !visible {
			return
		}
		triangle.HasClip = cut
	}
	builder.order = append(builder.order, shapeRef{index: int32(len(builder.clippedPaths)), triangle: true})
	builder.clippedPaths = append(builder.clippedPaths, triangle)
}

// clipVisibility reports whether any of bounds lies inside clip and whether the
// clip cuts it, that is, whether part of bounds lies outside.
func clipVisibility(bounds, clip gfx.Rect) (visible, cut bool) {
	if bounds.X+bounds.Width <= clip.X || bounds.Y+bounds.Height <= clip.Y ||
		bounds.X >= clip.X+clip.Width || bounds.Y >= clip.Y+clip.Height {
		return false, false
	}
	return true, !ContainsRect(clip, bounds)
}

// shadowBounds is the region a shadow paints in: the rectangle that casts it
// grown by how far its blur, spread and offset reach.
func shadowBounds(shadow gfx.Shadow) gfx.Rect {
	extent := shadow.Style.Extent()
	return gfx.Rect{
		X: shadow.Bounds.X - extent, Y: shadow.Bounds.Y - extent,
		Width: shadow.Bounds.Width + 2*extent, Height: shadow.Bounds.Height + 2*extent,
	}
}

func triangleBounds(points [3]gfx.Point) gfx.Rect {
	left := min32(points[0].X, min32(points[1].X, points[2].X))
	top := min32(points[0].Y, min32(points[1].Y, points[2].Y))
	right := max32(points[0].X, max32(points[1].X, points[2].X))
	bottom := max32(points[0].Y, max32(points[1].Y, points[2].Y))
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func quadBounds(points [4]gfx.Point) gfx.Rect {
	left := min32(min32(points[0].X, points[1].X), min32(points[2].X, points[3].X))
	top := min32(min32(points[0].Y, points[1].Y), min32(points[2].Y, points[3].Y))
	right := max32(max32(points[0].X, points[1].X), max32(points[2].X, points[3].X))
	bottom := max32(max32(points[0].Y, points[1].Y), max32(points[2].Y, points[3].Y))
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}
}

func textClipVisible(text gfx.Text) bool {
	_, visible := intersectVisible(text.Bounds, text.ClipOf)
	if text.HasQuad {
		_, warped := intersectVisible(quadBounds(text.Quad), text.ClipOf)
		visible = visible || warped
	}
	return visible
}

func textClipCuts(text gfx.Text) bool {
	return !ContainsRect(text.ClipOf, text.Bounds) ||
		text.HasQuad && !ContainsRect(text.ClipOf, quadBounds(text.Quad))
}

func intersectVisible(bounds, clip gfx.Rect) (gfx.Rect, bool) {
	left := max32(bounds.X, clip.X)
	top := max32(bounds.Y, clip.Y)
	right := min32(bounds.X+bounds.Width, clip.X+clip.Width)
	bottom := min32(bounds.Y+bounds.Height, clip.Y+clip.Height)
	if right <= left || bottom <= top {
		return gfx.Rect{}, false
	}
	return gfx.Rect{X: left, Y: top, Width: right - left, Height: bottom - top}, true
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func min32(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// drawnBy reports whether a stored instance already draws a clipped shape.
// Both sides are compared field by field, as the scene stores them.
func (builder *Builder) drawnBy(stored *render.Rectangle, ref shapeRef) bool {
	if ref.triangle {
		triangle := &builder.clippedPaths[ref.index]
		return stored.Kind == scene.KindTriangle && stored.Color == scene.Color(triangle.Color) &&
			stored.EdgeMask == triangle.EdgeMask && stored.Shade == sceneShade(*triangle) &&
			stored.Points[0] == scene.Point(triangle.Points[0]) &&
			stored.Points[1] == scene.Point(triangle.Points[1]) &&
			stored.Points[2] == scene.Point(triangle.Points[2])
	}
	if ref.shadow {
		shadow := &builder.clippedSoft[ref.index]
		return stored.Kind == scene.KindShadow && stored.Radius == shadow.Radius &&
			stored.Color == scene.Color(shadow.Color) && stored.Bounds == scene.Rect(shadow.Bounds) &&
			stored.Shadow == scene.Shadow(shadow.Style)
	}
	rectangle := &builder.clipped[ref.index]
	return stored.Kind == scene.KindRectangle && stored.Radius == rectangle.Radius &&
		stored.Color == scene.Color(rectangle.Color) && stored.Bounds == scene.Rect(rectangle.Bounds)
}

// syncShapes aligns the scene's nodes with the clipped shapes, rectangles and
// triangles together in paint order, so the render store, which paints in node
// order, always equals the canvas.
//
// A repaint usually changes one stretch of the shapes: a highlight appears, a
// tab closes, a page swaps. The shapes before and after that stretch are left
// alone; inside it the nodes are rewritten in place (a node that held a
// rectangle may become a triangle and keeps its place), and the difference in
// length is inserted or removed at the end of the stretch. Cost follows the
// size of the change, not of the page, and no node is ever painted out of
// order.
func (builder *Builder) syncShapes() {
	root := builder.graph.Root()
	stored := builder.renderer.Rectangles() // what the previous frame painted
	oldCount, newCount := len(builder.shapeIDs), len(builder.order)
	if len(stored) != oldCount {
		// Not reachable while the store follows the scene; if it ever does not,
		// rewrite every node rather than trust the comparison below.
		stored = nil
	}

	prefix := 0
	for prefix < min(len(stored), newCount) && builder.drawnBy(&stored[prefix], builder.order[prefix]) {
		prefix++
	}
	suffix := 0
	for suffix < min(oldCount, newCount)-prefix && len(stored) == oldCount &&
		builder.drawnBy(&stored[oldCount-1-suffix], builder.order[newCount-1-suffix]) {
		suffix++
	}
	oldMiddle, newMiddle := oldCount-prefix-suffix, newCount-prefix-suffix

	for index := prefix; index < prefix+min(oldMiddle, newMiddle); index++ {
		builder.setShape(builder.shapeIDs[index], builder.order[index])
	}
	switch {
	case newMiddle > oldMiddle:
		at := prefix + oldMiddle
		add := newMiddle - oldMiddle
		var previous scene.ID
		if at > 0 {
			previous = builder.shapeIDs[at-1]
		}
		for range add {
			builder.shapeIDs = append(builder.shapeIDs, 0)
		}
		copy(builder.shapeIDs[at+add:], builder.shapeIDs[at:oldCount])
		for index := at; index < at+add; index++ {
			previous = builder.insertShape(root, previous, builder.order[index])
			builder.shapeIDs[index] = previous
		}
	case oldMiddle > newMiddle:
		from := prefix + newMiddle
		for _, id := range builder.shapeIDs[from : prefix+oldMiddle] {
			builder.graph.Remove(id)
		}
		builder.shapeIDs = slices.Delete(builder.shapeIDs, from, prefix+oldMiddle)
	}
}

// setShape makes an existing node draw a shape, whatever it drew before.
func (builder *Builder) setShape(id scene.ID, ref shapeRef) {
	if ref.triangle {
		value := SceneTriangle(builder.clippedPaths[ref.index])
		builder.graph.SetTriangle(id, value.Points, value.Color, value.EdgeMask, value.Shade)
		return
	}
	if ref.shadow {
		shadow := &builder.clippedSoft[ref.index]
		builder.graph.SetShadow(id, scene.Rect(shadow.Bounds), scene.Color(shadow.Color), shadow.Radius, scene.Shadow(shadow.Style))
		return
	}
	builder.setRectangle(id, builder.clipped[ref.index])
}

// insertShape adds a node drawing a shape right after previous.
func (builder *Builder) insertShape(root, previous scene.ID, ref shapeRef) scene.ID {
	if ref.triangle {
		value := SceneTriangle(builder.clippedPaths[ref.index])
		id, _ := builder.graph.InsertTriangle(root, previous, value.Points, value.Color, value.EdgeMask, value.Shade)
		return id
	}
	if ref.shadow {
		shadow := &builder.clippedSoft[ref.index]
		id, _ := builder.graph.InsertShadow(root, previous, scene.Rect(shadow.Bounds), scene.Color(shadow.Color), shadow.Radius, scene.Shadow(shadow.Style))
		return id
	}
	rectangle := builder.clipped[ref.index]
	value := SceneRectangle(rectangle)
	id, _ := builder.graph.InsertRoundedRectangle(root, previous, value.Bounds, value.Color, rectangle.Radius)
	return id
}

func (builder *Builder) setRectangle(id scene.ID, rectangle gfx.Rectangle) {
	value := SceneRectangle(rectangle)
	builder.graph.SetRoundedRectangle(id, value.Bounds, value.Color, rectangle.Radius)
}

// syncLabels rebuilds the label list. A bitmap label carries its pixels across
// the native boundary only when that bitmap is not exactly what the same place
// in the previous frame showed, so a picture made of many bitmaps (see
// gfx.Surface) stages just the ones that changed.
func (builder *Builder) syncLabels() {
	builder.labels = builder.labels[:0]
	builder.labelImage = builder.labelImage[:0]
	images := builder.clippedImages
	texts := builder.clippedTexts
	fontGeneration := builder.canvas.FontGeneration()
	// Images and texts are each in paint order; merge them back into one order,
	// because a label and a bitmap are both platform text labels. A backdrop is
	// beneath everything whatever its own paint order, so it goes first. Each
	// label carries how many shapes the frame painted before it, which is where
	// the renderers put it back among them.
	for ii := range images {
		if images[ii].Backdrop {
			builder.appendImageLabel(ii, builder.clippedImageBefores[ii])
		}
	}
	for ii, ti := 0, 0; ii < len(images) || ti < len(texts); {
		for ii < len(images) && images[ii].Backdrop {
			ii++
		}
		if ti >= len(texts) || (ii < len(images) && images[ii].Order <= texts[ti].Order) {
			builder.appendImageLabel(ii, builder.clippedImageBefores[ii])
			ii++
			continue
		}
		builder.appendTextLabel(texts[ti], fontGeneration, builder.clippedTextBefores[ti])
		ti++
	}
}

// appendImageLabel adds image number index of builder.clippedImages to the
// frame's labels, after shapesBefore shapes.
func (builder *Builder) appendImageLabel(index int, shapesBefore uint32) {
	image := builder.clippedImages[index]
	bitmap := image.Bitmap
	left := max(int(image.Source.X), 0)
	top := max(int(image.Source.Y), 0)
	right := min(int(math.Ceil(float64(image.Source.X+image.Source.Width))), bitmap.Width)
	bottom := min(int(math.Ceil(float64(image.Source.Y+image.Source.Height))), bitmap.Height)
	if left >= right || top >= bottom {
		return
	}
	offset := top*bitmap.Stride + left*4
	length := (bottom-top-1)*bitmap.Stride + (right-left)*4
	if offset < 0 || length <= 0 || offset > len(bitmap.Pix) || length > len(bitmap.Pix)-offset {
		return
	}
	width := RoundPixel(image.Bounds.Width)
	height := RoundPixel(image.Bounds.Height)
	if width <= 0 || height <= 0 {
		return
	}
	layer := builder.PlatformLayer(image.Layer)
	if image.Backdrop {
		layer = 0
	}
	// Rounded to logical pixels, unless the image is laid out in device pixels.
	x, y, w, h := float32(RoundPosition(image.Bounds.X)), float32(RoundPosition(image.Bounds.Y)), float32(width), float32(height)
	if image.DevicePixels {
		x, y, w, h = image.Bounds.X, image.Bounds.Y, image.Bounds.Width, image.Bounds.Height
	}
	builder.labels = append(builder.labels, platform.TextLabel{
		X: x, Y: y,
		Width: w, Height: h,
		Layer:        layer,
		ShapesBefore: shapesBefore,
		Bitmap:       bitmap.Pix[offset : offset+length], BitmapWidth: right - left, BitmapHeight: bottom - top,
		BitmapStride: bitmap.Stride, BitmapID: bitmap.ID, BitmapVersion: bitmap.Version,
		BitmapUpload: builder.bitmapUpload(image, index),
	})
	builder.labelImage = append(builder.labelImage, int32(index))
}

// bitmapUpload reports whether image number index must carry its pixels to the
// platform. It may omit them only when the label it is about to become showed
// the same picture in the frame last sent: the platform then reuses that
// label's texture. Another draw of the same bitmap elsewhere is no substitute,
// because the platform decides which label takes which old texture by position
// first.
func (builder *Builder) bitmapUpload(image gfx.Image, index int) bool {
	label := len(builder.labels)
	if image.Bitmap.ID == 0 || label >= len(builder.lastLabelImage) {
		return true
	}
	previous := int(builder.lastLabelImage[label])
	if previous < 0 || previous >= len(builder.lastImages) {
		return true
	}
	return !SameImageBitmap(image, builder.lastImages[previous])
}

// appendTextLabel adds one canvas text to the frame's labels.
func (builder *Builder) appendTextLabel(label gfx.Text, fontGeneration uint32, shapesBefore uint32) {
	width := RoundPixel(label.Bounds.Width)
	height := RoundPixel(label.Bounds.Height)
	if width <= 0 || height <= 0 {
		return
	}
	// Integer logical-pixel bounds keep the texture exactly one texel per
	// physical pixel so linear sampling stays crisp on scaled displays.
	entry := platform.TextLabel{
		Value:    label.Value,
		Identity: label.Identity,
		X:        float32(RoundPosition(label.Bounds.X)), Y: float32(RoundPosition(label.Bounds.Y)),
		Width: float32(width), Height: float32(height),
		OriginX: RoundingRemainder(label.Bounds.X), OriginY: RoundingRemainder(label.Bounds.Y),
		FontSize: label.Style.Size, Weight: label.Style.Weight, Italic: label.Style.Italic,
		Color: platform.Color{
			Red: float64(label.Style.Color.Red), Green: float64(label.Style.Color.Green),
			Blue: float64(label.Style.Color.Blue), Alpha: float64(label.Style.Color.Alpha),
		},
		Layer:        builder.PlatformLayer(label.Layer),
		ShapesBefore: shapesBefore,
		Mono:         label.Style.Mono, MonoFace: label.Style.MonoFace, CellWidth: label.CellWidth, Baseline: label.Baseline, Runs: builder.canvas.TextRuns(label),
	}
	if !label.Style.Mono && label.RunCount == 0 {
		// Only text in the interface font follows its generation.
		entry.FontGeneration = fontGeneration
	}
	if label.RunCount > 0 {
		// The font does not draw the cell glyphs; appendCellGlyphs did.
		entry.Value, entry.Runs = builder.replaceCellGlyphs(entry.Value, entry.Runs)
	}
	if label.HasClip {
		if textClipCuts(label) {
			// The clip cuts the label partway: the renderers scissor to it.
			entry.Clip, entry.Clipped = label.ClipOf, true
		}
	}
	if label.HasQuad {
		// A warped line keeps its rasterisation box and moves only where the
		// picture lands, so the platform draws the same texture as a quad.
		entry.Quad, entry.HasQuad = label.Quad, true
		for corner, tint := range label.Tint {
			entry.Tint[corner] = platform.Color{
				Red: float64(tint.Red), Green: float64(tint.Green),
				Blue: float64(tint.Blue), Alpha: float64(tint.Alpha),
			}
		}
	}
	builder.labels = append(builder.labels, entry)
	builder.labelImage = append(builder.labelImage, -1)
}

// cellReplaceLimit bounds the memoized cell glyph replacements: a screen holds
// its grid's rows and a little slack, and past the limit the memo starts over
// rather than keeping every line a session ever showed.
const cellReplaceLimit = 256

// cellReplaceMemo is one line's font-ready form: out is what ReplaceCellGlyphs
// built for in, kept while the same value and runs keep arriving.
type cellReplaceMemo struct {
	inValue, outValue string
	inRuns, outRuns   []gfx.TextRun
}

// replaceCellGlyphs is gfx.ReplaceCellGlyphs with a memory. A label sync that
// one changed row opened sends every row again; the rows that kept their value
// and runs get the replacement they already have instead of a fresh copy of the
// line and its runs, which is most of the garbage a keystroke in a bordered
// grid page otherwise makes.
func (builder *Builder) replaceCellGlyphs(value string, runs []gfx.TextRun) (string, []gfx.TextRun) {
	for i := range builder.cellReplace {
		if memo := &builder.cellReplace[i]; memo.inValue == value && slices.Equal(memo.inRuns, runs) {
			return memo.outValue, memo.outRuns
		}
	}
	outValue, outRuns := gfx.ReplaceCellGlyphs(value, runs)
	// A line without cell glyphs comes back as it went in; it has nothing to
	// remember, and its runs are the caller's own slice.
	if len(outRuns) == 0 || len(runs) == 0 || &outRuns[0] == &runs[0] {
		return outValue, outRuns
	}
	if len(builder.cellReplace) >= cellReplaceLimit {
		builder.cellReplace = builder.cellReplace[:0]
	}
	builder.cellReplace = append(builder.cellReplace, cellReplaceMemo{
		inValue: value, inRuns: append([]gfx.TextRun(nil), runs...),
		outValue: outValue, outRuns: outRuns,
	})
	return outValue, outRuns
}

// deviceBounds rounds the edges of bounds to whole device pixels of scale.
func deviceBounds(bounds gfx.Rect, scale float32) gfx.Rect {
	edge := func(value float32) float32 { return float32(math.Floor(float64(value)*float64(scale) + 0.5)) }
	left, top := edge(bounds.X), edge(bounds.Y)
	right, bottom := edge(bounds.X+bounds.Width), edge(bounds.Y+bounds.Height)
	return gfx.Rect{X: left / scale, Y: top / scale, Width: (right - left) / scale, Height: (bottom - top) / scale}
}

// wholePixels rounds the edges of a bitmap region to whole bitmap pixels.
func wholePixels(region gfx.Rect) gfx.Rect {
	left, top := math.Floor(float64(region.X)+0.5), math.Floor(float64(region.Y)+0.5)
	right, bottom := math.Floor(float64(region.X+region.Width)+0.5), math.Floor(float64(region.Y+region.Height)+0.5)
	return gfx.Rect{X: float32(left), Y: float32(top), Width: float32(right - left), Height: float32(bottom - top)}
}

// ContainsRect reports whether inner lies wholly inside outer.
func ContainsRect(outer, inner gfx.Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y &&
		inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

// RoundingRemainder is what RoundPosition took off value: the sub-pixel position
// backends that place text at device pixels draw it at (platform.TextLabel.OriginX).
func RoundingRemainder(value float32) float32 {
	return value - float32(RoundPosition(value))
}

// RoundPixel rounds a size to the nearest logical pixel, never below zero.
func RoundPixel(value float32) int32 {
	if value < 0 {
		return 0
	}
	return int32(value + 0.5)
}

// RoundPosition rounds a coordinate to the nearest logical pixel. Unlike a
// size, a position may be negative: a leaving drawer and its text move past the
// window edge together, and clamping either to zero would stop it at the edge.
func RoundPosition(value float32) int32 {
	return int32(math.Floor(float64(value) + 0.5))
}

// RoundPresenceRect puts a native overlay on the same whole logical pixels as
// the moving label it covers. Presence rounds a text label's origin so its
// texture is not rebuilt every frame; the platform editor next to it has to
// land on the same pixel.
func RoundPresenceRect(rect gfx.Rect) gfx.Rect {
	return gfx.Rect{
		X: float32(RoundPosition(rect.X)), Y: float32(RoundPosition(rect.Y)),
		Width: float32(RoundPixel(rect.Width)), Height: float32(RoundPixel(rect.Height)),
	}
}

// SceneRectangle converts a canvas rectangle to its render instance shape.
func SceneRectangle(rectangle gfx.Rectangle) render.Rectangle {
	return render.Rectangle{
		Kind: scene.KindRectangle,
		Bounds: scene.Rect{
			X: rectangle.Bounds.X, Y: rectangle.Bounds.Y,
			Width: rectangle.Bounds.Width, Height: rectangle.Bounds.Height,
		},
		Color: scene.Color{
			Red: rectangle.Color.Red, Green: rectangle.Color.Green,
			Blue: rectangle.Color.Blue, Alpha: rectangle.Color.Alpha,
		},
		Radius: rectangle.Radius,
		Layer:  rectangle.Layer,
	}
}

// SceneTriangle converts a canvas triangle to its render instance shape.
func SceneTriangle(triangle gfx.Triangle) render.Rectangle {
	return render.Rectangle{
		Kind: scene.KindTriangle,
		Points: [3]scene.Point{
			{X: triangle.Points[0].X, Y: triangle.Points[0].Y},
			{X: triangle.Points[1].X, Y: triangle.Points[1].Y},
			{X: triangle.Points[2].X, Y: triangle.Points[2].Y},
		},
		EdgeMask: triangle.EdgeMask,
		Color: scene.Color{
			Red: triangle.Color.Red, Green: triangle.Color.Green,
			Blue: triangle.Color.Blue, Alpha: triangle.Color.Alpha,
		},
		Shade: sceneShade(triangle),
		Layer: triangle.Layer,
	}
}

// sceneShade is the vertex colors of a shaded triangle, and the zero Shade for a
// flat one, whatever stale colors its Shade array holds.
func sceneShade(triangle gfx.Triangle) scene.Shade {
	if !triangle.Shaded {
		return scene.Shade{}
	}
	return scene.Shade{On: true, Colors: [2]scene.Color{scene.Color(triangle.Shade[0]), scene.Color(triangle.Shade[1])}}
}

// instanceState is what a render instance knows about its place on screen that
// the scene does not carry: the layer it composes in and the clip that cuts it.
type instanceState struct {
	layer   uint16
	clipped bool
	clip    scene.Rect
}

// stampState copies each primitive's canvas layer and clip onto its render
// instance and reports whether either changed anywhere, which a partial update
// would not carry to the GPU. Instances are listed by the scene's tree order,
// which syncShapes keeps equal to paint order, so the shape at index i is the
// instance at index i; should the store ever disagree, the state travels with
// the node's storage slot instead.
func (builder *Builder) stampState() (changed bool) {
	instances := builder.renderer.Rectangles()
	if len(instances) != len(builder.shapeIDs) {
		return builder.stampBySlot()
	}
	for index, id := range builder.shapeIDs {
		instance := &instances[index]
		if instance.ID != id {
			return builder.stampBySlot()
		}
		state := builder.shapeState(builder.order[index])
		if instance.Layer != state.layer || instance.Clipped != state.clipped || instance.Clip != state.clip {
			instance.Layer, instance.Clipped, instance.Clip = state.layer, state.clipped, state.clip
			changed = true
		}
	}
	return changed
}

// stampBySlot is stampState for a store whose order is not the paint order.
func (builder *Builder) stampBySlot() (changed bool) {
	for index, id := range builder.shapeIDs {
		slot, ok := id.Slot()
		if !ok {
			continue
		}
		for int(slot) >= len(builder.stateBySlot) {
			builder.stateBySlot = append(builder.stateBySlot, instanceState{})
		}
		builder.stateBySlot[slot] = builder.shapeState(builder.order[index])
	}
	instances := builder.renderer.Rectangles()
	for index := range instances {
		slot, _ := instances[index].ID.Slot()
		state := builder.stateBySlot[slot]
		if instances[index].Layer != state.layer || instances[index].Clipped != state.clipped || instances[index].Clip != state.clip {
			instances[index].Layer, instances[index].Clipped, instances[index].Clip = state.layer, state.clipped, state.clip
			changed = true
		}
	}
	return changed
}

// shapeState is the layer and clip the platform draws a shape with.
func (builder *Builder) shapeState(ref shapeRef) instanceState {
	var state instanceState
	if ref.shadow {
		shadow := &builder.clippedSoft[ref.index]
		state = instanceState{layer: shadow.Layer, clipped: shadow.HasClip}
		if shadow.HasClip {
			state.clip = scene.Rect(shadow.ClipOf)
		}
	} else if ref.triangle {
		triangle := &builder.clippedPaths[ref.index]
		state = instanceState{layer: triangle.Layer, clipped: triangle.HasClip}
		if triangle.HasClip {
			state.clip = scene.Rect(triangle.ClipOf)
		}
	} else {
		rectangle := &builder.clipped[ref.index]
		state = instanceState{layer: rectangle.Layer, clipped: rectangle.HasClip}
		if rectangle.HasClip {
			state.clip = scene.Rect(rectangle.ClipOf)
		}
	}
	state.layer = builder.PlatformLayer(state.layer)
	return state
}

// sameAsLastContent reports whether the texts to draw are exactly those last
// sent to the platform.
func (builder *Builder) sameAsLastContent() bool {
	if builder.canvas.FontGeneration() != builder.lastFontGeneration || !sameImages(builder.clippedImages, builder.lastImages) {
		return false
	}
	// A label's place among the shapes can move while the label itself does
	// not: a shape before it can drop out of the clip, or paint nothing, and
	// then the renderers must be told where the label goes.
	if !slices.Equal(builder.clippedTextBefores, builder.lastTextBefores) ||
		!slices.Equal(builder.clippedImageBefores, builder.lastImageBefores) {
		return false
	}
	left, right := builder.clippedTexts, builder.lastTexts
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a.Value != b.Value || a.Style != b.Style || a.Identity != b.Identity || a.Layer != b.Layer || a.Order != b.Order {
			return false
		}
		if a.Bounds != b.Bounds || a.CellWidth != b.CellWidth || a.Baseline != b.Baseline || a.RunCount != b.RunCount {
			return false
		}
		if a.HasClip != b.HasClip || a.HasClip && a.ClipOf != b.ClipOf {
			return false
		}
		// Where a warped line lands changes every frame of an animation while
		// its picture stays the same: the platform must still see the new quad.
		if a.HasQuad != b.HasQuad {
			return false
		}
		if a.HasQuad && (a.Quad != b.Quad || a.Tint != b.Tint) {
			return false
		}
		if a.RunCount == 0 {
			continue
		}
		current := builder.canvas.TextRuns(a)
		if len(current) != int(a.RunCount) || b.RunFirst < 0 || b.RunCount < 0 || int(b.RunFirst+b.RunCount) > len(builder.lastRuns) {
			return false
		}
		saved := builder.lastRuns[b.RunFirst : b.RunFirst+b.RunCount]
		for run := range current {
			if current[run] != saved[run] {
				return false
			}
		}
	}
	return true
}

func sameImages(left, right []gfx.Image) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		a, b := left[index], right[index]
		if a.Bounds != b.Bounds || a.Source != b.Source || a.Layer != b.Layer || a.Order != b.Order || a.Backdrop != b.Backdrop || a.Bitmap.Width != b.Bitmap.Width ||
			a.Bitmap.Height != b.Bitmap.Height || a.Bitmap.Stride != b.Bitmap.Stride || a.Bitmap.ID != b.Bitmap.ID ||
			a.Bitmap.Version != b.Bitmap.Version || len(a.Bitmap.Pix) != len(b.Bitmap.Pix) {
			return false
		}
		if len(a.Bitmap.Pix) > 0 && &a.Bitmap.Pix[0] != &b.Bitmap.Pix[0] {
			return false
		}
	}
	return true
}

// SameImageBitmap reports whether two draws show the very same pixels: the same
// identity and version, source region and storage. A bitmap with no identity
// (ID 0) is never the same: nothing says its pixels did not change.
func SameImageBitmap(a, b gfx.Image) bool {
	if a.Source != b.Source || a.Bitmap.Width != b.Bitmap.Width || a.Bitmap.Height != b.Bitmap.Height ||
		a.Bitmap.Stride != b.Bitmap.Stride || a.Bitmap.ID != b.Bitmap.ID || a.Bitmap.Version != b.Bitmap.Version ||
		len(a.Bitmap.Pix) != len(b.Bitmap.Pix) || a.Bitmap.ID == 0 {
		return false
	}
	return len(a.Bitmap.Pix) == 0 || &a.Bitmap.Pix[0] == &b.Bitmap.Pix[0]
}

// saveContent remembers this frame's texts for the next comparison. Their runs
// live in the canvas, which is rewritten by the next paint, so they are copied.
func (builder *Builder) saveContent() {
	builder.lastFontGeneration = builder.canvas.FontGeneration()
	builder.lastImages = append(builder.lastImages[:0], builder.clippedImages...)
	builder.lastLabelImage = append(builder.lastLabelImage[:0], builder.labelImage...)
	builder.lastTexts = append(builder.lastTexts[:0], builder.clippedTexts...)
	builder.lastTextBefores = append(builder.lastTextBefores[:0], builder.clippedTextBefores...)
	builder.lastImageBefores = append(builder.lastImageBefores[:0], builder.clippedImageBefores...)
	builder.lastRuns = builder.lastRuns[:0]
	for index := range builder.lastTexts {
		text := &builder.lastTexts[index]
		runs := builder.canvas.TextRuns(*text)
		text.RunFirst = int32(len(builder.lastRuns))
		builder.lastRuns = append(builder.lastRuns, runs...)
	}
}
