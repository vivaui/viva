package ui

import "github.com/vivaui/viva/gfx"

// Memo declares the part of a page fn declares, and in a later paint, when
// version is what it was and nothing the part shows can have changed, does not
// declare it at all: the page keeps its room and the canvas copies what it painted
// last time (gfx.Canvas.Replay). A part of a page that is unchanged costs a lookup.
//
//	for i := range m.hosts {
//		h := &m.hosts[i]
//		ui.Memo(c.KeyInt(h.ID), h.Version, func() { m.row(c, h) })
//	}
//
// version is everything fn reads that can change: bump it when the part's data
// does. The rest is the Memo's business; the part is declared again, as if it had
// no Memo, in a paint that delivers a click or a pointer event, while one of its
// controls is hovered, pressed or focused (or was in the last paint), while a
// transition runs anywhere on the canvas, and when the window's size changed. A
// part that moved (a row of a list that scrolled, or under a row that was added)
// is copied moved; one that changed size, or moved where its clip would cut it, is
// declared again in the same paint (the page's paint is taken back and done again,
// once).
//
// A Memo holds no Scroll, List, Popover or Modal: their state is kept by being
// declared. MemoCheck(true) declares every part and panics where a copy would have
// differed from it: run tests with it.
func Memo(c C, version uint64, fn func()) Box {
	p := c.page
	at, _ := p.name(pkMemo)
	id := p.id(at)
	entry := p.memos[id]
	if entry == nil {
		entry = &memoEntry{}
		p.memos[id] = entry
	}
	f := c.f
	if p.memoHit(entry, version) && !memoCheck {
		b := f.Slot().Size(entry.rect.Width, entry.rect.Height).Shrink(0)
		f.extra(b.i).memo = p.memoBox(entry, true)
		p.touchMemo(entry)
		entry.epoch, entry.version = p.epoch, version
		if entry.listens && f.canvas != nil {
			// The part's controls were not asked about a click this paint, as
			// they were not declared: say they listen, so that the runtime
			// delivers the next click and the part is declared to take it.
			f.canvas.Clicked(gfx.ID{})
			f.canvas.PointerOn(gfx.ID{})
		}
		return b
	}
	var prev gfx.Span
	var prevAt Rect
	if memoCheck && entry.painted && entry.epoch+1 == p.epoch && entry.version == version {
		prev, prevAt = entry.span, entry.rect
	}
	entry.reset()
	entry.version = version
	p.recording = append(p.recording, entry)
	// The part is named by the Memo's own place, so what is in it keeps its names
	// whether the parts around it were declared or copied.
	b := f.Column()
	p.setNode(b.i, at)
	b.Children(fn)
	p.recording = p.recording[:len(p.recording)-1]
	if parent := len(p.recording); parent > 0 {
		p.recording[parent-1].memos = append(p.recording[parent-1].memos, entry)
	}
	f.extra(b.i).memo = p.memoBox(entry, false)
	if !prev.Empty() {
		p.memoChecks = append(p.memoChecks, memoCheckOf{span: prev, at: prevAt})
		p.memoBoxes[len(p.memoBoxes)-1].check = int32(len(p.memoChecks))
	}
	entry.epoch = p.epoch
	return b
}

// memoCheck declares every Memo's part and compares it with what a copy would
// have been.
var memoCheck bool

// MemoCheck(true) makes every Memo declare its part and panic where a copy of the
// last paint would have differed: a version that does not cover what fn reads. It
// is for tests; it costs what having no Memo costs, and more.
func MemoCheck(on bool) { memoCheck = on }

// memoEntry is what a page keeps for one Memo between paints.
type memoEntry struct {
	version uint64
	epoch   uint32   // the paint the part was last declared or copied in
	rect    Rect     // where it was
	bounds  Rect     // the page's bounds then
	span    gfx.Span // what it painted
	painted bool
	held    bool // one of its controls was hovered, pressed or focused
	stale   bool // its copy failed: declare it in the next try
	listens bool // it declared hit regions: a click or pointer event may be for it

	// What declaring the part told the page, so that a copy keeps it: the
	// names of its bound values, its rows' keys, its State values and the Memos
	// inside it.
	bound   []boundKey
	names   []gfx.ID
	keys    []keyKey
	intKeys []intKey
	states  []protoStateKey
	memos   []*memoEntry
}

func (entry *memoEntry) reset() {
	entry.bound, entry.names, entry.keys = entry.bound[:0], entry.names[:0], entry.keys[:0]
	entry.intKeys, entry.states = entry.intKeys[:0], entry.states[:0]
	clear(entry.memos)
	entry.memos = entry.memos[:0]
	entry.painted, entry.held = false, false
}

// memoBox is a Memo's element of this paint.
type memoBox struct {
	entry  *memoEntry
	before gfx.Mark
	copy   bool
	check  int32 // 1 + the index of what MemoCheck compares with in Page.memoChecks, or 0
}

// memoCheckOf is the last paint's span of a part MemoCheck declared again.
type memoCheckOf struct {
	span gfx.Span
	at   Rect
}

func (p *Page) memoBox(entry *memoEntry, copy bool) int32 {
	box := memoBox{entry: entry, copy: copy}
	p.memoBoxes = append(p.memoBoxes, box)
	if p.ctx.f.memoFn == nil {
		p.ctx.f.memoFn = p.memoPaint // a method value: made once, not per Memo
	}
	return int32(len(p.memoBoxes))
}

// memoHit reports whether the part may be copied rather than declared.
func (p *Page) memoHit(entry *memoEntry, version uint64) bool {
	canvas := p.ctx.f.canvas
	if canvas == nil || !entry.painted || entry.stale || entry.version != version || entry.epoch+1 != p.epoch ||
		entry.held || entry.bounds != p.ctx.f.bounds || canvas.Delivering() || canvas.Animating() {
		return false
	}
	hovered, pressed, focused := canvas.Hovered(), canvas.Pressed(), canvas.Focused()
	for _, hit := range canvas.PreviousHits(entry.span) {
		if !hit.ID.IsZero() && (hit.ID == hovered || hit.ID == pressed || hit.ID == focused) {
			return false
		}
	}
	return canvas.CanReplay(entry.span) || entry.span.Empty()
}

// memoPaint is the Frame's paint hook for a Memo's element.
func (p *Page) memoPaint(f *Frame, i int32, before bool) bool {
	box := &p.memoBoxes[f.exts[f.nodes[i].ext].memo-1]
	entry := box.entry
	canvas := f.canvas
	rect := f.nodes[i].rect
	if box.copy {
		dx, dy := rect.X-entry.rect.X, rect.Y-entry.rect.Y
		sameSize := rect.Width == entry.rect.Width && rect.Height == entry.rect.Height
		span, ok := gfx.Span{}, entry.span.Empty() && sameSize
		if sameSize && !entry.span.Empty() {
			span, ok = canvas.ReplayAt(entry.span, dx, dy)
		}
		if !ok {
			entry.stale = true
			p.retry = true
			return true
		}
		entry.span, entry.rect = span, rect
		return true
	}
	if before {
		box.before = canvas.Mark()
		return false
	}
	entry.span = canvas.SpanSince(box.before)
	entry.listens = len(canvas.HitsSince(box.before)) > 0
	entry.rect, entry.bounds, entry.painted, entry.stale = rect, f.bounds, true, false
	hovered, pressed, focused := canvas.Hovered(), canvas.Pressed(), canvas.Focused()
	for _, hit := range canvas.HitsSince(box.before) {
		if !hit.ID.IsZero() && (hit.ID == hovered || hit.ID == pressed || hit.ID == focused) {
			entry.held = true
		}
	}
	if box.check != 0 && !entry.held && !canvas.Delivering() && !canvas.Animating() && p.memoDiffers(entry.span, p.memoChecks[box.check-1], rect) {
		panic("ui: MemoCheck: the part a Memo declared (" + p.tree.Text(f.nodes[i].id) + ") differs from what it painted last time " +
			"with the same version: the version does not cover something its function reads")
	}
	return false
}

func (p *Page) memoDiffers(span gfx.Span, prev memoCheckOf, rect Rect) bool {
	if prev.at.Width != rect.Width || prev.at.Height != rect.Height {
		return false // laid out at another size: not a copy's place to compare
	}
	return !p.ctx.f.canvas.SameAsPrevious(span, prev.span, rect.X-prev.at.X, rect.Y-prev.at.Y)
}

// touchMemo tells the page that what a copied part declared last time is still on
// the page: its names, keys and states are kept, as if it had been declared.
func (p *Page) touchMemo(entry *memoEntry) {
	for _, key := range entry.bound {
		if bound, ok := p.bound[key]; ok {
			bound.epoch = p.epoch
			p.bound[key] = bound
		}
	}
	for _, id := range entry.names {
		if use, ok := p.names[id]; ok {
			use.seen = p.epoch
			p.names[id] = use
		}
	}
	for _, key := range entry.keys {
		if e, ok := p.keys[key]; ok {
			e.epoch = p.epoch
			p.keys[key] = e
		}
	}
	for _, key := range entry.intKeys {
		if e, ok := p.intKeys[key]; ok {
			e.epoch = p.epoch
			p.intKeys[key] = e
		}
	}
	for _, key := range entry.states {
		if s, ok := p.states[key]; ok {
			s.epoch = p.epoch
			p.states[key] = s
		}
	}
	for _, inner := range entry.memos {
		inner.epoch = p.epoch
		p.touchMemo(inner)
	}
}

// noteMemo records, in every Memo being declared, what one declaration told the page.
func (p *Page) noteBound(key boundKey, id gfx.ID) {
	for _, entry := range p.recording {
		entry.bound = append(entry.bound, key)
		entry.names = append(entry.names, id)
	}
}

func (p *Page) noteKey(key keyKey) {
	for _, entry := range p.recording {
		if key.isText {
			entry.keys = append(entry.keys, key)
		} else {
			entry.intKeys = append(entry.intKeys, intKey{parent: key.parent, number: key.number})
		}
	}
}

func (p *Page) noteState(key protoStateKey) {
	for _, entry := range p.recording {
		entry.states = append(entry.states, key)
	}
}

// forbidInMemo stops an element a copy cannot keep from being declared in a Memo.
func (p *Page) forbidInMemo(what string) {
	if len(p.recording) > 0 {
		panic("ui: a " + what + " in a Memo: its state is kept by being declared, so a copied part would lose it; declare it outside the Memo")
	}
}

// sweepMemos forgets the Memos the paint did not declare or copy.
func (p *Page) sweepMemos() {
	for id, entry := range p.memos {
		if entry.epoch+1 < p.epoch {
			delete(p.memos, id)
		}
	}
	clear(p.memoBoxes)
	p.memoBoxes = p.memoBoxes[:0]
	p.memoChecks = p.memoChecks[:0]
}
