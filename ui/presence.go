package ui

import "math"

// Presence describes how one overlay enters and leaves: a dialog fading in and
// out while it grows from 95% of its size, a menu sliding a few pixels from the
// edge it opens on, a sheet sliding in from the side. It is the shape of the
// animation, not the state of it; the canvas keeps the state (see
// Canvas.BeginPresence).
type Presence struct {
	// Motion is the duration and curve of entering and of leaving. A zero
	// duration disables the transition.
	Motion Motion
	// Fade multiplies the opacity of everything in the overlay by its progress.
	Fade bool
	// Scale is the size of the overlay at the start of entering and the end of
	// leaving, as a fraction of its own: 0.95 is Tailwind's zoom-95. Zero and one
	// leave the size alone.
	Scale float32
	// SlideIn is where the overlay sits relative to its place when it starts to
	// enter, and SlideOut where it ends up when it has left, in logical pixels.
	// A menu slides in a few pixels from its trigger and just fades out; a sheet
	// slides both ways. An overlay that is turned around halfway jumps by the
	// difference of the two at that moment, which is why they are small.
	SlideIn, SlideOut Point
}

func (presence Presence) moves() bool {
	return presence.Fade || (presence.Scale > 0 && presence.Scale != 1) || presence.SlideIn != Point{} || presence.SlideOut != Point{}
}

// presenceOpen is a group begun and not yet ended in the current paint pass: the
// entry it animates and where its primitives start in the canvas's lists.
type presenceOpen struct {
	// entry indexes motionState.presences; -1 for a group that does not animate.
	entry                                int
	rectangles, triangles, images, texts int
	runs                                 int
	shadows                              int
}

// presenceSnapshot is what an overlay painted in its last frame, untouched by
// its transition: what a leaving overlay is drawn from after the application has
// stopped painting it. Triangle.RectsBefore and Text.RunFirst are relative to
// the start of the snapshot, and so are Shadow.RectsBefore and TrianglesBefore.
type presenceSnapshot struct {
	rectangles []Rectangle
	shadows    []Shadow
	triangles  []Triangle
	images     []Image
	texts      []Text
	runs       []TextRun
}

// presenceEntry is one overlay that is open or leaving. Its progress is a
// motionSlot: 0 is gone, 1 is fully shown.
type presenceEntry struct {
	motionSlot
	presence Presence
	pivot    Point
	snapshot presenceSnapshot
	// captured says EndPresence has copied a frame. A group never ended has
	// nothing to leave with, and is dropped when the application stops painting it.
	captured bool
}

// BeginPresence starts the group of primitives that make up one overlay (or one
// part of it, such as its backdrop) and gives it an entrance and an exit. id
// names the overlay and slot distinguishes the parts of one overlay that move
// differently; pivot is the point the overlay grows from. Everything the canvas
// paints until the matching EndPresence belongs to the group.
//
// While the application keeps painting the group, it fades and moves in from its
// first frame, and a frame later is simply there. When the application stops
// painting it, the canvas keeps drawing the last frame it painted, fading and
// moving it out, until it is gone: the application needs no state for a
// leaving overlay, and the leaving overlay is only a picture. It has no hit
// regions, no focus scope, no Escape target, so it never takes a press or the
// keyboard from what is under it. If the application paints the group again
// while it leaves, it turns around from where it is.
//
// This is the one place the canvas keeps something its application no longer
// paints ("state follows what is painted"). It
// holds a copy of the group's last frame while the group is open, and drops it
// when the exit ends.
//
// A group does not animate, and BeginPresence costs a stack push, with ID zero
// (a static preview), without a clock, outside BeginFrame and EndFrame, under
// reduced motion, and for a presence without a duration or without effect: the
// overlay is then just painted and just gone. Images are moved but not faded,
// for a Bitmap has no opacity.
func (canvas *Canvas) BeginPresence(id ID, slot uint8, presence Presence, pivot Point) {
	state := &canvas.motion
	open := presenceOpen{
		entry:      -1,
		rectangles: len(canvas.rectangles), triangles: len(canvas.triangles), images: len(canvas.images),
		texts: len(canvas.texts), runs: len(canvas.runs), shadows: len(canvas.shadows),
	}
	if state.active && !state.reduce && !id.IsZero() && presence.Motion.Duration > 0 && presence.moves() {
		now := state.frameTime()
		index := state.presenceFor(id, slot)
		if index < 0 {
			// An overlay that opens enters, unlike a control that appears on a
			// page: opening is the event the user asked for.
			state.presences = append(state.presences, presenceEntry{motionSlot: motionSlot{
				id: id, slot: slot, from: 0, to: 0, start: now,
			}})
			index = len(state.presences) - 1
		}
		entry := &state.presences[index]
		entry.seen = state.frame
		entry.presence, entry.pivot = presence, pivot
		if entry.to != 1 {
			entry.from = entry.value(now)
			entry.to = 1
			entry.start = now
			entry.duration = presence.Motion.Duration
			entry.easing = presence.Motion.Easing
		}
		open.entry = index
	}
	state.open = append(state.open, open)
}

// EndPresence closes the group opened by BeginPresence. A group that is still
// entering is faded and moved here, in place, after its last primitive.
func (canvas *Canvas) EndPresence() {
	state := &canvas.motion
	if len(state.open) == 0 {
		return
	}
	open := state.open[len(state.open)-1]
	state.open = state.open[:len(state.open)-1]
	if open.entry < 0 || open.entry >= len(state.presences) {
		return
	}
	entry := &state.presences[open.entry]
	entry.snapshot.capture(canvas, open)
	entry.captured = true
	now := state.frameTime()
	progress := entry.value(now)
	if !entry.settled(now) {
		state.animating = true
	}
	if progress < 1 {
		entry.presence.apply(canvas, open, entry.pivot, progress, false)
	}
}

// presenceFor finds the entry of an overlay part.
func (state *motionState) presenceFor(id ID, slot uint8) int {
	for index := range state.presences {
		if state.presences[index].id == id && state.presences[index].slot == slot {
			return index
		}
	}
	return -1
}

// leave closes the paint pass for overlays: an entry the pass did not paint
// starts to leave, and one that is leaving is drawn from its snapshot at its
// progress. An entry whose exit has finished is released.
func (state *motionState) leave(canvas *Canvas) {
	state.open = state.open[:0]
	if len(state.presences) == 0 {
		return
	}
	if state.clock == nil || state.reduce {
		clear(state.presences)
		state.presences = state.presences[:0]
		return
	}
	now := state.frameTime()
	kept := state.presences[:0]
	for _, entry := range state.presences {
		if entry.seen != state.frame {
			if !entry.captured {
				continue
			}
			if entry.to != 0 {
				entry.from = entry.value(now)
				entry.to = 0
				entry.start = now
				entry.duration = entry.presence.Motion.Duration
				entry.easing = entry.presence.Motion.Easing
			}
			if entry.settled(now) {
				continue
			}
			state.animating = true
			entry.snapshot.replay(canvas, entry.presence, entry.pivot, entry.value(now))
		}
		kept = append(kept, entry)
	}
	clear(state.presences[len(kept):])
	state.presences = kept
}

// capture copies what the group painted since open.
func (snapshot *presenceSnapshot) capture(canvas *Canvas, open presenceOpen) {
	snapshot.rectangles = append(snapshot.rectangles[:0], canvas.rectangles[open.rectangles:]...)
	snapshot.shadows = append(snapshot.shadows[:0], canvas.shadows[open.shadows:]...)
	for index := range snapshot.shadows {
		snapshot.shadows[index].RectsBefore -= uint32(open.rectangles)
		snapshot.shadows[index].TrianglesBefore -= uint32(open.triangles)
	}
	snapshot.triangles = append(snapshot.triangles[:0], canvas.triangles[open.triangles:]...)
	for index := range snapshot.triangles {
		snapshot.triangles[index].RectsBefore -= uint32(open.rectangles)
	}
	snapshot.images = append(snapshot.images[:0], canvas.images[open.images:]...)
	snapshot.texts = append(snapshot.texts[:0], canvas.texts[open.texts:]...)
	for index := range snapshot.texts {
		if snapshot.texts[index].RunCount > 0 {
			snapshot.texts[index].RunFirst -= int32(open.runs)
		}
	}
	snapshot.runs = append(snapshot.runs[:0], canvas.runs[open.runs:]...)
}

// replay appends the snapshot to the canvas, moved and faded to progress.
func (snapshot *presenceSnapshot) replay(canvas *Canvas, presence Presence, pivot Point, progress float32) {
	open := presenceOpen{
		rectangles: len(canvas.rectangles), triangles: len(canvas.triangles), images: len(canvas.images),
		texts: len(canvas.texts), runs: len(canvas.runs), shadows: len(canvas.shadows),
	}
	canvas.rectangles = append(canvas.rectangles, snapshot.rectangles...)
	canvas.shadows = append(canvas.shadows, snapshot.shadows...)
	for index := open.shadows; index < len(canvas.shadows); index++ {
		canvas.shadows[index].RectsBefore += uint32(open.rectangles)
		canvas.shadows[index].TrianglesBefore += uint32(open.triangles)
	}
	canvas.triangles = append(canvas.triangles, snapshot.triangles...)
	for index := open.triangles; index < len(canvas.triangles); index++ {
		canvas.triangles[index].RectsBefore += uint32(open.rectangles)
	}
	canvas.images = append(canvas.images, snapshot.images...)
	canvas.texts = append(canvas.texts, snapshot.texts...)
	for index := open.texts; index < len(canvas.texts); index++ {
		if canvas.texts[index].RunCount > 0 {
			canvas.texts[index].RunFirst += int32(open.runs)
		}
	}
	canvas.runs = append(canvas.runs, snapshot.runs...)
	presence.apply(canvas, open, pivot, progress, true)
}

// presenceTransform is the placement and opacity of an overlay at one moment.
type presenceTransform struct {
	pivot, offset Point
	scale, alpha  float32
}

func (transform presenceTransform) point(point Point) Point {
	return Point{
		X: transform.pivot.X + (point.X-transform.pivot.X)*transform.scale + transform.offset.X,
		Y: transform.pivot.Y + (point.Y-transform.pivot.Y)*transform.scale + transform.offset.Y,
	}
}

func (transform presenceTransform) rect(rect Rect) Rect {
	origin := transform.point(Point{X: rect.X, Y: rect.Y})
	return Rect{X: origin.X, Y: origin.Y, Width: rect.Width * transform.scale, Height: rect.Height * transform.scale}
}

func (transform presenceTransform) color(color Color) Color {
	color.Alpha *= transform.alpha
	return color
}

// vectorizeBorderBackings turns the backing of every RoundedBorder painted
// since first into a vector ring. A fade multiplies the opacity of each shape,
// so a backing left under the fill would show through it for as long as the
// overlay is not fully there. A ring keeps both rounded edges without painting
// under the fill; at rest the cheaper pair of rounded rectangles is unchanged.
func vectorizeBorderBackings(canvas *Canvas, first int) {
	last := len(canvas.rectangles) - 1
	for index := first; index < last; index++ {
		backing, fill := canvas.rectangles[index], canvas.rectangles[index+1]
		width := fill.Bounds.X - backing.Bounds.X
		if width <= 0 || width > hairline || backing.Layer != fill.Layer || backing.HasClip != fill.HasClip ||
			fill.Bounds.Y-backing.Bounds.Y != width ||
			backing.Bounds.Width-fill.Bounds.Width != 2*width || backing.Bounds.Height-fill.Bounds.Height != 2*width {
			continue
		}
		appendRoundedBorderRing(canvas, backing, fill, uint32(index+2))
		canvas.rectangles[index].Color = Color{}
		index++
	}
}

// ringKey names one border ring's geometry: only the two rounded rectangles
// decide its triangles, not the color the fade multiplies into them.
type ringKey struct {
	backing, fill             Rect
	backingRadius, fillRadius float32
}

// ringCacheEntry keeps the triangles of one border ring. A fade vectorizes the
// same geometry on every frame, so the ear clipping is done once instead.
type ringCacheEntry struct {
	key       ringKey
	triangles []Triangle
}

// ringCacheCap bounds the retained rings; an overlay uses one or two, and a
// page that cycles through many distinct borders must not keep them all.
const ringCacheCap = 8

func appendRoundedBorderRing(canvas *Canvas, backing, fill Rectangle, rectsBefore uint32) {
	key := ringKey{backing: backing.Bounds, fill: fill.Bounds, backingRadius: backing.Radius, fillRadius: fill.Radius}
	var cached []Triangle
	for index := range canvas.rings {
		if canvas.rings[index].key == key {
			cached = canvas.rings[index].triangles
			break
		}
	}

	first := len(canvas.triangles)
	if cached != nil {
		for _, triangle := range cached {
			canvas.triangles = append(canvas.triangles, Triangle{
				Points:  triangle.Points,
				Color:   backing.Color,
				Layer:   backing.Layer,
				ClipOf:  backing.ClipOf,
				HasClip: backing.HasClip,
			})
		}
	} else {
		canvas.ringPoints = appendRoundedRectContour(canvas.ringPoints[:0], backing.Bounds, backing.Radius)
		canvas.ringEnds = append(canvas.ringEnds[:0], int32(len(canvas.ringPoints)))
		canvas.ringPoints = appendRoundedRectContour(canvas.ringPoints, fill.Bounds, fill.Radius)
		canvas.ringEnds = append(canvas.ringEnds, int32(len(canvas.ringPoints)))
		canvas.earClipRings(backing.Color)
	}
	last := len(canvas.triangles)
	for index := first; index < last; index++ {
		triangle := &canvas.triangles[index]
		triangle.Layer = backing.Layer
		triangle.ClipOf, triangle.HasClip = backing.ClipOf, backing.HasClip
		triangle.RectsBefore = rectsBefore
	}
	if cached == nil && last-first > 0 && last-first <= 4096 {
		entry := ringCacheEntry{key: key, triangles: make([]Triangle, last-first)}
		for index := range entry.triangles {
			triangle := canvas.triangles[first+index]
			entry.triangles[index] = Triangle{Points: triangle.Points}
		}
		if len(canvas.rings) >= ringCacheCap {
			copy(canvas.rings, canvas.rings[1:])
			canvas.rings = canvas.rings[:len(canvas.rings)-1]
		}
		canvas.rings = append(canvas.rings, entry)
	}
	if first == last {
		return
	}

	at := 0
	for at < first && canvas.triangles[at].RectsBefore < rectsBefore {
		at++
	}
	reverseTriangles(canvas.triangles[at:first])
	reverseTriangles(canvas.triangles[first:last])
	reverseTriangles(canvas.triangles[at:last])
}

func appendRoundedRectContour(points []Point, bounds Rect, radius float32) []Point {
	radius = min(max(radius, 0), bounds.Width/2, bounds.Height/2)
	left, top := bounds.X, bounds.Y
	right, bottom := left+bounds.Width, top+bounds.Height
	if radius == 0 {
		return append(points,
			Point{X: left, Y: top}, Point{X: right, Y: top},
			Point{X: right, Y: bottom}, Point{X: left, Y: bottom})
	}

	points = append(points, Point{X: left + radius, Y: top}, Point{X: right - radius, Y: top})
	points = appendArcSweep(points, Point{X: right - radius, Y: top + radius}, float64(radius), -math.Pi/2, 0, math.NaN())
	points = append(points, Point{X: right, Y: top + radius}, Point{X: right, Y: bottom - radius})
	points = appendArcSweep(points, Point{X: right - radius, Y: bottom - radius}, float64(radius), 0, math.Pi/2, math.NaN())
	points = append(points, Point{X: right - radius, Y: bottom}, Point{X: left + radius, Y: bottom})
	points = appendArcSweep(points, Point{X: left + radius, Y: bottom - radius}, float64(radius), math.Pi/2, math.Pi, math.NaN())
	points = append(points, Point{X: left, Y: bottom - radius}, Point{X: left, Y: top + radius})
	points = appendArcSweep(points, Point{X: left + radius, Y: top + radius}, float64(radius), math.Pi, 3*math.Pi/2, math.NaN())
	return append(points, Point{X: left + radius, Y: top})
}

func reverseTriangles(triangles []Triangle) {
	for left, right := 0, len(triangles)-1; left < right; left, right = left+1, right-1 {
		triangles[left], triangles[right] = triangles[right], triangles[left]
	}
}

// apply moves and fades the primitives painted since open, which are the group's.
// The size of text is not scaled: a glyph's size is a texture, and a few percent
// is not worth one per frame, so a label moves with the overlay but keeps its size.
func (presence Presence) apply(canvas *Canvas, open presenceOpen, pivot Point, progress float32, leaving bool) {
	transform := presenceTransform{pivot: pivot, scale: 1, alpha: 1}
	if presence.Scale > 0 && presence.Scale != 1 {
		transform.scale = Lerp(presence.Scale, 1, progress)
	}
	slide := presence.SlideIn
	if leaving {
		slide = presence.SlideOut
	}
	transform.offset = Point{X: slide.X * (1 - progress), Y: slide.Y * (1 - progress)}
	moving := transform.scale != 1 || slide != (Point{})
	if presence.Fade {
		transform.alpha = progress
	}
	fade := presence.Fade
	if fade && progress < 1 {
		vectorizeBorderBackings(canvas, open.rectangles)
	}
	for index := open.rectangles; index < len(canvas.rectangles); index++ {
		rectangle := &canvas.rectangles[index]
		rectangle.Bounds = transform.rect(rectangle.Bounds)
		rectangle.Radius *= transform.scale
		if fade {
			rectangle.Color = transform.color(rectangle.Color)
		}
		if rectangle.HasClip {
			rectangle.ClipOf = transform.rect(rectangle.ClipOf)
		}
	}
	for index := open.shadows; index < len(canvas.shadows); index++ {
		shadow := &canvas.shadows[index]
		shadow.Bounds = transform.rect(shadow.Bounds)
		shadow.Radius *= transform.scale
		shadow.Style.OffsetX *= transform.scale
		shadow.Style.OffsetY *= transform.scale
		shadow.Style.Blur *= transform.scale
		shadow.Style.Spread *= transform.scale
		if fade {
			shadow.Color = transform.color(shadow.Color)
		}
		if shadow.HasClip {
			shadow.ClipOf = transform.rect(shadow.ClipOf)
		}
	}
	for index := open.triangles; index < len(canvas.triangles); index++ {
		triangle := &canvas.triangles[index]
		for corner := range triangle.Points {
			triangle.Points[corner] = transform.point(triangle.Points[corner])
		}
		if fade {
			triangle.Color = transform.color(triangle.Color)
			triangle.Shade[0] = transform.color(triangle.Shade[0])
			triangle.Shade[1] = transform.color(triangle.Shade[1])
		}
		if triangle.HasClip {
			triangle.ClipOf = transform.rect(triangle.ClipOf)
		}
	}
	for index := open.images; index < len(canvas.images); index++ {
		image := &canvas.images[index]
		if image.Backdrop {
			continue
		}
		image.Bounds = transform.rect(image.Bounds)
		if image.HasClip {
			image.ClipOf = transform.rect(image.ClipOf)
		}
	}
	for index := open.texts; index < len(canvas.texts); index++ {
		text := &canvas.texts[index]
		center := transform.point(Point{X: text.Bounds.X + text.Bounds.Width/2, Y: text.Bounds.Y + text.Bounds.Height/2})
		text.Bounds.X, text.Bounds.Y = center.X-text.Bounds.Width/2, center.Y-text.Bounds.Height/2
		if moving {
			// A moving label's sub-pixel origin changes every frame. Native
			// text backends key retained textures by it, so keeping the
			// fraction would rasterize every visible label again on every
			// frame. Whole-pixel motion leaves the origin at zero; the label
			// still moves with its panel, and static text keeps its sub-pixel
			// placement.
			text.Bounds.X = float32(math.Round(float64(text.Bounds.X)))
			text.Bounds.Y = float32(math.Round(float64(text.Bounds.Y)))
		}
		if fade {
			text.Style.Color = transform.color(text.Style.Color)
		}
		if text.HasClip {
			text.ClipOf = transform.rect(text.ClipOf)
		}
	}
	if fade {
		for index := open.runs; index < len(canvas.runs); index++ {
			canvas.runs[index].Color = transform.color(canvas.runs[index].Color)
		}
	}
}
