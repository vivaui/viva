package textmetrics

import (
	"sync"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/grapheme"
)

// Bidirectional text. The width of a line does not depend on the order its
// characters are displayed in, so layout, wrapping and the box of a label are the same
// for left-to-right and right-to-left text. What changes is where each piece of the line
// is drawn: the pieces of Runs are placed in the visual order UAX #9 gives, a piece of
// right-to-left text is still drawn by the platform's own engine as the string it is (the
// engines reorder and shape inside a piece, and a piece never changes direction), and
// the x of each is the layout width of the pieces visually to its left.
//
// A line is resolved on its own: its paragraph direction is that of its first strong
// character (P2, P3, so an RLM or LRM at the start forces it) and a line of a wrapped
// paragraph is reordered as a line of that paragraph (L1 and L2). Text with no right-to-left
// letter, number or explicit control never reaches this file: NeedsBidi is false, and the
// left-to-right code of Runs and Width runs as before.

// piece is a word or full-width character of a line, with the layout position the bidi
// reordering gives it.
type piece struct {
	start, end int
	wordEnd    int // where the word the piece belongs to ends: pieces of one word share it
	x, width   int // layout units
	level      uint8
}

// bidiLine is the reusable scratch of one bidi layout. Pooled so that concurrent users
// each have their own and a warm one allocates nothing.
type bidiLine struct {
	resolver bidi.Resolver
	isolated bidi.Resolver // resolves a word on its own, as the platform's engine does
	runs     []bidi.Run
	wordRuns []bidi.Run
	pieces   []piece // in visual order, left to right
	glyphs   [2][]rune
	para     bidi.Paragraph
	weight   int
}

var bidiPool = sync.Pool{New: func() any { return new(bidiLine) }}

// maxRetained is the largest scratch slice a pooled line keeps.
const maxRetained = 4096

func (b *bidiLine) release() {
	if cap(b.pieces) > maxRetained || cap(b.runs) > maxRetained || cap(b.glyphs[0]) > maxRetained {
		b.pieces, b.runs, b.wordRuns, b.glyphs = nil, nil, nil, [2][]rune{}
	}
	b.para = bidi.Paragraph{}
	bidiPool.Put(b)
}

// resolve fills b.runs for value as one line of its own paragraph.
func (b *bidiLine) resolve(value string, weight int) {
	b.weight = weight
	b.para = b.resolver.Resolve(value, bidi.Auto)
	b.runs = b.resolver.LineRuns(b.para, 0, len(value), b.runs[:0])
}

// layout places the pieces of the resolved line in visual order.
func (b *bidiLine) layout(value string) {
	b.pieces = b.pieces[:0]
	x := 0
	for _, run := range b.runs {
		first := len(b.pieces)
		for from := run.Start; ; {
			start, end, ok := NextRun(value[:run.End], from)
			if !ok {
				break
			}
			_, wordEnd, _ := NextRun(value, start)
			b.pieces = append(b.pieces, piece{start: start, end: end, wordEnd: wordEnd, width: units(value[start:end], b.weight), level: run.Level})
			from = end
		}
		pieces := b.pieces[first:]
		if run.Level&1 == 0 {
			at := run.Start
			for i := range pieces {
				x += units(value[at:pieces[i].start], b.weight)
				pieces[i].x = x
				x += pieces[i].width
				at = pieces[i].end
			}
			x += units(value[at:run.End], b.weight)
			continue
		}
		// Right to left: the last piece is the leftmost. Walk from the right edge of the run.
		for i, j := 0, len(pieces)-1; i < j; i, j = i+1, j-1 {
			pieces[i], pieces[j] = pieces[j], pieces[i]
		}
		end := run.End
		for i := range pieces {
			x += units(value[pieces[i].end:end], b.weight)
			pieces[i].x = x
			x += pieces[i].width
			end = pieces[i].start
		}
		x += units(value[run.Start:end], b.weight)
	}
	b.mergeWords(value)
}

// mergeWords joins the pieces a word was cut into at changes of level when the platform's
// engine, given the whole word, displays it as UAX #9 does in the line. A word such as
// (192.168.1.1) between Hebrew words has its brackets at the level of the Hebrew and its
// digits above it: three pieces, of which the brackets are single characters that an
// engine would not mirror. Drawn whole, the engine resolves the word on its own, which
// shows the same glyphs in the same order, mirroring included; so it is one piece. A word
// whose neighbours change how it reads (an English word in a Hebrew line, run into a
// Hebrew one) stays cut where the levels change.
func (b *bidiLine) mergeWords(value string) {
	out := b.pieces[:0]
	for i := 0; i < len(b.pieces); {
		j := i + 1
		for j < len(b.pieces) && b.pieces[j].wordEnd == b.pieces[i].wordEnd {
			j++
		}
		group := b.pieces[i:j]
		if len(group) > 1 {
			start, left := group[0].start, group[0].x
			for _, p := range group {
				start, left = min(start, p.start), min(left, p.x)
			}
			end := group[0].wordEnd
			if b.readsAlike(value, start, end) {
				out = append(out, piece{start: start, end: end, wordEnd: end, x: left, width: units(value[start:end], b.weight)})
				i = j
				continue
			}
		}
		out = append(out, group...)
		i = j
	}
	b.pieces = out
}

// readsAlike reports whether value[start:end] shows the same glyphs in the same order
// whether it is displayed as part of the line or resolved by itself.
func (b *bidiLine) readsAlike(value string, start, end int) bool {
	b.wordRuns = b.resolver.LineRuns(b.para, start, end, b.wordRuns[:0])
	b.glyphs[0] = displayed(b.glyphs[0][:0], value, b.wordRuns)
	word := value[start:end]
	alone := b.isolated.Resolve(word, bidi.Auto)
	b.wordRuns = b.isolated.LineRuns(alone, 0, len(word), b.wordRuns[:0])
	b.glyphs[1] = displayed(b.glyphs[1][:0], word, b.wordRuns)
	if len(b.glyphs[0]) != len(b.glyphs[1]) {
		return false
	}
	for i, r := range b.glyphs[0] {
		if b.glyphs[1][i] != r {
			return false
		}
	}
	return true
}

// displayed appends the glyphs of text in display order, left to right, given its runs in
// visual order (byte offsets into text): a run of odd level is reversed and its mirrored
// characters replaced by their mirror images (L4).
func displayed(dst []rune, text string, runs []bidi.Run) []rune {
	for _, run := range runs {
		segment := text[run.Start:run.End]
		if run.Level&1 == 0 {
			for _, r := range segment {
				dst = append(dst, r)
			}
			continue
		}
		for end := len(segment); end > 0; {
			r, w := utf8.DecodeLastRuneInString(segment[:end])
			dst = append(dst, bidi.Mirrored(r))
			end -= w
		}
	}
	return dst
}

// RightToLeft reports whether value is a right-to-left paragraph: its first strong
// character (UAX #9 P2, P3) is Hebrew, Arabic or another right-to-left letter. It is how
// a label chooses between the two ends of its box when it is aligned to the start or
// the end of the text.
func RightToLeft(value string) bool { return bidi.BaseDirection(value) == bidi.RTL }

// Bidirectional reports whether value is laid out in more than one direction or from
// right to left, so that its pieces are drawn in an order other than the logical one.
func Bidirectional(value string) bool { return bidi.NeedsBidi(value) }

// Span is a horizontal extent of a line in logical pixels.
type Span struct{ X, Width float32 }

// Caret is the x, in logical pixels from the left end of the line, of the caret at byte
// offset in value: on the leading edge of the character at offset, or the trailing edge of
// the last one at the end, so it moves from the right in a right-to-left run (and sits at
// the right end of a right-to-left line at offset 0). Offsets inside a grapheme cluster
// snap back to its start.
func Caret(value string, size, weight float32, offset int) float32 {
	offset = max(0, min(offset, len(value)))
	if !bidi.NeedsBidi(value) {
		return Width(value[:snap(value, offset)], size, weight)
	}
	table := weightIndex(weight)
	b := bidiPool.Get().(*bidiLine)
	b.resolve(value, table)
	units := b.edge(value, snap(value, offset))
	b.release()
	return float32(units) * size / unitsPerEm
}

// snap moves offset back to the start of its grapheme cluster.
func snap(value string, offset int) int {
	for offset > 0 && offset < len(value) && !grapheme.Boundary(value, offset) {
		offset--
	}
	return offset
}

// edge is the x in layout units of the caret at byte offset: the boundary between two
// characters is drawn at the trailing edge of the one before it when that reads in the
// direction of the paragraph, so that typing there continues it, and at the leading edge
// of the one after it otherwise; at the ends of the line there is only one choice.
func (b *bidiLine) edge(value string, offset int) int {
	if len(b.runs) == 0 {
		return 0
	}
	before, after := b.runAt(offset-1), b.runAt(offset)
	use := after
	if offset >= len(value) || offset > 0 && before.Level&1 == b.para.Level&1 {
		use = before
	}
	x := 0
	for _, run := range b.runs {
		if run.Start == use.Start && run.End == use.End {
			return x + b.runEdge(value, run, offset)
		}
		x += units(value[run.Start:run.End], b.weight)
	}
	return x
}

func (b *bidiLine) runEdge(value string, run bidi.Run, offset int) int {
	if run.Level&1 != 0 {
		return units(value[offset:run.End], b.weight)
	}
	return units(value[run.Start:offset], b.weight)
}

// runAt is the run that holds the character at byte offset (the runs are in visual order);
// offsets before the line belong to its first run in logical order, offsets after it to its
// last.
func (b *bidiLine) runAt(offset int) bidi.Run {
	first, last := b.runs[0], b.runs[0]
	for _, run := range b.runs {
		if run.Start <= offset && offset < run.End {
			return run
		}
		if run.Start < first.Start {
			first = run
		}
		if run.End > last.End {
			last = run
		}
	}
	if offset < 0 {
		return first
	}
	return last
}

// Selection appends to dst the extents of the selection [start, end) of value, a line
// drawn from x = 0: one span for each stretch of the selection that is contiguous on the
// screen, in left to right order. A selection that crosses a change of direction is
// drawn as several spans.
func Selection(dst []Span, value string, size, weight float32, start, end int) []Span {
	start, end = max(0, min(start, len(value))), max(0, min(end, len(value)))
	if start > end {
		start, end = end, start
	}
	if start == end {
		return dst
	}
	start, end = snap(value, start), snap(value, end)
	if !bidi.NeedsBidi(value) {
		left := Width(value[:start], size, weight)
		return append(dst, Span{X: left, Width: Width(value[start:end], size, weight)})
	}
	table := weightIndex(weight)
	b := bidiPool.Get().(*bidiLine)
	b.resolve(value, table)
	scale := size / unitsPerEm
	x := 0
	for _, run := range b.runs {
		a, z := max(start, run.Start), min(end, run.End)
		if a < z {
			var left int
			if run.Level&1 == 0 {
				left = x + units(value[run.Start:a], table)
			} else {
				left = x + units(value[z:run.End], table)
			}
			span := Span{X: float32(left) * scale, Width: float32(units(value[a:z], table)) * scale}
			if n := len(dst); n > 0 && absf(dst[n-1].X+dst[n-1].Width-span.X) < 0.001 {
				dst[n-1].Width += span.Width
			} else {
				dst = append(dst, span)
			}
		}
		x += units(value[run.Start:run.End], table)
	}
	b.release()
	return dst
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// OffsetAt is the byte offset of the character boundary nearest to x, a distance in
// logical pixels from the left end of the line, which a click at x puts the caret at.
// Of two boundaries as near, the earlier.
func OffsetAt(value string, size, weight float32, x float32) int {
	if !bidi.NeedsBidi(value) {
		return NearestBoundary(value, x, func(offset int) float32 { return Width(value[:offset], size, weight) })
	}
	b := bidiPool.Get().(*bidiLine)
	b.resolve(value, weightIndex(weight))
	hit := b.nearestPrimary(value, size, x)
	b.release()
	return hit.Offset
}

// CaretHit is both the source offset and the visual position of a text hit.
// At a direction change there may be two positions for the same byte offset.
type CaretHit struct {
	Offset int
	X      float32
}

// HitAt places the caret within the directional run under x. This keeps the
// two sides of a direction change distinct even when they share a visual edge.
func HitAt(value string, size, weight float32, x float32) CaretHit {
	if !bidi.NeedsBidi(value) {
		caret := func(offset int) float32 { return Width(value[:offset], size, weight) }
		offset := NearestBoundary(value, x, caret)
		return CaretHit{Offset: offset, X: caret(offset)}
	}
	return hitBidi(value, size, weight, x)
}

func hitBidi(value string, size, weight float32, x float32) CaretHit {
	table := weightIndex(weight)
	b := bidiPool.Get().(*bidiLine)
	b.resolve(value, table)
	px := func(at int) float32 { return float32(at) * size / unitsPerEm }
	left := 0
	for _, run := range b.runs {
		width := units(value[run.Start:run.End], table)
		// A click exactly on a run's edge belongs to the run whose right edge
		// it is, and the first and last runs keep the line's own ends: without
		// that a click on the leftmost or rightmost pixel falls through to the
		// inner caret of the nearest run.
		if x >= px(left) && x <= px(left+width) {
			best, distance := CaretHit{}, float32(-1)
			at := snap(value, run.Start)
			if at < run.Start {
				at = grapheme.Next(value, at)
			}
			for ; at <= run.End; at = grapheme.Next(value, at) {
				pos := px(left + b.runEdge(value, run, at))
				if gap := absf(pos - x); distance < 0 || gap < distance {
					best, distance = CaretHit{Offset: at, X: pos}, gap
				}
				if at == run.End {
					break
				}
			}
			if distance >= 0 {
				b.release()
				return best
			}
			break
		}
		left += width
	}
	hit := b.nearestPrimary(value, size, x)
	b.release()
	return hit
}

// At the exact meeting point of two runs, use the paragraph's usual caret
// affinity. This also handles clicks outside the visible advances.
func (b *bidiLine) nearestPrimary(value string, size, x float32) CaretHit {
	best, bestDistance := CaretHit{}, float32(-1)
	for offset := 0; ; {
		pos := float32(b.edge(value, offset)) * size / unitsPerEm
		d := absf(pos - x)
		if bestDistance < 0 || d < bestDistance {
			best, bestDistance = CaretHit{Offset: offset, X: pos}, d
		}
		if offset >= len(value) {
			break
		}
		offset = grapheme.Next(value, offset)
	}
	return best
}

// NearestBoundary is the grapheme cluster boundary of value whose caret is nearest to
// x, given carets that do not decrease along the line: caret(offset) is the caret at a
// boundary. Of two boundaries as near, the earlier, and of a run of boundaries at one
// caret (characters with no advance), the first. It measures a logarithmic number of
// boundaries, so a click on a long line costs its length times that, not its square.
func NearestBoundary(value string, x float32, caret func(offset int) float32) int {
	if len(value) == 0 || x <= caret(0) {
		return 0
	}
	lo := len(value)
	if caret(lo) >= x {
		// The first boundary whose caret reaches x, found by bisection: hi reaches
		// it, lo does not.
		var hi int
		lo, hi = 0, len(value)
		for {
			mid := snap(value, lo+(hi-lo)/2)
			if mid <= lo {
				if mid = grapheme.Next(value, lo); mid >= hi {
					break
				}
			}
			if caret(mid) >= x {
				hi = mid
			} else {
				lo = mid
			}
		}
		// lo is the boundary before hi: hi when nearer, else lo.
		if caret(hi)-x < x-caret(lo) {
			return hi
		}
	}
	// lo is the answer's caret; the first boundary at it is the answer.
	at := caret(lo)
	for lo > 0 {
		before := grapheme.Prev(value, lo)
		if caret(before) != at {
			break
		}
		lo = before
	}
	return lo
}
