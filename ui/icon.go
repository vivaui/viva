package ui

import "math"

// Icon is one compiled vector glyph: immutable Path commands plus stroke
// metadata, produced ahead of time by the `viva icons` compiler from SVG
// sources. Icons carry no parser at runtime; an application embeds the
// generated Go values and paints them at any size in any tint.
//
//	var chevron = ui.Icon{
//	    ViewBox: 24,
//	    Stroke:  2,
//	    Cap:     ui.CapRound,
//	    Join:    ui.JoinRound,
//	    Commands: []ui.PathCommand{
//	        ui.MoveTo(6, 9), ui.QuadTo(12, 15, 18, 9),
//	    },
//	}
//	canvas.DrawIcon(chevron, bounds, theme.Foreground)
type Icon struct {
	// ViewBox is the source coordinate width, and the height too unless ViewHeight
	// says otherwise (Lucide's square icons use 24).
	ViewBox float32
	// ViewHeight is the source coordinate height of an icon that is not square,
	// such as a horizontal logo. Zero means the icon is square, ViewBox high.
	ViewHeight float32
	// Stroke is the source stroke width in view-box units (Lucide uses 2).
	Stroke float32
	// Cap and Join describe the source stroke geometry.
	Cap  StrokeCap
	Join StrokeJoin
	// Commands is the compiled path vocabulary, stroked (or filled when Stroke
	// is zero) in the tint DrawIcon is given.
	Commands []PathCommand
	// Layers are filled shapes with colors of their own, drawn after Commands
	// in order. They are what a multi-color glyph (a file-type icon, a brand
	// mark) is made of; the tint contributes only its alpha to them.
	Layers []IconLayer
}

// IconLayer is one filled shape of a multi-color icon.
type IconLayer struct {
	// Fill is the solid color. With a Gradient it is only the color a tool that
	// ignores gradients would use.
	Fill     Color
	FillRule PathFillRule
	// Gradient, when set, replaces Fill with a linear gradient.
	Gradient *LinearGradient
	Commands []PathCommand
}

// LinearGradient is a two-color linear gradient in an icon's view-box units: From
// at Start, To at End, held flat beyond both (SVG's spreadMethod "pad"), mixed as
// straight RGBA like SVG. Renderers interpolate colors across a triangle, so the
// canvas cuts the layer's triangles at the two lines where the gradient starts and
// ends: the pieces beyond are flat, the ones between carry the gradient as the
// colors of their vertices, which is exact. The tessellation is cached with the
// icon, so a gradient costs a replay no more than a solid layer.
type LinearGradient struct {
	Start, End Point
	From, To   Color
}

// At returns the gradient color at t, 0 at Start and 1 at End.
func (gradient LinearGradient) At(t float32) Color {
	if t <= 0 {
		return gradient.From
	}
	if t >= 1 {
		return gradient.To
	}
	return Color{
		Red:   gradient.From.Red + (gradient.To.Red-gradient.From.Red)*t,
		Green: gradient.From.Green + (gradient.To.Green-gradient.From.Green)*t,
		Blue:  gradient.From.Blue + (gradient.To.Blue-gradient.From.Blue)*t,
		Alpha: gradient.From.Alpha + (gradient.To.Alpha-gradient.From.Alpha)*t,
	}
}

// height is the icon's source height.
func (icon Icon) height() float32 {
	if icon.ViewHeight > 0 {
		return icon.ViewHeight
	}
	return icon.ViewBox
}

// IconTransform moves a painted icon away from the box it was laid out in:
// scale and rotation about the center of that box, then an offset. The zero
// value is the icon exactly where the layout put it, so a still frame and the
// end of a motion paint the same pixels.
//
// The glyph is drawn on whole device pixels (see DrawIcon), so a rotation that
// does not finish on a right angle leaves it off that grid: motion that turns an
// icon should end at the still icon, and only a glyph that is symmetric under
// the angle it turns through stays crisp while it turns.
type IconTransform struct {
	// Rotate turns the icon clockwise, in radians, about the center of its box.
	Rotate float32
	// Scale grows the glyph, stroke included, about the center of its box. Zero
	// is 1; a negative value mirrors it.
	Scale float32
	// ScaleY stretches the glyph vertically on top of Scale, about the same
	// point. Zero is 1, so a transform that sets only Scale stays uniform; a
	// negative value mirrors it.
	ScaleY float32
	// Anchor is the point of the glyph Scale, ScaleY and Rotate act about, in
	// view-box units. Nil is the centre of the icon's box.
	Anchor *Point
	// Translate moves the icon after it is scaled and turned, in view-box units:
	// one is one unit of the source glyph. Offset in logical pixels is added
	// after it. A view-box offset keeps a gesture's proportion when the same
	// icon is painted at another size.
	Translate Point
	// Offset moves the icon in logical pixels after the rotation and scale: x to
	// the right and y down.
	Offset Point
	// Parts move single contours of the glyph before the transform above, so a
	// part's offset travels with the glyph when it grows. Listing a part that
	// does not move costs nothing: the glyph is drawn whole until one of them
	// does.
	//
	// A part is what one contour of the source is — a lock's shackle, a
	// keyboard's keys — so parts allow a glyph's pieces to move relative to each
	// other while it stays one icon in the layout.
	Parts []IconPart
	// Reveal writes the glyph on from its right edge instead of painting it
	// whole: Reveal is the fraction written so far, one being all of it, and the
	// rest is clipped away. Zero is the whole glyph, like the zero of every other
	// field here, so a motion that writes a glyph on runs from just above zero to
	// one and ends on the still icon.
	//
	// The cut is made at the icon's box, on whole device pixels, so a glyph that
	// turns or grows out of that box is clipped with it.
	Reveal float32
	// ClipToBox clips the painted glyph to the box it was laid out in, on whole
	// device pixels. A part that moves outside the box disappears at its edge
	// instead of painting over its neighbours.
	ClipToBox bool
	// Clip clips the painted glyph to a convex shape in the icon's view box. The
	// shape moves, turns and scales with the whole glyph, not with a part that
	// moves inside it. Nil paints the whole glyph.
	Clip *IconClip
}

// IconClip is a convex clip shape in an icon's view-box units: a rounded
// rectangle, or a circle when Radius reaches half the shorter side. It is
// rasterized by cutting the icon's triangles before they reach the canvas, so
// it needs no renderer mask or extra draw state.
type IconClip struct {
	Bounds Rect
	// Radius rounds the rectangle's corners. Zero leaves square corners.
	Radius float32
}

// iconUnits is the canvas size of one view-box unit along each axis. A glyph
// keeps its source aspect ratio, so an icon painted into a target has one scale
// per axis; keeping the axes separate makes every placement use the same
// conversion as the path it moves.
type iconUnits struct {
	x, y float32
}

// unitsAt is the view-box to canvas scale for an icon painted into target.
func iconUnitsAt(icon Icon, target Rect) iconUnits {
	return iconUnits{x: target.Width / icon.ViewBox, y: target.Height / icon.height()}
}

// point places a point given in view-box units into the tessellation.
func (units iconUnits) point(point Point) Point {
	return Point{X: point.X * units.x, Y: point.Y * units.y}
}

// IconPart moves one contour of an icon on its own. Contours are numbered in
// drawing order from zero; Icon.Contours yields them as icons, which is how a
// caller finds out which contour is which.
//
// The number is a handle on a particular drawing, not a name: regenerating a
// glyph may insert, merge or reorder its contours, and the same index then moves
// a different part. An index that no contour has, or a second part on the same
// contour, is ignored rather than reported. A component that addresses contours
// must therefore pin which one it means with a test that paints the glyph and
// checks the part moves where it should.
type IconPart struct {
	// Contour is the index of the contour this part moves.
	Contour int
	// Rotate turns the part clockwise, in radians, about the center of its own
	// bounds rather than the glyph's, so a part turned in place stays put.
	Rotate float32
	// Scale grows the part about the center of its own bounds. Zero is 1.
	Scale float32
	// ScaleX stretches the part horizontally, on top of Scale, about the same
	// point. Zero is 1 and a negative value mirrors it: passing through zero is
	// the halfway point of a page turning over, or of a meridian crossing a globe.
	ScaleX float32
	// ScaleY stretches the part vertically, on top of Scale, about the same
	// point. Zero is 1 and a negative value mirrors it.
	ScaleY float32
	// Translate moves the part after its scale, in view-box units, and then
	// Offset moves it in logical pixels.
	Translate Point
	// Offset moves the part in logical pixels: x to the right and y down.
	Offset Point
	// Anchor is the point of the glyph a part turns and scales about, in the
	// icon's own view-box units. Nil means the centre of the part's own bounds,
	// so a part turned in place stays put; an anchor is how a page turns over a
	// book's spine rather than twisting around itself.
	Anchor *Point
	// Fade makes the part transparent: zero paints it at the icon's tint, one
	// leaves nothing. It affects the stroked contours; a filled Layer has no
	// contour to fade on its own.
	Fade float32
	// Trim writes part of the contour on. Its range is measured along the
	// contour, including a closing segment, and every part trims its own
	// contour. Filled icons have no stroke to trim.
	Trim IconTrim
}

// IconTrim is the visible fraction of one stroked contour, measured from its
// start. Active is the opt-in, so the zero IconTrim leaves the contour whole.
// From and To are clamped to [0, 1]; From greater than To is swapped. The full
// range paints the source commands unchanged.
type IconTrim struct {
	Active   bool
	From, To float32
}

// rangeValues is the trim's visible interval with its ends in order and
// clamped to the contour.
func (trim IconTrim) rangeValues() (from, to float32) {
	from, to = min(max(trim.From, 0), 1), min(max(trim.To, 0), 1)
	if from > to {
		from, to = to, from
	}
	return from, to
}

// moves reports whether the part changes the painted contour.
func (part IconPart) moves() bool {
	return part.Rotate != 0 || part.Translate != (Point{}) || part.Offset != (Point{}) ||
		part.stretches() || part.Scale != 0 && part.Scale != 1 || part.fades() || part.trims()
}

// stretches reports whether the part scales away from one on either axis.
func (part IconPart) stretches() bool {
	return part.ScaleX != 0 && part.ScaleX != 1 || part.ScaleY != 0 && part.ScaleY != 1
}

// fades reports whether the part is less opaque than its glyph.
func (part IconPart) fades() bool {
	return part.Fade > 0
}

// trims reports whether the part paints less than its whole contour.
func (part IconPart) trims() bool {
	if !part.Trim.Active {
		return false
	}
	from, to := part.Trim.rangeValues()
	return from > 0 || to < 1
}

// Contours splits the icon into one icon per contour of its commands, in
// drawing order. Each keeps the view box, the stroke and the caps of the whole
// glyph, so it draws on the same box and differs only in which contour reaches
// the canvas.
//
// Parts of a glyph are separate contours in the source — the shackle of a lock,
// the keys of a keyboard — so moving one is a matter of drawing that contour
// with its own transform. Filled layer colors belong to the whole glyph and are
// left out of the contours.
func (icon Icon) Contours() []Icon {
	return icon.appendContours(nil)
}

// appendContours is Contours into a caller's own buffer, so a paint that runs
// every frame does not allocate the slice again. The returned slice may share
// dst's backing array.
func (icon Icon) appendContours(dst []Icon) []Icon {
	start := -1
	for index, command := range icon.Commands {
		if command.Kind != PathCommandMove {
			continue
		}
		if start >= 0 {
			dst = append(dst, icon.contour(start, index))
		}
		start = index
	}
	if start >= 0 {
		dst = append(dst, icon.contour(start, len(icon.Commands)))
	}
	return dst
}

// contour is one slice of the icon's commands drawn as an icon of its own. The
// slice is capped at its length, so a caller that appends to one contour cannot
// overwrite the next.
func (icon Icon) contour(from, to int) Icon {
	part := icon
	part.Commands = icon.Commands[from:to:to]
	part.Layers = nil
	return part
}

// paints reports whether the icon has anything to draw at this tint's alpha.
func (icon Icon) paints(tint Color) bool {
	return tint.Alpha > 0 && icon.ViewBox > 0 && (len(icon.Commands) > 0 || len(icon.Layers) > 0)
}

// moves reports whether the transform changes the painted icon.
func (transform IconTransform) moves() bool {
	return transform.Rotate != 0 || transform.Translate != (Point{}) || transform.Offset != (Point{}) ||
		transform.Scale != 0 && transform.Scale != 1 || transform.scaleY() != 1 ||
		transform.Anchor != nil && (transform.Scale != 0 && transform.Scale != 1 || transform.scaleY() != 1 || transform.Rotate != 0) ||
		transform.partsMove() || transform.reveals() || transform.ClipToBox || transform.Clip != nil
}

// geometry reports whether the transform changes the glyph's points. A
// transform that only clips keeps the still tessellation and can replay the
// paint cache.
func (transform IconTransform) geometry() bool {
	return transform.Rotate != 0 || transform.Translate != (Point{}) || transform.Offset != (Point{}) ||
		transform.Scale != 0 && transform.Scale != 1 || transform.scaleY() != 1 || transform.partsMove()
}

// reveals reports whether the transform writes part of the glyph rather than all
// of it. One is the whole glyph, which is what a motion ends on.
func (transform IconTransform) reveals() bool {
	return transform.Reveal > 0 && transform.Reveal < 1
}

// scaleY is the vertical stretch on top of Scale, with zero read as one.
func (transform IconTransform) scaleY() float32 {
	if transform.ScaleY == 0 {
		return 1
	}
	return transform.ScaleY
}

// partsMove reports whether any listed contour moves on its own.
func (transform IconTransform) partsMove() bool {
	for _, part := range transform.Parts {
		if part.moves() {
			return true
		}
	}
	return false
}

// DrawIcon paints one icon tinted with a single color, scaled uniformly
// to fit bounds and centered. The icon strokes scale with the icon, but its size
// and origin are rounded to whole device pixels and a stroke up to half a device
// pixel (see SetPixelScale), never thinner than one: a 16 point Lucide icon draws
// a 1.33 point stroke, which whole-pixel rounding turned into a single hairline at
// 100% but 1.5 points at 200%, so the same icon looked a third fainter on a 1x
// display.
//
// A multi-color icon paints each of its layers in the layer's own color; the
// tint's alpha fades the whole glyph and its color is not used.
func (canvas *Canvas) DrawIcon(icon Icon, bounds Rect, tint Color) {
	if !icon.paints(tint) {
		return
	}
	target, ok := canvas.iconTarget(icon, bounds)
	if !ok {
		return
	}
	canvas.drawIconCached(icon, target.Width, Point{X: target.X, Y: target.Y}, tint)
}

// DrawIconTransformed paints an icon like DrawIcon, moved by transform: scaled
// and turned about the center of its box, then offset. The transform is applied
// to the glyph's triangles, so a frame of motion costs the cached tessellation
// and a transform per vertex, not a new icon.
//
// A zero transform is exactly DrawIcon, and that path stays on the paint cache:
// an icon that is not in motion pays nothing for being able to move.
func (canvas *Canvas) DrawIconTransformed(icon Icon, bounds Rect, tint Color, transform IconTransform) {
	if !transform.moves() {
		canvas.DrawIcon(icon, bounds, tint)
		return
	}
	if !icon.paints(tint) {
		return
	}
	target, ok := canvas.iconTarget(icon, bounds)
	if !ok {
		return
	}
	if transform.reveals() {
		canvas.Clip(canvas.iconRevealClip(target, transform.Reveal))
		defer canvas.Unclip()
	}
	if transform.ClipToBox {
		canvas.Clip(target)
		defer canvas.Unclip()
	}
	if !transform.geometry() {
		canvas.drawIconClipped(icon, target, tint, transform)
		return
	}
	scale := transform.Scale
	if scale == 0 {
		scale = 1
	}
	if transform.ScaleY == 0 {
		// ScaleY is a stretch on top of Scale; zero does not switch it off.
		transform.ScaleY = 1
	}
	units := iconUnitsAt(icon, target)
	center := Point{X: target.Width * 0.5, Y: target.Height * 0.5}
	if transform.Anchor != nil {
		center = units.point(*transform.Anchor)
	}
	offset := Point{
		X: transform.Translate.X*units.x + transform.Offset.X,
		Y: transform.Translate.Y*units.y + transform.Offset.Y,
	}
	place := iconPlacementAtSize(center, offset, transform.Rotate, scale, scale*transform.scaleY())
	place.origin = Point{X: target.X, Y: target.Y}
	var clip *iconClipShape
	if transform.Clip != nil {
		shape := iconClipShapeAt(*transform.Clip, units, place)
		clip = &shape
	}
	if transform.partsMove() {
		canvas.drawIconPartsClipped(icon, target, tint, place, transform.Parts, clip)
		return
	}
	canvas.drawIconPlacedClipped(icon, target.Width, tint, place, clip)
}

// drawIconClipped paints an unmoved icon through its local clip. The still glyph
// can replay the paint cache for the unchanged triangles.
func (canvas *Canvas) drawIconClipped(icon Icon, target Rect, tint Color, transform IconTransform) {
	if transform.Clip == nil {
		canvas.DrawIcon(icon, target, tint)
		return
	}
	place := stillPlacement(Point{X: target.X, Y: target.Y})
	clip := iconClipShapeAt(*transform.Clip, iconUnitsAt(icon, target), place)
	canvas.drawIconPlacedClipped(icon, target.Width, tint, place, &clip)
}

// iconRevealClip is the written part of an icon's box: from the edge the glyph is
// being written up to, to the right of the box. The edge lands on whole device
// pixels, which is the grid the glyph is painted on, so a reveal steps through as
// many clips as the box is wide in pixels.
func (canvas *Canvas) iconRevealClip(target Rect, reveal float32) Rect {
	edge := canvas.snapToDevice(target.X + target.Width*(1-reveal))
	return Rect{X: edge, Y: target.Y, Width: target.X + target.Width - edge, Height: target.Height}
}

// iconTarget is where and how large DrawIcon paints an icon in bounds, on whole device pixels.
func (canvas *Canvas) iconTarget(icon Icon, bounds Rect) (Rect, bool) {
	width, height := icon.fitted(bounds)
	if width <= 0 || height <= 0 {
		return Rect{}, false
	}
	// Stay in device pixels until placement is rounded. Converting a snapped
	// width back to float32 first can turn an exact half-pixel origin into a
	// value just below the tie (e.g. a 14px icon at 125%), shifting it up/left.
	scale := float64(canvas.PixelScale())
	pixelWidth, pixelHeight := float64(width)*scale, float64(height)*scale
	if snapped := math.Floor(pixelWidth + 0.5); snapped > 0 {
		pixelHeight *= snapped / pixelWidth
		pixelWidth = snapped
	}
	x := (float64(bounds.X)+float64(bounds.Width)*0.5)*scale - pixelWidth*0.5
	y := (float64(bounds.Y)+float64(bounds.Height)*0.5)*scale - pixelHeight*0.5
	return Rect{
		X: float32(math.Floor(x+0.5) / scale), Y: float32(math.Floor(y+0.5) / scale),
		Width: float32(pixelWidth / scale), Height: float32(pixelHeight / scale),
	}, true
}

// drawIconDirect tessellates and paints an icon from scratch. DrawIcon replays a
// cached tessellation instead; this is what fills the cache and what the tests
// compare it against.
func (canvas *Canvas) drawIconDirect(icon Icon, bounds Rect, tint Color) {
	if tint.Alpha <= 0 || icon.ViewBox <= 0 || len(icon.Commands) == 0 && len(icon.Layers) == 0 {
		return
	}
	target, ok := canvas.iconTarget(icon, bounds)
	if !ok {
		return
	}
	canvas.paintIconShapes(icon, target, tint, nil)
}

// fitted is the size the icon takes inside bounds: as large as fits with its
// aspect ratio kept.
func (icon Icon) fitted(bounds Rect) (width, height float32) {
	scale := min(bounds.Width/icon.ViewBox, bounds.Height/icon.height())
	return icon.ViewBox * scale, icon.height() * scale
}

// paintIconShapes paints the icon's strokes or fills and then its layers into
// target. After each shape it calls shape, if given, with -1 for the icon's own and
// the layer's index otherwise.
func (canvas *Canvas) paintIconShapes(icon Icon, target Rect, tint Color, shape func(layer int)) {
	units := iconUnitsAt(icon, target)
	if len(icon.Commands) > 0 {
		strokeWidth := icon.Stroke * units.y
		if strokeWidth > 0 {
			// A stroke never renders thinner than it was drawn: rounding to whole
			// pixels turned a 1.33 point stroke into 1 point at 100% but 1.5 at 200%.
			// Half a device pixel is the finest grid that keeps the weight steady.
			strokeWidth = max(canvas.snapToHalfDeviceUp(strokeWidth), canvas.snapToDevice(1))
		}
		style := PathStyle{
			Stroke:      tint,
			StrokeWidth: strokeWidth,
			Cap:         icon.Cap,
			Join:        icon.Join,
		}
		if icon.Stroke <= 0 {
			style.Stroke = Color{}
			style.Fill = tint
		}
		canvas.DrawPath(Path{width: icon.ViewBox, height: icon.height(), commands: icon.Commands}, target, style)
		if shape != nil {
			shape(-1)
		}
	}
	for index := range icon.Layers {
		layer := &icon.Layers[index]
		fill := layer.Fill
		if layer.Gradient != nil {
			fill = layer.Gradient.From
		}
		fill.Alpha *= tint.Alpha
		start := len(canvas.triangles)
		canvas.DrawPath(Path{width: icon.ViewBox, height: icon.height(), commands: layer.Commands}, target,
			PathStyle{Fill: fill, FillRule: layer.FillRule})
		if shape != nil {
			shape(index)
		} else if layer.Gradient != nil {
			canvas.shadeGradient(start, newGradientAxis(*layer.Gradient, target, units.x, units.y), layer.Gradient, tint.Alpha)
		}
	}
}

// referenceScale is the device scale a canvas paints for until SetPixelScale says
// otherwise: a Retina display, and the scale the headless renderer defaults to.
const referenceScale = 2

// SetPixelScale tells the canvas how many device pixels one logical pixel is on the
// display it paints for (1.25 at 125%), so that what must land on whole pixels, such as the
// edges of an icon, does at that scale, not only at 2x; an icon's stroke snaps up to half
// a device pixel at that scale. Zero is unspecified and restores the reference scale. It
// survives Reset.
func (canvas *Canvas) SetPixelScale(scale float32) {
	if scale > 0 {
		canvas.pixelScale = scale
	} else {
		canvas.pixelScale = 0
	}
}

// PixelScale is the device pixels per logical pixel the canvas paints for.
func (canvas *Canvas) PixelScale() float32 {
	if canvas.pixelScale > 0 {
		return canvas.pixelScale
	}
	return referenceScale
}

// snapToDevice rounds a logical-pixel length or coordinate to the nearest whole device pixel.
func (canvas *Canvas) snapToDevice(value float32) float32 {
	scale := canvas.PixelScale()
	return float32(math.Floor(float64(value)*float64(scale)+0.5)) / scale
}

// snapToHalfDeviceUp rounds a logical-pixel length up to the nearest half device pixel:
// fine enough for a stroke to keep the weight it was drawn with, and never below it, so a
// 1.33 point icon stroke is 1.5 pixels at 100% and 3 at 200% instead of 1 and 3.
func (canvas *Canvas) snapToHalfDeviceUp(value float32) float32 {
	scale := canvas.PixelScale()
	// A float32 stroke puts an exact half a hair above the boundary; without the
	// tolerance a 1.33 point stroke at 150% would gain a whole half pixel.
	halves := math.Ceil(float64(value)*float64(scale)*2 - 1e-4)
	return float32(halves) / (2 * scale)
}
