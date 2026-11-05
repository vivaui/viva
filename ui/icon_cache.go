package ui

import (
	"math"
	"slices"
)

// Painting an icon means flattening its curves and triangulating its strokes and
// fills, and an interface paints the same few icons at the same few sizes every
// frame. The tessellation depends on the icon, the size and whether the stroke is
// opaque, and on nothing else: where it is and what color it is are applied when it
// is replayed. So the canvas tessellates an icon once, at the origin, and every
// later paint is a translation and a color.
//
// The result is the same triangles the direct path emits (see the test), so no
// renderer changes; what is saved is the flattening, the stroke geometry and the
// ear clipping, which dominated a repaint of the Viva SSH workbench.

// iconCacheLimit bounds the cache: when it is full it starts over, so an
// application that paints many sizes cannot grow it without end.
const iconCacheLimit = 512

type iconKey struct {
	commands *PathCommand
	count    int // commands[0:count] is the glyph: a contour shares the slice's start
	layers   *IconLayer
	layerN   int
	viewBox  float32
	viewHigh float32
	stroke   float32
	fit      float32
	scale    float32 // the pixel scale the strokes were rounded for
	cap      StrokeCap
	join     StrokeJoin
	opaque   bool
}

// iconTriangle is one tessellated triangle, relative to the icon's origin. layer is
// -1 for the icon's own strokes and fills, and the index of the layer otherwise.
type iconTriangle struct {
	points [3]Point
	edges  uint8
	layer  int16
	// t is the gradient position at each corner of a triangle of a gradient layer.
	t [3]float32
}

type iconCache struct {
	entries map[iconKey][]iconTriangle
}

// iconPaintKey identifies a fully materialized icon: the tessellation, where it
// is, how it is tinted, and the canvas state its triangles carry. A repeated
// paint can then be one slice copy instead of one Triangle fill per triangle.
type iconPaintKey struct {
	icon        iconKey
	origin      Point
	tint        Color
	layer       uint16
	clip        Rect
	hasClip     bool
	rectsBefore uint32
}

type iconPaintCache struct {
	entries  map[iconPaintKey][]Triangle
	previous map[iconPaintKey][]Triangle
}

const iconPaintCacheLimit = 512

func (canvas *Canvas) iconPaints() *iconPaintCache {
	if canvas.iconPaintCache == nil {
		canvas.iconPaintCache = new(iconPaintCache)
	}
	return canvas.iconPaintCache
}

func (cache *iconPaintCache) get(key iconPaintKey) ([]Triangle, bool) {
	if triangles, ok := cache.entries[key]; ok {
		return triangles, true
	}
	triangles, ok := cache.previous[key]
	if !ok {
		return nil, false
	}
	delete(cache.previous, key)
	cache.put(key, triangles)
	return triangles, true
}

func (cache *iconPaintCache) put(key iconPaintKey, triangles []Triangle) {
	if cache.entries == nil {
		cache.entries = make(map[iconPaintKey][]Triangle, 64)
	}
	if len(cache.entries) >= iconPaintCacheLimit {
		cache.previous = cache.entries
		cache.entries = make(map[iconPaintKey][]Triangle, 64)
	}
	cache.entries[key] = triangles
}

func (canvas *Canvas) icons() *iconCache {
	if canvas.iconCache == nil {
		canvas.iconCache = new(iconCache)
	}
	return canvas.iconCache
}

func newIconKey(icon Icon, fit, scale float32, tint Color) iconKey {
	// A contour is a slice of its glyph's commands, so the start pointer alone does
	// not identify an icon: the head of a glyph and the glyph itself begin at the
	// same command when the contour is the first one. The length is part of the key
	// for that reason.
	key := iconKey{viewBox: icon.ViewBox, viewHigh: icon.height(), stroke: icon.Stroke, fit: fit, scale: scale, cap: icon.Cap, join: icon.Join, opaque: tint.Alpha >= 0.999}
	if len(icon.Commands) > 0 {
		key.commands = &icon.Commands[0]
		key.count = len(icon.Commands)
	}
	if len(icon.Layers) > 0 {
		key.layers = &icon.Layers[0]
		key.layerN = len(icon.Layers)
	}
	return key
}

func (canvas *Canvas) drawIconCached(icon Icon, fit float32, origin Point, tint Color) {
	key := newIconKey(icon, fit, canvas.PixelScale(), tint)
	// The layer, the clip and the rectangle count are the same for every triangle
	// of the icon: read them once.
	layer := canvas.currentLayer()
	clip, hasClip := canvas.clipSnapshot()
	rectsBefore := uint32(len(canvas.rectangles))
	paintKey := iconPaintKey{icon: key, origin: origin, tint: tint, layer: layer, clip: clip, hasClip: hasClip, rectsBefore: rectsBefore}
	paints := canvas.iconPaints()
	if cached, ok := paints.get(paintKey); ok {
		canvas.triangles = append(canvas.triangles, cached...)
		return
	}
	cache := canvas.icons()
	triangles, ok := cache.entries[key]
	if !ok {
		triangles = canvas.tessellateIcon(icon, fit, tint)
		if cache.entries == nil || len(cache.entries) >= iconCacheLimit {
			cache.entries = make(map[iconKey][]iconTriangle, 64)
		}
		cache.entries[key] = triangles
	}
	start := len(canvas.triangles)
	canvas.triangles = slices.Grow(canvas.triangles, len(triangles))
	for index := range triangles {
		cached := &triangles[index]
		triangle := Triangle{
			Points: [3]Point{
				{X: cached.points[0].X + origin.X, Y: cached.points[0].Y + origin.Y},
				{X: cached.points[1].X + origin.X, Y: cached.points[1].Y + origin.Y},
				{X: cached.points[2].X + origin.X, Y: cached.points[2].Y + origin.Y},
			},
			Color: tint, Layer: layer, EdgeMask: cached.edges,
			ClipOf: clip, HasClip: hasClip, RectsBefore: rectsBefore,
		}
		if cached.layer >= 0 {
			source := &icon.Layers[cached.layer]
			if source.Gradient != nil {
				colorize(&triangle, source.Gradient, cached.t, tint.Alpha)
			} else {
				triangle.Color = fade(source.Fill, tint.Alpha)
			}
			if !triangle.visible() {
				continue
			}
		}
		canvas.triangles = append(canvas.triangles, triangle)
	}
	paints.put(paintKey, append([]Triangle(nil), canvas.triangles[start:]...))
}

// iconTriangles returns the icon's tessellation, tessellating it the first time.
func (canvas *Canvas) iconTriangles(icon Icon, fit float32, tint Color) []iconTriangle {
	return canvas.trianglesFor(newIconKey(icon, fit, canvas.PixelScale(), tint), icon, fit, tint)
}

// trianglesFor is iconTriangles for a caller that has already built the key.
func (canvas *Canvas) trianglesFor(key iconKey, icon Icon, fit float32, tint Color) []iconTriangle {
	cache := canvas.icons()
	if triangles, ok := cache.entries[key]; ok {
		return triangles
	}
	triangles := canvas.tessellateIcon(icon, fit, tint)
	if cache.entries == nil || len(cache.entries) >= iconCacheLimit {
		cache.entries = make(map[iconKey][]iconTriangle, 64)
	}
	cache.entries[key] = triangles
	return triangles
}

// drawIconPlacedClipped paints the icon's tessellation through place, which is
// how an icon is drawn while it is in motion. Nothing is cached for it: the
// points move on every frame, so the paint cache would never hit. The optional
// icon-local clip cuts each triangle before the canvas sees it, so the paint
// stays one ordinary mesh and needs no renderer mask.
func (canvas *Canvas) drawIconPlacedClipped(icon Icon, fit float32, tint Color, place iconPlacement, clip *iconClipShape) {
	triangles := canvas.iconTriangles(icon, fit, tint)
	canvas.drawIconTrianglesPlaced(triangles, icon, tint, place, clip)
}

// drawIconTrianglesPlaced applies place and clip to an icon tessellation.
func (canvas *Canvas) drawIconTrianglesPlaced(triangles []iconTriangle, icon Icon, tint Color, place iconPlacement, clip *iconClipShape) {
	state := canvas.iconPaintState()
	canvas.triangles = slices.Grow(canvas.triangles, len(triangles))
	if clip == nil {
		for index := range triangles {
			cached := &triangles[index]
			triangle := Triangle{
				Points: [3]Point{
					place.apply(cached.points[0]),
					place.apply(cached.points[1]),
					place.apply(cached.points[2]),
				},
				Color: tint, Layer: state.layer, EdgeMask: cached.edges,
				ClipOf: state.clip, HasClip: state.hasClip, RectsBefore: state.rectsBefore,
			}
			if cached.layer >= 0 && !shadeIconTriangle(icon, tint, cached, &triangle) {
				continue
			}
			canvas.triangles = append(canvas.triangles, triangle)
		}
		return
	}
	for index := range triangles {
		cached := &triangles[index]
		source := [3]Point{
			place.apply(cached.points[0]),
			place.apply(cached.points[1]),
			place.apply(cached.points[2]),
		}
		emit := func(points [3]Point, edges uint8, weights [3][3]float32) {
			triangle := Triangle{
				Points: points, Color: tint, Layer: state.layer, EdgeMask: edges,
				ClipOf: state.clip, HasClip: state.hasClip, RectsBefore: state.rectsBefore,
			}
			if cached.layer >= 0 && !shadeIconTriangleAt(icon, tint, cached, weights, &triangle) {
				return
			}
			canvas.triangles = append(canvas.triangles, triangle)
		}
		clip.eachTriangle(source, cached.edges, emit)
	}
}

// iconPaintState is what every triangle of one icon shares: the layer it lands
// in, the clip it was drawn under and how many rectangles came before it.
type iconPaintState struct {
	layer       uint16
	clip        Rect
	hasClip     bool
	rectsBefore uint32
}

func (canvas *Canvas) iconPaintState() iconPaintState {
	state := iconPaintState{layer: canvas.currentLayer(), rectsBefore: uint32(len(canvas.rectangles))}
	state.clip, state.hasClip = canvas.clipSnapshot()
	return state
}

// shadeIconTriangle applies the fill of the icon layer the triangle came from and
// reports whether the triangle is visible. Only a multi-coloured icon has layers,
// so the caller checks for one before it pays for the call.
func shadeIconTriangle(icon Icon, tint Color, cached *iconTriangle, triangle *Triangle) bool {
	var weights [3][3]float32
	for corner := range weights {
		weights[corner][corner] = 1
	}
	return shadeIconTriangleAt(icon, tint, cached, weights, triangle)
}

// shadeIconTriangleAt is shadeIconTriangle for a triangle a clip cut out of
// cached: weights are its corners' barycentric coordinates in cached.
func shadeIconTriangleAt(icon Icon, tint Color, cached *iconTriangle, weights [3][3]float32, triangle *Triangle) bool {
	source := &icon.Layers[cached.layer]
	if source.Gradient != nil {
		var t [3]float32
		for corner := range t {
			for sourceCorner := range cached.t {
				t[corner] += weights[corner][sourceCorner] * cached.t[sourceCorner]
			}
		}
		colorize(triangle, source.Gradient, t, tint.Alpha)
	} else {
		triangle.Color = fade(source.Fill, tint.Alpha)
	}
	return triangle.visible()
}

// iconPlacement maps a tessellated icon, which is laid out from (0, 0), onto the
// canvas. It is the six numbers of an affine map plus the origin the icon is
// painted at, which keeps the mapping composable: a part of a glyph is placed by
// its own matrix and then by the glyph's.
type iconPlacement struct {
	origin           Point
	a, b, c, d, e, f float32
}

// stillPlacement places the tessellation unchanged at origin, the case the paint
// cache can replay.
func stillPlacement(origin Point) iconPlacement {
	return iconPlacement{origin: origin, a: 1, d: 1}
}

// iconPlacementAt is the placement that scales by scale, turns by rotate radians
// and then moves by offset, all about center — the glyph's center for the icon,
// and the bounds' center for a part of it.
func iconPlacementAt(center, offset Point, rotate, scale float32) iconPlacement {
	return iconPlacementAtSize(center, offset, rotate, scale, scale)
}

// iconPlacementAtSize is iconPlacementAt with independent vertical scale.
func iconPlacementAtSize(center, offset Point, rotate, scaleX, scaleY float32) iconPlacement {
	sine, cosine := math.Sincos(float64(rotate))
	s, c := float32(sine), float32(cosine)
	a, b, c1, d := scaleX*c, scaleX*s, -scaleY*s, scaleY*c
	return iconPlacement{
		a: a, b: b, c: c1, d: d,
		e: center.X - a*center.X - c1*center.Y + offset.X,
		f: center.Y - b*center.X - d*center.Y + offset.Y,
	}
}

// after composes the placements: this one first, then outer, as outer ∘ this.
func (place iconPlacement) after(outer iconPlacement) iconPlacement {
	x := outer.a*place.origin.X + outer.c*place.origin.Y + outer.e
	y := outer.b*place.origin.X + outer.d*place.origin.Y + outer.f
	return iconPlacement{
		origin: Point{X: outer.origin.X + x, Y: outer.origin.Y + y},
		a:      outer.a*place.a + outer.c*place.b,
		b:      outer.b*place.a + outer.d*place.b,
		c:      outer.a*place.c + outer.c*place.d,
		d:      outer.b*place.c + outer.d*place.d,
		e:      outer.a*place.e + outer.c*place.f + outer.e,
		f:      outer.b*place.e + outer.d*place.f + outer.f,
	}
}

// turns reports whether the placement does more than translate the icon.
func (place iconPlacement) turns() bool {
	return place.a != 1 || place.b != 0 || place.c != 0 || place.d != 1
}

// still reports whether the placement only moves the icon, so the paint cache
// can replay it as it would any other origin.
func (place iconPlacement) still() bool {
	return !place.turns() && place.e == 0 && place.f == 0
}

// apply places one vertex of the tessellation.
func (place iconPlacement) apply(point Point) Point {
	return Point{
		X: place.origin.X + place.a*point.X + place.c*point.Y + place.e,
		Y: place.origin.Y + place.b*point.X + place.d*point.Y + place.f,
	}
}

// drawIconParts paints the glyph contour by contour, each through the glyph's
// placement composed with its own part. A contour nobody moves is drawn with the
// glyph's placement, so the only cost of an unmoving part is that the glyph is
// painted as several paths rather than one.
//
// Layers are not contours: a multi-color icon paints them once through the
// glyph's placement, after the contours, in the order the whole glyph paints
// them. A part that moves changes the stroked glyph only, never the layer colors.
func (canvas *Canvas) drawIconParts(icon Icon, target Rect, tint Color, place iconPlacement, parts []IconPart) {
	canvas.drawIconPartsClipped(icon, target, tint, place, parts, nil)
}

// drawIconPartsClipped is drawIconParts with an optional icon-local clip.
func (canvas *Canvas) drawIconPartsClipped(icon Icon, target Rect, tint Color, place iconPlacement, parts []IconPart, clip *iconClipShape) {
	units := iconUnitsAt(icon, target)
	contours := icon.appendContours(canvas.iconContours[:0])
	canvas.iconContours = contours
	centers := canvas.iconPartCenters[:0]
	for index := range contours {
		var center Point
		if part, ok := iconPartAt(parts, index); ok && part.moves() {
			// A part that names an anchor turns about that point of the glyph, so
			// its own bounds do not have to be measured.
			if part.Anchor != nil {
				center = units.point(*part.Anchor)
			} else {
				center = iconCenter(canvas.iconTriangles(contours[index], target.Width, tint))
			}
		}
		centers = append(centers, center)
	}
	canvas.iconPartCenters = centers
	for index := range contours {
		own := place
		partTint := tint
		if part, ok := iconPartAt(parts, index); ok && part.moves() {
			center := centers[index]
			offset := Point{
				X: part.Translate.X*units.x + part.Offset.X,
				Y: part.Translate.Y*units.y + part.Offset.Y,
			}
			local := iconPlacementAtSize(center, offset, part.Rotate, iconPartScale(part), iconPartScale(part))
			if part.stretches() {
				local = stretchPlacementAt(center, iconPartScaleX(part), iconPartScaleY(part)).after(local)
			}
			own = local.after(place)
			partTint.Alpha *= 1 - min(max(part.Fade, 0), 1)
		}
		if partTint.Alpha <= 0 {
			continue
		}
		contour := contours[index]
		fresh := false
		// A filled glyph has no stroke to write, so its trim is the whole
		// contour; applying the polyline would turn a fill into an outline.
		if part, ok := iconPartAt(parts, index); ok && part.trims() && contour.Stroke > 0 {
			from, to := part.Trim.rangeValues()
			contour = canvas.trimIconContour(contour, from, to)
			// The contiguous trim buffers change in place from frame to
			// frame, so their address cannot key the icon cache.
			fresh = true
		}
		if fresh {
			triangles := canvas.tessellateIconScratch(contour, target.Width, partTint)
			canvas.drawIconTrianglesPlaced(triangles, contour, partTint, own, clip)
			continue
		}
		canvas.drawIconPlacedClipped(contour, target.Width, partTint, own, clip)
	}
	if len(icon.Layers) > 0 {
		layers := icon
		layers.Commands = nil
		canvas.drawIconPlacedClipped(layers, target.Width, tint, place, clip)
	}
}

// stretchPlacementAt scales both axes about center, the way a page turning over
// its spine or a meridian crossing a globe projects onto the plane.
func stretchPlacementAt(center Point, stretchX, stretchY float32) iconPlacement {
	return iconPlacement{
		a: stretchX, d: stretchY,
		e: center.X - stretchX*center.X,
		f: center.Y - stretchY*center.Y,
	}
}

// iconPartAt returns the part that moves the given contour.
func iconPartAt(parts []IconPart, contour int) (IconPart, bool) {
	for _, part := range parts {
		if part.Contour == contour {
			return part, true
		}
	}
	return IconPart{}, false
}

// iconPartScale is a part's scale with zero read as 1.
func iconPartScale(part IconPart) float32 {
	if part.Scale == 0 {
		return 1
	}
	return part.Scale
}

// iconPartScaleX is a part's horizontal stretch with zero read as 1.
func iconPartScaleX(part IconPart) float32 {
	if part.ScaleX == 0 {
		return 1
	}
	return part.ScaleX
}

// iconPartScaleY is a part's vertical stretch with zero read as 1.
func iconPartScaleY(part IconPart) float32 {
	if part.ScaleY == 0 {
		return 1
	}
	return part.ScaleY
}

// iconCenter is the middle of the bounds of a tessellation: the point a part of
// the glyph turns and grows about.
func iconCenter(triangles []iconTriangle) Point {
	if len(triangles) == 0 {
		return Point{}
	}
	minimum := triangles[0].points[0]
	maximum := minimum
	for _, triangle := range triangles {
		for _, point := range triangle.points {
			minimum.X = min(minimum.X, point.X)
			minimum.Y = min(minimum.Y, point.Y)
			maximum.X = max(maximum.X, point.X)
			maximum.Y = max(maximum.Y, point.Y)
		}
	}
	return Point{X: (minimum.X + maximum.X) * 0.5, Y: (minimum.Y + maximum.Y) * 0.5}
}

// tessellateIcon paints the icon at the origin with the direct path and takes the
// triangles it appended back out of the canvas, recording which layer made each.
// Colors are placeholders of the right opacity: geometry does not depend on them.
func (canvas *Canvas) tessellateIcon(icon Icon, fit float32, tint Color) []iconTriangle {
	return canvas.tessellateIconInto(icon, fit, tint, nil)
}

// tessellateIconScratch is tessellateIcon into the canvas's own slice. A
// trimmed contour is new geometry every frame but never enters the cache, so
// its tessellation must not allocate once the slice has grown.
func (canvas *Canvas) tessellateIconScratch(icon Icon, fit float32, tint Color) []iconTriangle {
	triangles := canvas.tessellateIconInto(icon, fit, tint, canvas.iconTrimTriangles[:0])
	canvas.iconTrimTriangles = triangles
	return triangles
}

// tessellateIconInto is tessellateIcon appending to out.
func (canvas *Canvas) tessellateIconInto(icon Icon, fit float32, tint Color, out []iconTriangle) []iconTriangle {
	out = out[:0]
	probe := Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	if tint.Alpha < 0.999 {
		probe.Alpha = 0.5
	}
	start, rectangles := len(canvas.triangles), len(canvas.rectangles)
	canvas.paintIconShapes(icon, Rect{Width: fit, Height: fit * icon.height() / icon.ViewBox}, probe, func(layer int) {
		if layer >= 0 && icon.Layers[layer].Gradient != nil {
			axis := newGradientAxis(*icon.Layers[layer].Gradient, Rect{Width: fit, Height: fit * icon.height() / icon.ViewBox}, fit/icon.ViewBox, fit/icon.ViewBox)
			for _, triangle := range canvas.triangles[start:] {
				axis.each(triangle.Points, triangle.EdgeMask, func(points [3]Point, edges uint8) {
					out = append(out, iconTriangle{points: points, edges: edges, layer: int16(layer), t: axis.vertexT(points)})
				})
			}
		} else {
			for _, triangle := range canvas.triangles[start:] {
				out = append(out, iconTriangle{points: triangle.Points, edges: triangle.EdgeMask, layer: int16(layer)})
			}
		}
		canvas.triangles = canvas.triangles[:start]
	})
	canvas.rectangles = canvas.rectangles[:rectangles]
	return out
}
