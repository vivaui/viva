package components

import (
	"strconv"
	"strings"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
)

// MarkdownStyle sizes the text of a MarkdownView. The zero value is a 13-point
// body on a 20-point line.
type MarkdownStyle struct {
	// Size is the body text size. Headings are scaled from it, inline code and
	// code blocks are a point smaller.
	Size float32
	// LineHeight is the distance between the lines of body text.
	LineHeight float32
	// MonoFace is the fixed-pitch family of code.
	MonoFace gfx.MonoFace
	// CodeInset keeps this much of the right side of every code block free of
	// text: room for a button the application paints over the block.
	CodeInset float32
	// Highlight, when set, is asked for the colored stretches of every line of a
	// fenced block: language is the block's info string ("bash", "go", ""), line
	// one line of its code. It appends to dst the runs it wants colored, in
	// order, not overlapping, on character boundaries, and returns dst; whatever
	// it leaves out is plain code. It sees one line at a time, so a comment or
	// string that runs over several lines is only colored where the lexer can
	// tell from the line itself. The colors come from MarkdownLook.Code.
	Highlight func(language, line string, dst []MarkdownCodeRun) []MarkdownCodeRun
	// Image, when set, is asked for the bitmap of every image (![alt](destination)),
	// with the destination as written, while the view lays out: the bitmap decides
	// the room the image takes. It is asked on every Layout, and more than once for
	// one image (a table measures its cells before laying them out), so answer from
	// a cache. The view never reads files or the network; an application resolves
	// relative paths and fetches what it allows. A bitmap's pixels are its size in
	// the layout, scaled down to the column and never up. The renderer's texture
	// cache knows a bitmap by gfx.Bitmap.ID and Version, as AvatarData's doc says,
	// so give each bitmap a stable ID and bump Version when its pixels change.
	// Without an ID every paint sends the pixels to the painter again, so a document
	// with images re-uploads them on every frame. An ok of false lays the image out
	// as its alt text, so an application can answer once the image has loaded, lay
	// the document out again and invalidate. The alt text stays part of the
	// document's text, for search and copy, while an image itself has no caret or
	// selection highlight.
	Image func(destination string) (gfx.Bitmap, bool)
}

// MarkdownCodeClass says what a stretch of code is, which MarkdownLook.Code turns
// into a color. The zero class is plain code.
type MarkdownCodeClass uint8

const (
	CodeComment MarkdownCodeClass = iota + 1
	CodeString
	CodeNumber
	CodeKeyword
	CodeFunction
	CodeVariable
	CodeFlag
	CodeKey
	CodeAdded
	CodeRemoved
	// MarkdownCodeClasses is the size of MarkdownLook.Code.
	MarkdownCodeClasses
)

// MarkdownCodeRun is a stretch of one line of code, in bytes, and what it is.
type MarkdownCodeRun struct {
	Start, End int
	Class      MarkdownCodeClass
}

// MarkdownLook colors a MarkdownView.
type MarkdownLook struct {
	Text           gfx.Color // body text, headings and code
	Muted          gfx.Color // quotes, list markers
	Link           gfx.Color
	CodeFill       gfx.Color // behind a fenced block
	InlineCodeFill gfx.Color // behind code inside a line
	Line           gfx.Color // borders, rules and quote bars
	// Code colors the classes of highlighted code, by class; a class left
	// transparent is drawn in Text.
	Code      [MarkdownCodeClasses]gfx.Color
	Selection gfx.Color // behind selected text; transparent paints none
	Radius    float32   // corners of code blocks and tables
}

// MarkdownLookFor derives a look from a theme.
func MarkdownLookFor(theme gfx.Theme) MarkdownLook {
	selection := theme.Primary
	selection.Alpha *= 0.50
	return MarkdownLook{
		Text: theme.Foreground, Muted: theme.MutedForeground, Link: theme.Chart[1],
		CodeFill: theme.Muted, InlineCodeFill: theme.Muted, Line: theme.Border,
		Selection: selection, Radius: theme.Metrics.RadiusControl,
	}
}

// MarkdownSelection is a range of the rendered text of a MarkdownView, in byte
// offsets. Anchor is where the user began and Focus is where the pointer is now;
// either may be ahead of the other. Range clamps and orders them against the
// view's text, so a selection that was made before a message changed still
// copies the part that is there.
type MarkdownSelection struct {
	Anchor, Focus int
}

// Range returns the selected bytes, ordered and clamped to text. Clamping snaps
// both ends back to the start of the rune they point into, because a selection
// kept while a streaming message shortened can land inside a multi-byte
// character; without the snap SelectedText would return invalid UTF-8.
func (s MarkdownSelection) Range(text []byte) (start, end int) {
	start, end = s.Anchor, s.Focus
	if start > end {
		start, end = end, start
	}
	start = snapRune(text, min(max(start, 0), len(text)))
	end = snapRune(text, min(max(end, 0), len(text)))
	return start, end
}

// snapRune moves offset back to the leading byte of the rune it points into.
// Offsets that are already at a boundary, including len(text), are returned as
// they are.
func snapRune(text []byte, offset int) int {
	for offset > 0 && offset < len(text) && text[offset]&0xC0 == 0x80 {
		offset--
	}
	return offset
}

// MarkdownCode is a fenced code block of a laid-out document: where it is, so
// the application can put a copy button on it, and what it holds.
type MarkdownCode struct {
	Bounds gfx.Rect // relative to the origin the document is painted at
	Text   string
}

// MarkdownPaint says where and how to paint a MarkdownView.
type MarkdownPaint struct {
	Look MarkdownLook
	// X and Y are where the top-left corner of the document goes.
	X, Y float32
	// Clip is the visible part of the canvas. Nothing outside it is painted
	// and no link outside it gets a hit region. The zero Rect means everything.
	Clip gfx.Rect
	// Links names the hit regions of links: fragment i of the view is
	// Links.At(Base+i), and MarkdownView.Link(i) says where it points. Zero
	// paints links without hits.
	Links gfx.Scope
	// Base is added to the number of every fragment, so that several views
	// painted in one frame can share a scope: give each the total of the
	// LinkCount of the views before it.
	Base int
	// Selection is highlighted behind its text. The zero range paints nothing.
	Selection MarkdownSelection
}

var _ gfx.TextPlacement = (*MarkdownView)(nil)

// MarkdownView lays out and paints a markdown.Document. It keeps the arrays it
// builds, so laying out the next version of a message that is still being typed
// allocates nothing once the view is warm. The zero value is ready; a view is
// used by one goroutine, and one view serves one document.
//
//	doc.Parse(message)
//	height := view.Layout(canvas, components.MarkdownStyle{Size: 13}, &doc, width)
//	...
//	view.Paint(canvas, components.MarkdownPaint{Look: components.MarkdownLookFor(theme), X: x, Y: y, Links: links})
//
// Layout measures text and is the expensive half: do it when the text or the
// width changed, and call Paint every frame.
type MarkdownView struct {
	pieces     []mdPiece
	runs       []mdCodeRun
	hl         []MarkdownCodeRun
	scratch    []gfx.TextRun
	selScratch []gfx.TextSpan
	boxes      []mdBox
	links      []mdLink
	codes      []MarkdownCode
	images     []mdImage
	anchors    []MarkdownAnchor
	atoms      []mdAtom
	numbers    []byte
	tabs       []byte  // code lines with their tabs spread into blanks
	tabBound   []int32 // where each byte of a spread code line came from
	plain      []byte  // the rendered text, for selection and copying
	spanBase   []int32 // where each span of the current inline block starts in plain
	measure    *gfx.Canvas
	paintX     float32 // where the last Paint put the document
	paintY     float32
	painted    bool
	bars       [markdownMaxQuote + 1]int32
	face       gfx.MonoFace
	height     float32
}

// Height is the height of the laid-out document.
func (v *MarkdownView) Height() float32 { return v.height }

// TextLen is the length in bytes of the rendered text that can be selected.
func (v *MarkdownView) TextLen() int { return len(v.plain) }

// SelectedText returns the rendered text of sel: what a copy of the selection
// puts on the clipboard. Offsets are clamped to the laid-out text, so an empty
// view or a selection outside it returns "".
func (v *MarkdownView) SelectedText(sel MarkdownSelection) string {
	start, end := sel.Range(v.plain)
	return string(v.plain[start:end])
}

// OffsetAt is the text offset nearest a window point, for a pointer that selects
// answer text. It uses the origin and measurements of the last Paint, so it is
// valid from that paint until the next Layout or Paint. Decoration such as list
// markers is skipped; a point in the margin snaps to the nearest character.
func (v *MarkdownView) OffsetAt(point gfx.Point) (offset int, ok bool) {
	if !v.painted || v.measure == nil || len(v.pieces) == 0 {
		return 0, false
	}
	x, y := point.X-v.paintX, point.Y-v.paintY
	best, bestScore := -1, float32(0)
	var previous *mdPiece
	for index := range v.pieces {
		piece := &v.pieces[index]
		if piece.plainLo < 0 {
			continue
		}
		if previous != nil && previous.y == piece.y && previous.plainHi < piece.plainLo && y >= piece.y && y <= piece.y+piece.h {
			start := previous.x + v.caretAdvance(previous, int(previous.plainHi-previous.plainLo))
			end := piece.x + v.caretAdvance(piece, 0)
			if end > start && x >= start && x <= end {
				gap := int(piece.plainLo - previous.plainHi)
				position := int(float32(gap)*(x-start)/(end-start) + 0.5)
				return gfx.GraphemeStart(unsafe.String(unsafe.SliceData(v.plain), len(v.plain)), int(previous.plainHi)+min(max(position, 0), gap)), true
			}
		}
		previous = piece
		dx, dy := float32(0), float32(0)
		if x < piece.x {
			dx = piece.x - x
		} else if x > piece.x+piece.w {
			dx = x - piece.x - piece.w
		}
		if y < piece.y {
			dy = piece.y - y
		} else if y > piece.y+piece.h {
			dy = y - piece.y - piece.h
		}
		score := dy*1_000_000 + dx
		if best < 0 || score < bestScore {
			best, bestScore = index, score
		}
	}
	if best < 0 {
		return 0, false
	}
	piece := &v.pieces[best]
	localX := min(max(x-piece.x, 0), piece.w)
	var local int
	if piece.mono {
		local = v.codeOffsetAt(piece, localX)
	} else {
		local = v.measure.TextOffsetAtStyle(piece.text, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: piece.italic}, localX)
	}
	local = min(max(local, 0), int(piece.plainHi-piece.plainLo))
	return int(piece.plainLo) + local, true
}

// CaretAt returns the window rectangle for a rendered byte offset after Paint.
// It reports geometry only; MarkdownView does not own the selection or editor.
func (v *MarkdownView) CaretAt(offset int) (gfx.Rect, bool) {
	if !v.painted || v.measure == nil || len(v.pieces) == 0 {
		return gfx.Rect{}, false
	}
	value := unsafe.String(unsafe.SliceData(v.plain), len(v.plain))
	offset = gfx.GraphemeStart(value, offset)
	var chosen, before, after *mdPiece
	for index := range v.pieces {
		piece := &v.pieces[index]
		if piece.plainLo < 0 {
			continue
		}
		switch {
		case offset >= int(piece.plainLo) && offset <= int(piece.plainHi):
			if chosen == nil || piece.plainLo == int32(offset) && chosen.plainLo < int32(offset) {
				chosen = piece
			}
		case int(piece.plainHi) < offset:
			if before == nil || piece.plainHi > before.plainHi {
				before = piece
			}
		case int(piece.plainLo) > offset:
			if after == nil || piece.plainLo < after.plainLo {
				after = piece
			}
		}
	}
	if chosen == nil {
		if before != nil && after != nil && before.y == after.y {
			start := before.x + v.caretAdvance(before, int(before.plainHi-before.plainLo))
			end := after.x + v.caretAdvance(after, 0)
			fraction := float32(offset-int(before.plainHi)) / float32(after.plainLo-before.plainHi)
			return gfx.Rect{X: v.paintX + start + (end-start)*fraction, Y: v.paintY + before.y, Width: 1, Height: before.h}, true
		}
		if before == nil && after == nil {
			return gfx.Rect{}, false
		}
		if before == nil || after != nil && int(after.plainLo)-offset <= offset-int(before.plainHi) {
			chosen = after
			offset = int(after.plainLo)
		} else {
			chosen = before
			offset = int(before.plainHi)
		}
	}
	local := min(max(offset-int(chosen.plainLo), 0), int(chosen.plainHi-chosen.plainLo))
	advance := v.caretAdvance(chosen, local)
	return gfx.Rect{X: v.paintX + chosen.x + advance, Y: v.paintY + chosen.y, Width: 1, Height: chosen.h}, true
}

func (v *MarkdownView) caretAdvance(piece *mdPiece, local int) float32 {
	var advance float32
	if piece.mono {
		_, display := v.codeDisplayRange(piece, local, local)
		advance = piece.padX + float32(monoCells(piece.text[:display]))*piece.cell
	} else {
		advance = v.measure.TextCaretStyle(piece.text, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: piece.italic}, local)
	}
	return advance
}

// CaretBounds reports no caret: a MarkdownView paints text, not an input field.
func (v *MarkdownView) CaretBounds() (gfx.Rect, bool) { return gfx.Rect{}, false }

// codeOffsetAt is the source byte offset nearest x inside a fixed-pitch piece.
// The piece's own cell width is used, because inline code is drawn smaller than
// a fenced block. In a fenced piece a tab is one byte in the source but several
// cells on screen; a point inside one snaps to the nearer edge of the tab, never
// to a synthetic byte.
func (v *MarkdownView) codeOffsetAt(piece *mdPiece, x float32) int {
	source := v.plain[piece.plainLo:piece.plainHi]
	if len(source) == 0 || x <= piece.padX {
		return 0
	}
	x -= piece.padX
	if x >= piece.w-2*piece.padX {
		return len(source)
	}
	cell := piece.cell
	if cell <= 0 {
		cell = 1
	}
	column := int(piece.codeCol)
	at, display := 0, float32(0)
	for at < len(source) {
		r, n := utf8.DecodeRune(source[at:])
		cells := monoRuneCells(r)
		if r == '\t' && piece.spread {
			cells = 4 - column%4
		}
		end := display + float32(cells)*cell
		if x < end {
			if x-display < end-x {
				return at
			}
			return at + n
		}
		display, column, at = end, column+cells, at+n
	}
	return len(source)
}

// codeDisplayRange maps byte offsets of a fixed-pitch piece's source range to
// the bytes of its painted text, where tabs are already spaces. A tab is whole
// in one piece, so a source range that touches it covers all of its cells.
func (v *MarkdownView) codeDisplayRange(piece *mdPiece, start, end int) (int, int) {
	source := v.plain[piece.plainLo:piece.plainHi]
	from, to := -1, -1
	if start == 0 {
		from = 0
	}
	if end == 0 {
		to = 0
	}
	column, display, at := int(piece.codeCol), 0, 0
	for at < len(source) {
		if from < 0 && at >= start {
			from = display
		}
		if to < 0 && at >= end {
			to = display
			break
		}
		r, n := utf8.DecodeRune(source[at:])
		if r == '\t' {
			fill := 4 - column%4
			display += fill
			column += fill
		} else {
			display += n
			column += monoRuneCells(r)
		}
		at += n
	}
	if from < 0 {
		from = display
	}
	if to < 0 {
		to = display
	}
	return from, to
}

// Codes lists the fenced code blocks, in reading order.
func (v *MarkdownView) Codes() []MarkdownCode { return v.codes }

// LinkCount is how many link fragments the document has: a link that wraps onto
// two lines is two fragments.
func (v *MarkdownView) LinkCount() int { return len(v.links) }

// Link returns the destination of link fragment i, as numbered by
// MarkdownPaint.Links (less Base), and "" for a number that is not one.
func (v *MarkdownView) Link(i int) string {
	if i < 0 || i >= len(v.links) {
		return ""
	}
	return v.links[i].dest
}

const (
	markdownMaxQuote  = 8
	markdownQuoteStep = 14
	markdownListStep  = 20
	markdownMarkerW   = 24
	markdownInlinePad = 2
	markdownCodePadX  = 10
	markdownCodePadY  = 8
	markdownCellPadX  = 8
	markdownCellPadY  = 5
	markdownMaxLines  = 20000 // the most lines one layout produces
)

type mdColor uint8

const (
	mdColText mdColor = iota
	mdColMuted
	mdColLink
)

// mdCodeRun is a stretch of a highlighted piece: how many bytes and cells.
type mdCodeRun struct {
	bytes, cells int32
	class        MarkdownCodeClass
}

type mdPiece struct {
	runLo, runHi int32 // the fixed-grid runs of the piece, in View.runs
	// plainLo and plainHi are the bytes of View.plain this piece draws, or -1
	// when it is decoration (a list marker) that is not part of the copied text.
	plainLo, plainHi int32
	// codeCol is where a fixed-pitch piece starts in the cells of its source
	// line, needed to expand the tabs in its plain range for painting. spread
	// marks a fenced-code piece, whose text has its tabs already expanded. cell
	// is the piece's cell width and padX is its visual inset: each differs
	// between a fenced block and smaller inline code, and between body text and
	// a heading.
	codeCol    int32
	cell       float32
	padX       float32
	spread     bool
	text       string
	x, y, w, h float32
	size       float32
	weight     float32
	color      mdColor
	mono       bool
	italic     bool
	strike     bool
	link       int32
}

type mdBoxKind uint8

const (
	mdBoxCode mdBoxKind = iota
	mdBoxInlineCode
	mdBoxBar
	mdBoxLine
	mdBoxHead
)

type mdBox struct {
	kind       mdBoxKind
	x, y, w, h float32
}

type mdLink struct {
	x, y, w, h float32
	size       float32
	text, dest string
}

// mdImage is an image laid out at x, y, w by h.
type mdImage struct {
	bitmap     gfx.Bitmap
	x, y, w, h float32
}

type mdAtom struct {
	span, lo, hi int32
	// image marks an image's atom: w by h of bitmap.
	image    bool
	h        float32
	bitmap   gfx.Bitmap
	w, space float32 // width, and the width of the blank that follows it
	brk      bool    // a line break
	glue     bool    // punctuation that may not start a line
}

type mdFont struct {
	size, line, weight float32
	color              mdColor
}

// mdLayout is the state of one Layout call.
type mdLayout struct {
	// column is the widest an image may be laid out, set by inline.
	column float32
	v      *MarkdownView
	canvas *gfx.Canvas
	style  MarkdownStyle
	doc    *markdown.Document
	lines  int
}

// Layout positions the blocks of doc in a column width wide and returns its
// height. Positions are relative to the point the document is painted at.
func (v *MarkdownView) Layout(canvas *gfx.Canvas, style MarkdownStyle, doc *markdown.Document, width float32) float32 {
	if style.Size <= 0 {
		style.Size = 13
	}
	if style.LineHeight <= 0 {
		style.LineHeight = float32(int(style.Size*1.55 + 0.5))
	}
	v.images = v.images[:0]
	v.pieces, v.boxes, v.links, v.codes, v.anchors, v.numbers, v.runs, v.tabs, v.tabBound, v.plain, v.spanBase = v.pieces[:0], v.boxes[:0], v.links[:0], v.codes[:0], v.anchors[:0], v.numbers[:0], v.runs[:0], v.tabs[:0], v.tabBound[:0], v.plain[:0], v.spanBase[:0]
	v.bars = [markdownMaxQuote + 1]int32{}
	v.face = style.MonoFace
	v.measure = canvas
	v.painted = false
	l := mdLayout{v: v, canvas: canvas, style: style, doc: doc}
	width = max(width, 40)

	body := mdFont{size: style.Size, line: style.LineHeight, weight: gfx.WeightRegular, color: mdColText}
	y := float32(0)
	var prev *markdown.Block
	for index := range doc.Blocks {
		if l.lines >= markdownMaxLines {
			break
		}
		b := &doc.Blocks[index]
		y += l.gap(prev, b)
		x0 := float32(b.Quote) * markdownQuoteStep
		w := width - x0
		top := y
		t := body
		if b.Quote > 0 {
			t.color = mdColMuted
		}
		l.blockBreak()
		switch b.Kind {
		case markdown.Paragraph:
			indent := float32(0)
			if b.Level > 0 {
				indent = float32(b.Level-1)*markdownListStep + markdownMarkerW
			}
			l.appendSpans(doc.Spans(b))
			y = l.inline(doc.Spans(b), x0+indent, w-indent, y, t, markdown.AlignLeft)
		case markdown.Heading:
			l.appendSpans(doc.Spans(b))
			y = l.inline(doc.Spans(b), x0, w, y, l.heading(b.Level, t), markdown.AlignLeft)
		case markdown.Item:
			indent := float32(b.Level)*markdownListStep + markdownMarkerW
			l.marker(b, x0+float32(b.Level)*markdownListStep, y, t)
			l.appendSpans(doc.Spans(b))
			y = l.inline(doc.Spans(b), x0+indent, w-indent, y, t, markdown.AlignLeft)
			y = max(y, top+t.line)
		case markdown.Code:
			y = l.code(b, x0, w, y)
		case markdown.Rule:
			v.boxes = append(v.boxes, mdBox{kind: mdBoxLine, x: x0, y: y + 6, w: w, h: 1})
			y += 13
		case markdown.Table:
			y = l.table(b, x0, w, y, t)
		}
		l.quoteBars(b.Quote, top, y)
		v.anchors = append(v.anchors, MarkdownAnchor{Start: b.Start, End: b.End, Top: top, Bottom: y})
		prev = b
	}
	v.height = y
	return y
}

func (l *mdLayout) heading(level int, t mdFont) mdFont {
	scale := float32(1)
	switch level {
	case 1:
		scale = 1.4
	case 2:
		scale = 1.25
	case 3:
		scale = 1.1
	}
	t.size *= scale
	t.line = float32(int(t.line*scale + 0.5))
	t.weight = gfx.WeightSemibold
	if level == 1 {
		t.weight = gfx.WeightBold
	}
	return t
}

// blockBreak separates the text of two blocks with one newline, however much
// space the layout puts between them. A soft wrap adds nothing: it is not in the
// source the user is copying.
func (l *mdLayout) blockBreak() {
	v := l.v
	if len(v.plain) == 0 || v.plain[len(v.plain)-1] == '\n' {
		return
	}
	v.plain = append(v.plain, '\n')
}

// appendSpans adds the visible text of an inline block to the rendered plain
// text and records where each span starts, so every piece laid out from it can
// say which bytes it draws.
func (l *mdLayout) appendSpans(spans []markdown.Span) {
	v := l.v
	v.spanBase = v.spanBase[:0]
	for _, sp := range spans {
		v.spanBase = append(v.spanBase, int32(len(v.plain)))
		v.plain = append(v.plain, sp.Text...)
	}
}

// gap is the space between two blocks.
func (l *mdLayout) gap(prev, cur *markdown.Block) float32 {
	if prev == nil {
		return 0
	}
	lh := l.style.LineHeight
	switch {
	case prev.Kind == markdown.Item && (cur.Kind == markdown.Item || cur.Kind == markdown.Paragraph && cur.Level > 0):
		return float32(int(lh * 0.25))
	case cur.Kind == markdown.Heading:
		return float32(int(lh * 0.75))
	case prev.Kind == markdown.Heading:
		return float32(int(lh * 0.3))
	}
	return float32(int(lh * 0.45))
}

// quoteBars keeps one bar per quote level running through the blocks inside it.
func (l *mdLayout) quoteBars(quote int, top, bottom float32) {
	v := l.v
	for level := 1; level <= markdownMaxQuote; level++ {
		if level > quote {
			v.bars[level] = 0
			continue
		}
		if open := v.bars[level]; open > 0 {
			box := &v.boxes[open-1]
			box.h = bottom - box.y
			continue
		}
		v.boxes = append(v.boxes, mdBox{kind: mdBoxBar, x: float32(level-1)*markdownQuoteStep + 2, y: top, w: 3, h: bottom - top})
		v.bars[level] = int32(len(v.boxes))
	}
}

// marker draws the bullet or number in front of a list item.
func (l *mdLayout) marker(b *markdown.Block, x, y float32, t mdFont) {
	v := l.v
	text := "•"
	if b.Number > 0 {
		start := len(v.numbers)
		v.numbers = strconv.AppendInt(v.numbers, int64(b.Number), 10)
		v.numbers = append(v.numbers, '.')
		text = unsafe.String(&v.numbers[start], len(v.numbers)-start)
	}
	w := l.canvas.TextWidthWeight(text, t.size, gfx.WeightRegular)
	v.pieces = append(v.pieces, mdPiece{
		plainLo: -1, plainHi: -1,
		text: text, x: x + markdownMarkerW - 6 - w, y: y, w: w + 2, h: t.line, size: t.size,
		weight: gfx.WeightRegular, color: mdColMuted, link: -1,
	})
}

// code lays out a fenced block.
func (l *mdLayout) code(b *markdown.Block, x, width, y float32) float32 {
	v, style := l.v, l.style
	size := max(style.Size-1, 9)
	cell, height := l.canvas.MonoCellSize(size, style.MonoFace)
	if cell <= 0 {
		cell = size * 0.6
	}
	cols := max(int((width-2*markdownCodePadX-style.CodeInset)/cell), 4)
	top := y
	y += markdownCodePadY
	text := b.Text
	for lineIndex := 0; ; lineIndex++ {
		source, rest, more := cutLine(text)
		if lineIndex > 0 {
			v.plain = append(v.plain, '\n')
		}
		line, bound := v.spreadTabs(source)
		var marked []MarkdownCodeRun
		if style.Highlight != nil && len(line) <= 4000 {
			v.hl = style.Highlight(b.Info, line, v.hl[:0])
			marked = v.hl
		}
		column := 0
		for start := 0; ; {
			if l.lines >= markdownMaxLines {
				more = false
				break
			}
			end, done := monoBreak(line[start:], cols)
			end += start
			end = codeBreak(line, bound, start, end)
			if end > start {
				chunk := line[start:end]
				plainLo := int32(len(v.plain))
				if bound == nil {
					v.plain = append(v.plain, source[start:end]...)
				} else {
					v.plain = append(v.plain, source[bound[start]:bound[end]]...)
				}
				piece := mdPiece{
					plainLo: plainLo, plainHi: int32(len(v.plain)), codeCol: int32(column), cell: cell, spread: true,
					text: chunk, x: x + markdownCodePadX, y: y, w: float32(monoCells(chunk)) * cell, h: height,
					size: size, weight: gfx.WeightRegular, color: mdColText, mono: true, link: -1,
				}
				piece.runLo = int32(len(v.runs))
				v.appendCodeRuns(line, start, end, marked)
				piece.runHi = int32(len(v.runs))
				v.pieces = append(v.pieces, piece)
				column += monoCells(chunk)
			}
			l.lines++
			y += height
			start = end
			if done || start >= len(line) {
				break
			}
		}
		if !more {
			break
		}
		text = rest
	}
	y += markdownCodePadY
	rect := gfx.Rect{X: x, Y: top, Width: width, Height: y - top}
	v.boxes = append(v.boxes, mdBox{kind: mdBoxCode, x: rect.X, y: rect.Y, w: rect.Width, h: rect.Height})
	v.codes = append(v.codes, MarkdownCode{Bounds: rect, Text: b.Text})
	return y
}

// spreadTabs returns line with each tab replaced by the blanks that reach the
// next stop four cells on, since a fixed-pitch grid has no tab character. bound
// maps every byte boundary of the result to a byte offset of line: a boundary
// inside a spread tab belongs to the tab, and the boundary after it belongs to
// the byte after the tab. A line without tabs returns itself and a linear bound.
func (v *MarkdownView) spreadTabs(line string) (string, []int32) {
	if strings.IndexByte(line, '\t') < 0 {
		// Nothing to spread and the display bytes are the source bytes: no
		// boundary map is needed, so ordinary code lines pay nothing.
		return line, nil
	}
	start, boundStart := len(v.tabs), len(v.tabBound)
	v.tabBound = append(v.tabBound, 0)
	cells := 0
	for index := 0; index < len(line); {
		r, n := utf8.DecodeRuneInString(line[index:])
		if r == '\t' {
			fill := 4 - cells%4
			for i := 0; i < fill; i++ {
				v.tabs = append(v.tabs, ' ')
				cells++
				at := int32(index)
				if i == fill-1 {
					at = int32(index + n)
				}
				v.tabBound = append(v.tabBound, at)
			}
			index += n
			continue
		}
		if r == utf8.RuneError && n == 1 {
			// Invalid UTF-8: keep the source byte. AppendRune would write three
			// bytes for it and the boundary map would no longer match the
			// display text.
			v.tabs = append(v.tabs, line[index])
			cells++
			v.tabBound = append(v.tabBound, int32(index+1))
			index++
			continue
		}
		v.tabs = utf8.AppendRune(v.tabs, r)
		cells += monoRuneCells(r)
		for i := 1; i <= n; i++ {
			v.tabBound = append(v.tabBound, int32(index+i))
		}
		index += n
	}
	bound := v.tabBound[boundStart:]
	if len(v.tabs) == start {
		// An empty source line (a blank line inside a fence) produced no bytes;
		// &v.tabs[start] would be one past the buffer.
		return "", bound
	}
	return unsafe.String(&v.tabs[start], len(v.tabs)-start), bound
}

// codeBreak moves a chunk boundary that fell inside a spread tab out of it, so
// every piece owns whole tabs. A tab that starts the chunk is kept whole; one
// that starts later moves to the next chunk.
func codeBreak(line string, bound []int32, start, end int) int {
	if bound == nil || end <= start || end >= len(line) {
		return end
	}
	tabStart, tabEnd, inside := tabRun(bound, end)
	if !inside {
		return end
	}
	if tabStart > start {
		return tabStart
	}
	if tabEnd > start {
		return tabEnd
	}
	return end
}

// tabRun is the run of boundaries in bound that name the same tab, and whether
// at is strictly inside it. The boundary at a tab's start is not inside.
func tabRun(bound []int32, at int) (start, end int, inside bool) {
	if at <= 0 || at >= len(bound) {
		return at, at, false
	}
	source := bound[at]
	start, end = at, at
	for start > 0 && bound[start-1] == source {
		start--
	}
	for end < len(bound) && bound[end] == source {
		end++
	}
	return start, end, start < at && at < end
}

// appendCodeRuns records fixed-grid runs covering line[from:to], with the
// stretches that marked colors and the plain code between them. Each character
// that is not ASCII is a stretch of its own, as LabelRuns wants.
func (v *MarkdownView) appendCodeRuns(line string, from, to int, marked []MarkdownCodeRun) {
	at := from
	emit := func(start, end int, class MarkdownCodeClass) {
		for start < end {
			stop := start
			for stop < end && line[stop] < utf8.RuneSelf {
				stop++
			}
			if stop > start {
				v.runs = append(v.runs, mdCodeRun{bytes: int32(stop - start), cells: int32(stop - start), class: class})
				start = stop
				continue
			}
			r, n := utf8.DecodeRuneInString(line[start:end])
			v.runs = append(v.runs, mdCodeRun{bytes: int32(n), cells: int32(monoRuneCells(r)), class: class})
			start += n
		}
	}
	for _, run := range marked {
		start, end := max(run.Start, at), min(run.End, to)
		if end <= start {
			continue
		}
		emit(at, start, 0)
		emit(start, end, run.Class)
		at = end
	}
	emit(at, to, 0)
}

func cutLine(text string) (line, rest string, more bool) {
	for i := 0; i < len(text); i++ {
		if text[i] == '\n' {
			return trimCR(text[:i]), text[i+1:], true
		}
	}
	return trimCR(text), "", false
}

func trimCR(s string) string {
	if len(s) > 0 && s[len(s)-1] == '\r' {
		return s[:len(s)-1]
	}
	return s
}

// monoCells is how many cells of a fixed-pitch grid s takes: two for the wide scripts.
func monoCells(s string) int {
	cells := 0
	for _, r := range s {
		cells += monoRuneCells(r)
	}
	return cells
}

func monoRuneCells(r rune) int {
	switch {
	case r < 0x1100:
		return 1
	case r >= 0x1100 && r <= 0x115F, r >= 0x2E80 && r <= 0xA4CF, r >= 0xAC00 && r <= 0xD7A3,
		r >= 0xF900 && r <= 0xFAFF, r >= 0xFE30 && r <= 0xFE6F, r >= 0xFF00 && r <= 0xFF60,
		r >= 0xFFE0 && r <= 0xFFE6, r >= 0x1F300 && r <= 0x1FAFF, r >= 0x20000 && r <= 0x3FFFD:
		return 2
	}
	return 1
}

// monoBreak is where to cut s so that at most cols cells remain in the first part.
func monoBreak(s string, cols int) (end int, done bool) {
	used := 0
	for index, r := range s {
		width := monoRuneCells(r)
		if used+width > cols && used > 0 {
			return index, false
		}
		used += width
	}
	return len(s), true
}

// table lays out a pipe table: columns as wide as their text, or as wide as
// the space allows, cells that wrap.
func (l *mdLayout) table(b *markdown.Block, x, width, y float32, t mdFont) float32 {
	v, doc := l.v, l.doc
	cells := doc.Cells(b)
	columns := b.Columns
	if columns < 1 || len(cells) < columns {
		return y
	}
	rows := len(cells) / columns
	var natural, widths [24]float32
	for c := 0; c < columns; c++ {
		natural[c] = 2 * markdownCellPadX
	}
	head := t
	head.weight = gfx.WeightSemibold
	for i, cell := range cells[:rows*columns] {
		style := t
		if i < columns {
			style = head
		}
		c := i % columns
		natural[c] = max(natural[c], l.natural(doc.CellSpans(cell), style)+2*markdownCellPadX)
	}
	var total float32
	for c := 0; c < columns; c++ {
		total += natural[c]
	}
	if total <= width {
		widths = natural
	} else {
		remaining, free := width, columns
		var fixed [24]bool
		for free > 0 {
			share := remaining / float32(free)
			changed := false
			for c := 0; c < columns; c++ {
				if !fixed[c] && natural[c] <= share {
					widths[c], fixed[c] = natural[c], true
					remaining -= natural[c]
					free--
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		for c := 0; c < columns; c++ {
			if !fixed[c] {
				widths[c] = max(remaining/float32(free), 4*markdownCellPadX)
			}
		}
	}
	var tableW float32
	for c := 0; c < columns; c++ {
		tableW += widths[c]
	}
	for r := 0; r < rows; r++ {
		if l.lines >= markdownMaxLines {
			rows = r
			break
		}
		if r > 0 && len(v.plain) > 0 && v.plain[len(v.plain)-1] != '\n' {
			v.plain = append(v.plain, '\n')
		}
		rowTop := y
		rowBottom := y + 2*markdownCellPadY + t.line
		cx := x
		for c := 0; c < columns; c++ {
			cell := cells[r*columns+c]
			style := t
			if r == 0 {
				style = head
			}
			if c > 0 {
				v.plain = append(v.plain, ' ')
			}
			l.appendSpans(doc.CellSpans(cell))
			end := l.inline(doc.CellSpans(cell), cx+markdownCellPadX, widths[c]-2*markdownCellPadX, rowTop+markdownCellPadY, style, cell.Align)
			rowBottom = max(rowBottom, end+markdownCellPadY)
			cx += widths[c]
		}
		y = rowBottom
		if r == 0 {
			v.boxes = append(v.boxes, mdBox{kind: mdBoxHead, x: x, y: rowTop, w: tableW, h: rowBottom - rowTop})
		}
		v.boxes = append(v.boxes, mdBox{kind: mdBoxLine, x: x, y: y - 1, w: tableW, h: 1})
	}
	return y
}

// natural is the width the spans take on one line per Newline.
func (l *mdLayout) natural(spans []markdown.Span, t mdFont) float32 {
	l.atomize(spans, t)
	var best, line float32
	for _, a := range l.v.atoms {
		if a.brk {
			best, line = max(best, line), 0
			continue
		}
		line += a.w + a.space
	}
	return max(best, line)
}

// spanWeight is the weight of text with a style, in a context of weight base.
func spanWeight(base float32, style markdown.Style) float32 {
	switch {
	case style&markdown.Bold != 0:
		if base >= gfx.WeightSemibold {
			return gfx.WeightBold
		}
		return gfx.WeightSemibold
	}
	return base
}

func isBreakRune(r rune) bool {
	switch {
	case r >= 0x2E80 && r <= 0x9FFF, r >= 0xAC00 && r <= 0xD7AF, r >= 0xF900 && r <= 0xFAFF, r >= 0xFF00 && r <= 0xFFEF:
		return true
	}
	return false
}

// closesLine reports whether r is punctuation that may not start a line.
func closesLine(r rune) bool {
	switch r {
	case '，', '。', '、', '；', '：', '！', '？', '）', '」', '』', '》', '】', '…', '”', '’', ',', '.', ';', ':', '!', '?', ')', ']', '}':
		return true
	}
	return false
}

// atomize cuts the spans into the pieces a line may break between: words, single
// ideographs, and the code that sits between blanks.
func (l *mdLayout) atomize(spans []markdown.Span, t mdFont) {
	atoms := l.v.atoms[:0]
	monoSize := max(t.size-1, 9)
	cell, _ := l.canvas.MonoCellSize(monoSize, l.style.MonoFace)
	// A blank is as wide as one of the regular face whatever its span's weight, so that
	// emphasis does not move the words around it. An italic span's blank is the italic face's.
	space := l.canvas.TextWidthWeight(" ", t.size, gfx.WeightRegular)
	italicSpace := l.canvas.TextWidthStyle(" ", gfx.TextStyle{Size: t.size, Weight: gfx.WeightRegular, Italic: true})
	var pending float32
	for si, sp := range spans {
		if sp.Text == markdown.Newline {
			atoms = append(atoms, mdAtom{span: int32(si), brk: true})
			pending = 0
			continue
		}
		if sp.Image != "" && l.style.Image != nil {
			if bitmap, ok := l.style.Image(sp.Image); ok && bitmap.Width > 0 && bitmap.Height > 0 {
				w, h := float32(bitmap.Width), float32(bitmap.Height)
				if column := l.column; column > 0 && w > column {
					w, h = column, h*column/w
				}
				if pending > 0 && len(atoms) > 0 && !atoms[len(atoms)-1].brk {
					atoms[len(atoms)-1].space = pending
				}
				pending = 0
				atoms = append(atoms, mdAtom{span: int32(si), image: true, w: w, h: h, bitmap: bitmap})
				continue
			}
		}
		mono := sp.Style&markdown.Mono != 0
		italic := sp.Style&markdown.Italic != 0
		size, weight := t.size, spanWeight(t.weight, sp.Style)
		if mono {
			size = monoSize
		}
		text := sp.Text
		for i := 0; i < len(text); {
			r, n := utf8.DecodeRuneInString(text[i:])
			if r == ' ' || r == '\t' {
				if mono {
					pending += cell
				} else if italic {
					pending = italicSpace
				} else {
					pending = space
				}
				i += n
				continue
			}
			j := i + n
			switch {
			case mono:
				for j < len(text) && text[j] != ' ' && text[j] != '\t' {
					j++
				}
			case isBreakRune(r):
				for j < len(text) {
					next, w := utf8.DecodeRuneInString(text[j:])
					if !closesLine(next) {
						break
					}
					j += w
				}
			default:
				for j < len(text) {
					next, w := utf8.DecodeRuneInString(text[j:])
					if next == ' ' || next == '\t' || isBreakRune(next) {
						break
					}
					j += w
				}
			}
			glue := pending == 0 && len(atoms) > 0 && !atoms[len(atoms)-1].brk && closesLine(r)
			if pending > 0 && len(atoms) > 0 && !atoms[len(atoms)-1].brk {
				atoms[len(atoms)-1].space = pending
			}
			pending = 0
			var w float32
			if mono {
				w = float32(monoCells(text[i:j]))*cell + 2*markdownInlinePad
			} else if italic {
				w = l.canvas.TextWidthStyle(text[i:j], gfx.TextStyle{Size: size, Weight: weight, Italic: true})
			} else {
				w = l.canvas.TextWidthWeight(text[i:j], size, weight)
			}
			atoms = append(atoms, mdAtom{span: int32(si), lo: int32(i), hi: int32(j), w: w, glue: glue})
			i = j
		}
	}
	l.v.atoms = atoms
}

// mdLine is the line being filled by inline.
type mdLine struct {
	l      *mdLayout
	spans  []markdown.Span
	t      mdFont
	align  markdown.Align
	x0     float32 // where lines start
	width  float32
	y      float32 // top of the current line
	x      float32 // cursor, relative to x0
	w      float32 // end of the last atom, relative to x0
	has    bool
	piece  int // the open piece, or -1
	lo     int32
	span   int32
	pieceP int // first piece, box, link and image of the line, for alignment
	pieceB int
	pieceL int
	pieceI int
	// tall is the height of the tallest image on the line.
	tall float32
	// grew reports that the open piece took a second atom, so its width is a
	// running sum that closePiece must settle.
	grew bool
	// measured* remembers candidateWidth's complete-piece measurement so that
	// place can commit the same fragment without measuring it again.
	measuredSpan int32
	measuredLo   int32
	measuredHi   int32
	measuredEnd  float32
}

// inline lays spans out as wrapped lines from y and returns where the text ends.
func (l *mdLayout) inline(spans []markdown.Span, x, width, y float32, t mdFont, align markdown.Align) float32 {
	if len(spans) == 0 {
		return y
	}
	l.column = width
	l.atomize(spans, t)
	l.column = 0
	m := mdLine{l: l, spans: spans, t: t, align: align, x0: x, width: width, y: y, piece: -1}
	m.reset()
	for _, a := range l.v.atoms {
		if l.lines >= markdownMaxLines {
			break
		}
		if a.brk {
			m.endLine()
			continue
		}
		// Most atoms sit well inside the line. They fit without consulting the
		// shaped width, which is what keeps filling a line from costing one
		// shaping per atom.
		if !a.glue && a.w <= m.width && m.x+a.w <= m.width-2*m.t.size {
			m.place(a)
			continue
		}
		a, place := m.fit(a)
		if place {
			m.place(a)
		}
	}
	m.endLine()
	return m.y
}

// fit decides which atoms stay on the current line. An ordinary token that
// does not fit moves whole to the next line. A token wider than an empty line
// is split first; if even one decorated character does not fit, it is placed
// as the unavoidable minimum so the loop always makes progress.
func (m *mdLine) fit(a mdAtom) (mdAtom, bool) {
	if a.brk {
		return a, false
	}
	if a.glue && m.has && m.candidateWidth(a) > m.width {
		// Closing punctuation may not start a line. Move the preceding piece
		// with it when the pair fits; otherwise move the shortest suffix that
		// can carry it. When neither works, nothing on this line can hold the
		// punctuation, so it starts the next one: painting it past the column,
		// where the clip cuts it in silence, is worse.
		if !m.rewindForGlue(a) && !m.splitForGlue(a) {
			m.endLine()
		}
	}
	if m.has && m.candidateWidth(a) > m.width {
		m.endLine()
	}
	if a.w <= m.width {
		return a, true
	}
	if !a.glue && m.exactWidth(a) > m.width {
		return m.split(a), true
	}
	if m.exactWidth(a) > m.width {
		// Punctuation wider than a line cannot use the ordinary split path,
		// but it still has to consume one character or fit would loop.
		return m.split(a), true
	}
	return a, true
}

func (m *mdLine) reset() {
	v := m.l.v
	m.x, m.w, m.has, m.piece = 0, 0, false, -1
	m.pieceP, m.pieceB, m.pieceL, m.pieceI = len(v.pieces), len(v.boxes), len(v.links), len(v.images)
	m.tall = 0
	m.measuredSpan, m.measuredLo, m.measuredHi, m.measuredEnd = -1, 0, 0, 0
	m.grew = false
}

func (m *mdLine) spanWidth(sp markdown.Span, text string) float32 {
	size, weight := m.t.size, spanWeight(m.t.weight, sp.Style)
	if sp.Style&markdown.Mono == 0 {
		if sp.Style&markdown.Italic != 0 {
			return m.l.canvas.TextWidthStyle(text, gfx.TextStyle{Size: size, Weight: weight, Italic: true})
		}
		return m.l.canvas.TextWidthWeight(text, size, weight)
	}
	size = max(size-1, 9)
	cell, _ := m.l.canvas.MonoCellSize(size, m.l.style.MonoFace)
	if cell <= 0 {
		cell = size * 0.6
	}
	return float32(monoCells(text)) * cell
}

func (m *mdLine) spanPad(sp markdown.Span) float32 {
	if sp.Style&markdown.Mono != 0 {
		return markdownInlinePad
	}
	return 0
}

func (m *mdLine) exactWidth(a mdAtom) float32 {
	if a.image {
		return a.w
	}
	if a.hi <= a.lo {
		return 0
	}
	sp := m.spans[a.span]
	return m.spanWidth(sp, sp.Text[a.lo:a.hi]) + 2*m.spanPad(sp)
}

// candidateWidth is where atom a would end if it continued the current line.
// A piece is remeasured as the complete string it will paint, so shaped text
// cannot change the right edge after the line has been committed. Shaping the
// growing piece once per atom would make a line quadratic in its length, so
// only an atom that may cross the edge is measured that way: further inside the
// line the running width cannot decide the break.
func (m *mdLine) candidateWidth(a mdAtom) float32 {
	sp := m.spans[a.span]
	if m.piece >= 0 && m.span == a.span {
		p := &m.l.v.pieces[m.piece]
		// Either side of the edge is decided as well by the running width; only
		// the band around it needs the shaped one.
		if approx := m.x + a.w; approx <= m.width-2*m.t.size || approx >= m.width+2*m.t.size {
			m.measuredSpan = -1
			return approx
		}
		end := p.x - m.x0 + m.spanWidth(sp, sp.Text[m.lo:a.hi]) + 2*p.padX
		m.measuredSpan, m.measuredLo, m.measuredHi, m.measuredEnd = a.span, m.lo, a.hi, end
		return end
	}
	m.measuredSpan = -1
	return m.x + a.w
}

// joinedWidth measures prefix and atom a as they will be painted. Text from one
// span shares a piece, so its complete string must be shaped together.
func (m *mdLine) joinedWidth(a mdAtom, prefix string) float32 {
	if m.piece >= 0 && m.span == a.span {
		sp := m.spans[a.span]
		p := &m.l.v.pieces[m.piece]
		// prefix is a suffix ending at a.lo, so measure the original text in place.
		start := a.lo - int32(len(prefix))
		return m.spanWidth(sp, sp.Text[start:a.hi]) + 2*p.padX
	}
	prefixSpan := m.spans[m.span]
	return m.spanWidth(prefixSpan, prefix) + 2*m.spanPad(prefixSpan) + m.exactWidth(a)
}

// rewindForGlue moves the open piece and the punctuation after it to a new
// line when their combined width fits. The piece has not been closed, so it
// has no background or link yet to remove.
func (m *mdLine) rewindForGlue(a mdAtom) bool {
	if m.piece < 0 {
		return false
	}
	m.settlePiece()
	v := m.l.v
	prev := v.pieces[m.piece]
	start := prev.x - m.x0
	if start <= 0 || m.joinedWidth(a, prev.text) > m.width {
		return false
	}
	span, lo := m.span, m.lo
	v.pieces = v.pieces[:m.piece]
	m.piece = -1
	m.x, m.w = start, start
	m.measuredSpan = -1
	m.endLine()
	prev.x, prev.y = m.x0, m.y
	v.pieces = append(v.pieces, prev)
	m.piece, m.span, m.lo = len(v.pieces)-1, span, lo
	m.x = prev.w
	m.w = m.x
	m.has = true
	return true
}

// splitForGlue is the fallback when the open piece is too long to move whole.
// It leaves the longest fitting prefix on the current line and moves the
// shortest suffix that can carry the punctuation to the next one. The piece
// is still open, so there is no background or link to duplicate.
func (m *mdLine) splitForGlue(a mdAtom) bool {
	if m.piece < 0 {
		return false
	}
	m.settlePiece()
	v := m.l.v
	prev := v.pieces[m.piece]
	sp := m.spans[m.span]
	if len(prev.text) == 0 {
		return false
	}
	cut := len(prev.text)
	for cut > 0 {
		_, size := utf8.DecodeLastRuneInString(prev.text[:cut])
		cut -= size
		if cut == 0 {
			break
		}
		suffix := prev.text[cut:]
		if m.joinedWidth(a, suffix) <= m.width {
			prefix := prev.text[:cut]
			p := &v.pieces[m.piece]
			p.text = prefix
			p.plainHi = p.plainLo + int32(len(prefix))
			p.w = m.spanWidth(sp, prefix) + 2*p.padX
			m.x = p.x - m.x0 + p.w
			m.w = m.x
			m.closePiece()
			m.endLine()

			next := prev
			next.text = suffix
			next.plainLo = prev.plainLo + int32(len(prefix))
			next.plainHi = prev.plainHi
			next.x, next.y = m.x0, m.y
			next.w = m.spanWidth(sp, suffix) + 2*next.padX
			v.pieces = append(v.pieces, next)
			m.piece, m.lo = len(v.pieces)-1, m.lo+int32(cut)
			m.x, m.w, m.has = next.w, next.w, true
			return true
		}
	}
	return false
}

// split places the leading chunks of an atom wider than a whole line, each on a
// line of its own, and returns the rest.
func (m *mdLine) split(a mdAtom) mdAtom {
	sp := m.spans[a.span]
	pad := m.spanPad(sp)
	m.endLine()
	for m.exactWidth(a) > m.width {
		text := sp.Text[a.lo:a.hi]
		// The longest prefix that fits, by bisection on rune boundaries.
		lo, hi := 0, len(text)
		for hi-lo > 1 {
			mid := (lo + hi) / 2
			for mid > lo && !utf8.RuneStart(text[mid]) {
				mid--
			}
			if mid <= lo {
				break
			}
			if m.spanWidth(sp, text[:mid])+2*pad <= m.width {
				lo = mid
			} else {
				hi = mid
			}
		}
		if lo == 0 {
			// No whole character fits. Consume one anyway so an extremely
			// narrow column makes progress; clipping is preferable to a loop.
			_, lo = utf8.DecodeRuneInString(text)
		}
		chunk := a
		chunk.hi = a.lo + int32(lo)
		chunk.w, chunk.space = m.spanWidth(sp, text[:lo])+2*pad, 0
		m.place(chunk)
		m.endLine()
		a.lo += int32(lo)
		a.w = m.spanWidth(sp, sp.Text[a.lo:a.hi]) + 2*pad
	}
	return a
}

// place puts an atom at the cursor, continuing the open piece when it is more
// of the same span.
func (m *mdLine) place(a mdAtom) {
	l := m.l
	v := l.v
	sp := m.spans[a.span]
	mono := sp.Style&markdown.Mono != 0
	if a.image {
		m.closePiece()
		x := m.x0 + m.x
		v.images = append(v.images, mdImage{bitmap: a.bitmap, x: x, y: m.y, w: a.w, h: a.h})
		if sp.Link != "" {
			v.links = append(v.links, mdLink{x: x, y: m.y, w: a.w, h: a.h, size: m.t.size, text: sp.Text, dest: sp.Link})
		}
		m.x += a.w
		m.w = m.x
		m.x += a.space
		m.has = true
		m.tall = max(m.tall, a.h)
		return
	}
	if a.hi <= a.lo {
		return
	}
	base := v.spanBase[a.span]
	if m.piece >= 0 && m.span == a.span {
		p := &v.pieces[m.piece]
		p.text = sp.Text[m.lo:a.hi]
		p.plainHi = base + int32(a.hi)
		if m.measuredSpan == a.span && m.measuredLo == m.lo && m.measuredHi == a.hi {
			p.w = m.measuredEnd - (p.x - m.x0)
			m.grew = false
		} else {
			// Growing the open piece costs no shaping: the cursor already holds
			// what the piece covers, so its width is that plus the atom. Only
			// shaping that is not a sum of its atoms can move the result, and
			// settlePiece corrects that once, when the piece stops growing.
			p.w = m.x - (p.x - m.x0) + a.w
			m.grew = true
		}
		m.x = p.x - m.x0 + p.w
		m.w = m.x
		m.x += a.space
		m.has = true
		return
	}
	m.closePiece()
	size, weight, color := m.t.size, spanWeight(m.t.weight, sp.Style), m.t.color
	var cell, padX float32
	if mono {
		size = max(size-1, 9)
		padX = markdownInlinePad
		cell, _ = l.canvas.MonoCellSize(size, l.style.MonoFace)
		if cell <= 0 {
			cell = size * 0.6
		}
	}
	if sp.Link != "" {
		color = mdColLink
	}
	v.pieces = append(v.pieces, mdPiece{
		plainLo: base + int32(a.lo), plainHi: base + int32(a.hi), cell: cell, padX: padX,
		text: sp.Text[a.lo:a.hi], x: m.x0 + m.x, y: m.y, w: a.w, h: m.t.line,
		size: size, weight: weight, color: color, mono: mono, italic: sp.Style&markdown.Italic != 0, strike: sp.Style&markdown.Strike != 0, link: -1,
	})
	m.piece, m.span, m.lo = len(v.pieces)-1, a.span, a.lo
	m.measuredSpan, m.grew = -1, false
	p := &v.pieces[m.piece]
	m.x = p.x - m.x0 + p.w
	m.w = m.x
	m.x += a.space
	m.has = true
}

// settlePiece makes the open piece's width exact, which place leaves running
// while the piece grows. Only the open piece is last on its line, so the
// correction moves nothing that is already placed.
func (m *mdLine) settlePiece() {
	if m.piece < 0 || !m.grew {
		return
	}
	v := m.l.v
	p := &v.pieces[m.piece]
	sp := m.spans[m.span]
	exact := m.spanWidth(sp, p.text) + 2*p.padX
	m.grew = false
	if delta := exact - p.w; delta != 0 {
		p.w = exact
		m.x += delta
		m.w += delta
	}
}

// closePiece finishes the open piece: the box behind inline code, the region of a link.
func (m *mdLine) closePiece() {
	if m.piece < 0 {
		return
	}
	m.settlePiece()
	v := m.l.v
	p := &v.pieces[m.piece]
	sp := m.spans[m.span]
	if p.mono {
		if p.runLo == p.runHi && p.text != "" {
			p.runLo = int32(len(v.runs))
			v.appendCodeRuns(p.text, 0, len(p.text), nil)
			p.runHi = int32(len(v.runs))
		}
		h := p.size * 1.5
		v.boxes = append(v.boxes, mdBox{kind: mdBoxInlineCode, x: p.x, y: p.y + (p.h-h)/2, w: p.w, h: h})
	}
	if sp.Link != "" {
		v.links = append(v.links, mdLink{
			x: p.x + p.padX, y: p.y, w: p.w - 2*p.padX, h: p.h,
			size: p.size, text: p.text, dest: sp.Link,
		})
		p.link = int32(len(v.links) - 1)
	}
	m.piece = -1
}

// endLine finishes the current line and starts the next one below it.
func (m *mdLine) endLine() {
	if !m.has {
		return
	}
	m.closePiece()
	v := m.l.v
	if m.align != markdown.AlignLeft {
		shift := m.width - m.w
		if m.align == markdown.AlignCenter {
			shift /= 2
		}
		if shift > 0 {
			for i := m.pieceP; i < len(v.pieces); i++ {
				v.pieces[i].x += shift
			}
			for i := m.pieceB; i < len(v.boxes); i++ {
				v.boxes[i].x += shift
			}
			for i := m.pieceL; i < len(v.links); i++ {
				v.links[i].x += shift
			}
			for i := m.pieceI; i < len(v.images); i++ {
				v.images[i].x += shift
			}
		}
	}
	// A line with an image is as tall as the tallest image, with the text along its
	// top and a little air below the pixels so that stacked images do not touch.
	m.y += max(m.t.line, m.tall+4)
	m.l.lines++
	m.reset()
}

// Paint draws the laid-out document.
func (v *MarkdownView) Paint(canvas *gfx.Canvas, p MarkdownPaint) {
	v.paintX, v.paintY, v.painted = p.X, p.Y, true
	look := p.Look
	clipped := p.Clip.Height > 0
	visible := func(y, h float32) bool {
		return !clipped || (y+h >= p.Clip.Y && y <= p.Clip.Y+p.Clip.Height)
	}
	if clipped {
		canvas.Clip(gfx.Rect{
			X: p.Clip.X, Y: p.Clip.Y, Width: p.Clip.Width, Height: p.Clip.Height,
		})
		defer canvas.Unclip()
	}
	for _, box := range v.boxes {
		rect := gfx.Rect{X: p.X + box.x, Y: p.Y + box.y, Width: box.w, Height: box.h}
		if !visible(rect.Y, rect.Height) {
			continue
		}
		switch box.kind {
		case mdBoxCode:
			canvas.RoundedBorder(rect, look.Radius, 1, look.Line, look.CodeFill)
		case mdBoxInlineCode:
			canvas.Rounded(rect, 4, look.InlineCodeFill)
		case mdBoxBar:
			canvas.Rounded(rect, 1.5, look.Line)
		case mdBoxLine:
			canvas.Fill(rect, look.Line)
		case mdBoxHead:
			canvas.Rounded(rect, 4, look.InlineCodeFill)
		}
	}
	for _, image := range v.images {
		rect := gfx.Rect{X: p.X + image.x, Y: p.Y + image.y, Width: image.w, Height: image.h}
		if visible(rect.Y, rect.Height) {
			canvas.DrawBitmap(image.bitmap, rect)
		}
	}
	start, end := p.Selection.Range(v.plain)
	if start < end && look.Selection.Alpha > 0 {
		for index := range v.pieces {
			piece := &v.pieces[index]
			if piece.plainLo < 0 {
				continue
			}
			lo, hi := max(int(piece.plainLo), start), min(int(piece.plainHi), end)
			if lo >= hi || !visible(p.Y+piece.y, piece.h) {
				continue
			}
			if piece.mono {
				from, to := lo-int(piece.plainLo), hi-int(piece.plainLo)
				if piece.spread {
					from, to = v.codeDisplayRange(piece, from, to)
				}
				if to > from {
					cell := piece.cell
					if cell <= 0 {
						cell = 1
					}
					x := p.X + piece.x + piece.padX + float32(monoCells(piece.text[:from]))*cell
					width := float32(monoCells(piece.text[from:to])) * cell
					canvas.Fill(gfx.Rect{X: x, Y: p.Y + piece.y, Width: width, Height: piece.h}, look.Selection)
				}
				continue
			}
			v.selScratch = canvas.TextSelectionStyle(v.selScratch[:0], piece.text, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: piece.italic},
				lo-int(piece.plainLo), hi-int(piece.plainLo))
			for _, span := range v.selScratch {
				canvas.Fill(gfx.Rect{X: p.X + piece.x + span.X, Y: p.Y + piece.y, Width: span.Width, Height: piece.h}, look.Selection)
			}
		}
	}
	for index := range v.pieces {
		piece := &v.pieces[index]
		rect := gfx.Rect{
			X: p.X + piece.x + piece.padX, Y: p.Y + piece.y,
			Width: piece.w - 2*piece.padX, Height: piece.h,
		}
		if !visible(rect.Y, rect.Height) {
			continue
		}
		color := look.Text
		switch piece.color {
		case mdColMuted:
			color = look.Muted
		case mdColLink:
			color = look.Link
		}
		if piece.runHi > piece.runLo {
			v.scratch = v.scratch[:0]
			column := int32(0)
			for _, run := range v.runs[piece.runLo:piece.runHi] {
				stretch := color
				if run.class > 0 && int(run.class) < len(look.Code) && look.Code[run.class].Alpha > 0 {
					stretch = look.Code[run.class]
				}
				v.scratch = append(v.scratch, gfx.TextRun{
					Column: column, Bytes: run.bytes, Cells: run.cells, Color: stretch, Weight: piece.weight,
				})
				column += run.cells
			}
			canvas.LabelRuns(piece.text, rect, gfx.TextStyle{
				Size: piece.size, Weight: piece.weight, Color: color, Mono: true, MonoFace: v.face,
			}, piece.cell, v.scratch)
		} else {
			canvas.Label(piece.text, rect, gfx.TextStyle{Size: piece.size, Weight: piece.weight, Italic: piece.italic, Color: color, Mono: piece.mono, MonoFace: v.face})
		}
		if piece.strike {
			canvas.Fill(gfx.Rect{X: rect.X, Y: rect.Y + rect.Height/2, Width: rect.Width, Height: 1}, color)
		}
	}
	for index := range v.links {
		link := &v.links[index]
		rect := gfx.Rect{X: p.X + link.x, Y: p.Y + link.y, Width: link.w, Height: link.h}
		if !visible(rect.Y, rect.Height) || p.Links.IsZero() {
			continue
		}
		id := p.Links.At(p.Base + index)
		canvas.HitWithOptions(id, rect, link.text, gfx.HitOptions{Role: gfx.RoleLink})
		if canvas.Hovered() == id {
			canvas.Fill(gfx.Rect{X: rect.X, Y: rect.Y + rect.Height/2 + link.size*0.6, Width: rect.Width, Height: 1}, look.Link)
		}
	}
}
