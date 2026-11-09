package gfx

import "slices"

// A part of a page that has not changed since the last paint can be drawn by
// copying what it painted then, instead of painting it again: a subtree cache
// (ui.Memo) records the stretch of each primitive list a part painted, and
// in a later paint replays that stretch.
//
// The previous paint's lists are what the stretch is copied from, so a canvas
// that replays keeps them: KeepPrevious(true) makes Reset swap the lists with a
// second set instead of emptying them, which costs one more frame of primitives
// in memory and nothing else. A canvas that never asks keeps one set, as before.
//
// A stretch is replayed where it was painted, unmoved: the caller replays only a
// part whose place, clip and layer are as they were. Everything a stretch holds
// is copied, hit regions and native views included, with the indexes that refer
// across lists (Triangle.RectsBefore, Text.RunFirst, Shadow.RectsBefore and
// TrianglesBefore) moved to where the copy lands.

// Mark is a position in every primitive list of a paint: where a stretch starts
// or ends.
type Mark struct {
	rectangles, triangles, shadows, images, texts, runs, hits, nativeViews uint32
	// epoch is the paint the mark was taken in.
	epoch uint32
	// The clip, layer and focus scope in effect at the mark: what everything the
	// stretch holds was clipped to and placed on.
	clip    Rect
	clipped bool
	layer   uint16
	scope   ID
}

// Span is what one part of a paint painted: the stretch of each list between two
// marks of the same paint.
type Span struct{ from, to Mark }

// Empty reports whether the span holds nothing, or was never recorded.
func (span Span) Empty() bool { return span.from.epoch == 0 || span.from == span.to }

// KeepPrevious makes the canvas keep the previous paint's primitives through
// Reset, so a Span recorded then can be replayed. It holds one more frame of
// primitives; turning it off drops them.
func (canvas *Canvas) KeepPrevious(keep bool) {
	canvas.previous.keep = keep
	if !keep {
		canvas.previous.lists = primitiveLists{}
	}
}

// Mark returns where the canvas's lists stand now.
func (canvas *Canvas) Mark() Mark {
	mark := Mark{
		rectangles: uint32(len(canvas.rectangles)), triangles: uint32(len(canvas.triangles)),
		shadows: uint32(len(canvas.shadows)), images: uint32(len(canvas.images)),
		texts: uint32(len(canvas.texts)), runs: uint32(len(canvas.runs)),
		hits: uint32(len(canvas.hits)), nativeViews: uint32(len(canvas.nativeViews)),
		epoch: canvas.previous.epoch, layer: canvas.currentLayer(), scope: canvas.currentFocusScope(),
	}
	mark.clip, mark.clipped = canvas.clipSnapshot()
	return mark
}

// Delivering reports whether this paint is delivering a click or a pointer event
// (an event pass), in which a declaration runs its handlers: a part that is
// replayed is not declared, so a cache declares every part in such a paint.
func (canvas *Canvas) Delivering() bool { return canvas.eventPass || canvas.hasPointer }

// SpanSince is what the canvas painted since from, in this paint.
func (canvas *Canvas) SpanSince(from Mark) Span { return Span{from: from, to: canvas.Mark()} }

// CanReplay reports whether span was recorded in the previous paint of a canvas
// that keeps it, under the clip, layer and focus scope in effect now, so Replay
// can copy it unchanged.
func (canvas *Canvas) CanReplay(span Span) bool {
	if !canvas.previous.keep || span.Empty() || span.from.epoch+1 != canvas.previous.epoch || !canvas.previous.lists.holds(span) {
		return false
	}
	clip, clipped := canvas.clipSnapshot()
	f := span.from
	return f.layer == canvas.currentLayer() && f.scope == canvas.currentFocusScope() && f.clipped == clipped && (!clipped || f.clip == clip)
}

// Replay appends what span painted in the previous paint, and returns the span
// of the copy in this paint, to be replayed again by the next. It reports false,
// and appends nothing, when CanReplay does.
func (canvas *Canvas) Replay(span Span) (Span, bool) { return canvas.ReplayAt(span, 0, 0) }

// ReplayAt is Replay moved by (dx, dy): a part that is unchanged but sits
// elsewhere, a row of a list that scrolled. Under a clip it is replayed moved only
// when everything it painted lay inside the clip then and lies inside it moved,
// since the clip cut nothing of it then and must cut nothing now.
func (canvas *Canvas) ReplayAt(span Span, dx, dy float32) (Span, bool) {
	if !canvas.CanReplay(span) {
		return Span{}, false
	}
	if (dx != 0 || dy != 0) && span.from.clipped {
		extent, ok := canvas.previous.lists.extent(span)
		if !ok || !contains(span.from.clip, extent) || !contains(span.from.clip, extent.Offset(dx, dy)) {
			return Span{}, false
		}
	}
	from := canvas.Mark()
	old := &canvas.previous.lists
	f, t := span.from, span.to
	canvas.rectangles = append(canvas.rectangles, old.rectangles[f.rectangles:t.rectangles]...)
	rectShift := int64(from.rectangles) - int64(f.rectangles)
	triShift := int64(from.triangles) - int64(f.triangles)
	start := len(canvas.triangles)
	canvas.triangles = append(canvas.triangles, old.triangles[f.triangles:t.triangles]...)
	for index := start; index < len(canvas.triangles); index++ {
		canvas.triangles[index].RectsBefore = uint32(int64(canvas.triangles[index].RectsBefore) + rectShift)
	}
	start = len(canvas.shadows)
	canvas.shadows = append(canvas.shadows, old.shadows[f.shadows:t.shadows]...)
	for index := start; index < len(canvas.shadows); index++ {
		shadow := &canvas.shadows[index]
		shadow.RectsBefore = uint32(int64(shadow.RectsBefore) + rectShift)
		shadow.TrianglesBefore = uint32(int64(shadow.TrianglesBefore) + triShift)
	}
	canvas.images = append(canvas.images, old.images[f.images:t.images]...)
	runShift := int32(int64(from.runs) - int64(f.runs))
	start = len(canvas.texts)
	canvas.texts = append(canvas.texts, old.texts[f.texts:t.texts]...)
	for index := start; index < len(canvas.texts); index++ {
		if canvas.texts[index].RunCount > 0 {
			canvas.texts[index].RunFirst += runShift
		}
	}
	canvas.runs = append(canvas.runs, old.runs[f.runs:t.runs]...)
	canvas.hits = append(canvas.hits, old.hits[f.hits:t.hits]...)
	canvas.nativeViews = append(canvas.nativeViews, old.nativeViews[f.nativeViews:t.nativeViews]...)
	if dx != 0 || dy != 0 {
		canvas.moveSince(from, dx, dy)
	}
	return canvas.SpanSince(from), true
}

// moveSince moves everything painted since from by (dx, dy). Clips stay: the
// caller made sure they cut nothing.
func (canvas *Canvas) moveSince(from Mark, dx, dy float32) {
	for i := from.rectangles; i < uint32(len(canvas.rectangles)); i++ {
		canvas.rectangles[i].Bounds = canvas.rectangles[i].Bounds.Offset(dx, dy)
	}
	for i := from.triangles; i < uint32(len(canvas.triangles)); i++ {
		points := &canvas.triangles[i].Points
		for k := range points {
			points[k].X += dx
			points[k].Y += dy
		}
	}
	for i := from.shadows; i < uint32(len(canvas.shadows)); i++ {
		canvas.shadows[i].Bounds = canvas.shadows[i].Bounds.Offset(dx, dy)
	}
	for i := from.images; i < uint32(len(canvas.images)); i++ {
		canvas.images[i].Bounds = canvas.images[i].Bounds.Offset(dx, dy)
	}
	for i := from.texts; i < uint32(len(canvas.texts)); i++ {
		canvas.texts[i].Bounds = canvas.texts[i].Bounds.Offset(dx, dy)
	}
	for i := from.hits; i < uint32(len(canvas.hits)); i++ {
		hit := &canvas.hits[i]
		hit.Bounds = hit.Bounds.Offset(dx, dy)
		if hit.TextBounds.Width != 0 || hit.TextBounds.Height != 0 {
			hit.TextBounds = hit.TextBounds.Offset(dx, dy)
		}
	}
	for i := from.nativeViews; i < uint32(len(canvas.nativeViews)); i++ {
		canvas.nativeViews[i].Bounds = canvas.nativeViews[i].Bounds.Offset(dx, dy)
	}
}

// Rewind takes back everything painted since to, in this paint: a part that is
// painted again a better way (a cache that found its guess wrong) starts over
// from where it began. It reports false, and changes nothing, for a mark of
// another paint or one past what was painted.
func (canvas *Canvas) Rewind(to Mark) bool {
	if to.epoch != canvas.previous.epoch || int(to.rectangles) > len(canvas.rectangles) || int(to.triangles) > len(canvas.triangles) ||
		int(to.shadows) > len(canvas.shadows) || int(to.images) > len(canvas.images) || int(to.texts) > len(canvas.texts) ||
		int(to.runs) > len(canvas.runs) || int(to.hits) > len(canvas.hits) || int(to.nativeViews) > len(canvas.nativeViews) {
		return false
	}
	clear(canvas.texts[to.texts:]) // their strings are not kept past the rewind
	clear(canvas.hits[to.hits:])
	clear(canvas.images[to.images:])
	canvas.rectangles = canvas.rectangles[:to.rectangles]
	canvas.triangles = canvas.triangles[:to.triangles]
	canvas.shadows = canvas.shadows[:to.shadows]
	canvas.images = canvas.images[:to.images]
	canvas.texts = canvas.texts[:to.texts]
	canvas.runs = canvas.runs[:to.runs]
	canvas.hits = canvas.hits[:to.hits]
	canvas.nativeViews = canvas.nativeViews[:to.nativeViews]
	kept := canvas.motion.hitMotions[:0]
	for _, entry := range canvas.motion.hitMotions {
		if entry.index < int(to.hits) {
			kept = append(kept, entry)
		}
	}
	canvas.motion.hitMotions = kept
	return true
}

// PreviousHits are the hit regions span declared, in the previous paint: what a
// cache asks to know whether a part's controls are hovered, pressed or focused.
func (canvas *Canvas) PreviousHits(span Span) []Hit {
	if !canvas.previous.keep || span.Empty() || span.from.epoch+1 != canvas.previous.epoch || !canvas.previous.lists.holds(span) {
		return nil
	}
	return canvas.previous.lists.hits[span.from.hits:span.to.hits]
}

// HitsSince are the hit regions declared since from, in this paint.
func (canvas *Canvas) HitsSince(from Mark) []Hit {
	if int(from.hits) > len(canvas.hits) {
		return nil
	}
	return canvas.hits[from.hits:]
}

// SameAsPrevious reports whether what span painted in this paint is what prev
// painted in the previous one, moved by (dx, dy): the check of a cache that
// replays prev in place of painting span.
func (canvas *Canvas) SameAsPrevious(span, prev Span, dx, dy float32) bool {
	if !canvas.previous.keep || prev.from.epoch+1 != canvas.previous.epoch || !canvas.previous.lists.holds(prev) {
		return false
	}
	var scratch Canvas
	scratch.previous = previousPaint{keep: true, epoch: canvas.previous.epoch, lists: canvas.previous.lists}
	scratch.layers, scratch.focusScopes = canvas.layers, canvas.focusScopes
	scratch.clips, scratch.overlayClips = canvas.clips, canvas.overlayClips
	scratch.rectangles = make([]Rectangle, 0, prev.to.rectangles-prev.from.rectangles)
	scratch.rectangles = append(scratch.rectangles, canvas.rectangles[:span.from.rectangles]...)
	scratch.triangles = append(scratch.triangles, canvas.triangles[:span.from.triangles]...)
	scratch.shadows = append(scratch.shadows, canvas.shadows[:span.from.shadows]...)
	scratch.images = append(scratch.images, canvas.images[:span.from.images]...)
	scratch.texts = append(scratch.texts, canvas.texts[:span.from.texts]...)
	scratch.runs = append(scratch.runs, canvas.runs[:span.from.runs]...)
	scratch.hits = append(scratch.hits, canvas.hits[:span.from.hits]...)
	scratch.nativeViews = append(scratch.nativeViews, canvas.nativeViews[:span.from.nativeViews]...)
	prev.from.clip, prev.from.clipped, prev.from.layer, prev.from.scope = span.from.clip, span.from.clipped, span.from.layer, span.from.scope
	copied, ok := scratch.ReplayAt(prev, dx, dy)
	if !ok {
		return false
	}
	return slices.Equal(canvas.rectangles[span.from.rectangles:span.to.rectangles], scratch.rectangles[copied.from.rectangles:]) &&
		slices.Equal(canvas.triangles[span.from.triangles:span.to.triangles], scratch.triangles[copied.from.triangles:]) &&
		slices.Equal(canvas.shadows[span.from.shadows:span.to.shadows], scratch.shadows[copied.from.shadows:]) &&
		slices.EqualFunc(canvas.images[span.from.images:span.to.images], scratch.images[copied.from.images:], sameImage) &&
		slices.Equal(canvas.texts[span.from.texts:span.to.texts], scratch.texts[copied.from.texts:]) &&
		slices.Equal(canvas.runs[span.from.runs:span.to.runs], scratch.runs[copied.from.runs:]) &&
		slices.EqualFunc(canvas.hits[span.from.hits:span.to.hits], scratch.hits[copied.from.hits:], sameHit) &&
		slices.Equal(canvas.nativeViews[span.from.nativeViews:span.to.nativeViews], scratch.nativeViews[copied.from.nativeViews:])
}

func sameImage(a, b Image) bool {
	return a.Source == b.Source && a.Bounds == b.Bounds && a.Layer == b.Layer && a.ClipOf == b.ClipOf && a.HasClip == b.HasClip &&
		a.Backdrop == b.Backdrop && a.DevicePixels == b.DevicePixels && a.Bitmap.Width == b.Bitmap.Width && a.Bitmap.Height == b.Bitmap.Height
}

func sameHit(a, b Hit) bool {
	return a.ID == b.ID && a.Bounds == b.Bounds && a.Label == b.Label && a.Value == b.Value && a.Layer == b.Layer && a.Scope == b.Scope &&
		a.Role == b.Role && a.Disabled == b.Disabled && a.Checked == b.Checked && a.Selected == b.Selected && a.Focusable == b.Focusable &&
		a.IsLabel == b.IsLabel && a.Backdrop == b.Backdrop && a.Block == b.Block
}

func contains(outer, inner Rect) bool {
	return inner.X >= outer.X && inner.Y >= outer.Y && inner.X+inner.Width <= outer.X+outer.Width && inner.Y+inner.Height <= outer.Y+outer.Height
}

// primitiveLists is one paint's primitives.
type primitiveLists struct {
	rectangles  []Rectangle
	triangles   []Triangle
	shadows     []Shadow
	images      []Image
	texts       []Text
	runs        []TextRun
	hits        []Hit
	nativeViews []NativeView
}

// extent is the rectangle everything span painted lies in, and false when it
// painted nothing with an extent.
func (lists *primitiveLists) extent(span Span) (Rect, bool) {
	f, t := span.from, span.to
	var out Rect
	have := false
	grow := func(r Rect) {
		if !have {
			out, have = r, true
			return
		}
		x0, y0 := min(out.X, r.X), min(out.Y, r.Y)
		out = Rect{X: x0, Y: y0, Width: max(out.X+out.Width, r.X+r.Width) - x0, Height: max(out.Y+out.Height, r.Y+r.Height) - y0}
	}
	for _, r := range lists.rectangles[f.rectangles:t.rectangles] {
		grow(r.Bounds)
	}
	for _, s := range lists.shadows[f.shadows:t.shadows] {
		grow(s.Bounds.Inset(All(-s.Style.Extent())))
	}
	for _, tr := range lists.triangles[f.triangles:t.triangles] {
		for _, p := range tr.Points {
			grow(Rect{X: p.X, Y: p.Y})
		}
	}
	for _, i := range lists.images[f.images:t.images] {
		grow(i.Bounds)
	}
	for _, x := range lists.texts[f.texts:t.texts] {
		grow(x.Bounds)
	}
	for _, h := range lists.hits[f.hits:t.hits] {
		grow(h.Bounds)
	}
	for _, v := range lists.nativeViews[f.nativeViews:t.nativeViews] {
		grow(v.Bounds)
	}
	return out, have
}

func (lists *primitiveLists) holds(span Span) bool {
	t := span.to
	return int(t.rectangles) <= len(lists.rectangles) && int(t.triangles) <= len(lists.triangles) &&
		int(t.shadows) <= len(lists.shadows) && int(t.images) <= len(lists.images) &&
		int(t.texts) <= len(lists.texts) && int(t.runs) <= len(lists.runs) &&
		int(t.hits) <= len(lists.hits) && int(t.nativeViews) <= len(lists.nativeViews)
}

// previousPaint is the previous paint's lists, kept for Replay, and the number of
// the paint now in progress.
type previousPaint struct {
	keep  bool
	epoch uint32
	lists primitiveLists
}

// rotate starts a paint: with keep, the lists just painted become the previous
// paint's and the previous paint's arrays are reused for this one.
func (canvas *Canvas) rotate() {
	canvas.previous.epoch++
	if !canvas.previous.keep {
		return
	}
	old := &canvas.previous.lists
	canvas.rectangles, old.rectangles = old.rectangles[:0], canvas.rectangles
	canvas.triangles, old.triangles = old.triangles[:0], canvas.triangles
	canvas.shadows, old.shadows = old.shadows[:0], canvas.shadows
	canvas.images, old.images = old.images[:0], canvas.images
	canvas.texts, old.texts = old.texts[:0], canvas.texts
	canvas.runs, old.runs = old.runs[:0], canvas.runs
	canvas.hits, old.hits = old.hits[:0], canvas.hits
	canvas.nativeViews, old.nativeViews = old.nativeViews[:0], canvas.nativeViews
}

// trimPrevious gives back the previous paint's arrays after a large page, as Trim
// does for the current one.
func (canvas *Canvas) trimPrevious() {
	old := &canvas.previous.lists
	old.rectangles = trimCapacity(old.rectangles)
	old.triangles = trimCapacity(old.triangles)
	old.shadows = trimCapacity(old.shadows)
	old.images = trimCapacity(old.images)
	old.texts = trimCapacity(old.texts)
	old.runs = trimCapacity(old.runs)
	old.hits = trimCapacity(old.hits)
	old.nativeViews = trimCapacity(old.nativeViews)
}
