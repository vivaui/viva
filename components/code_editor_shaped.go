package components

import (
	"cmp"
	"slices"
	"sort"
	"strings"

	"github.com/vivaui/viva/gfx"
)

type codeShaping struct {
	canvas     *gfx.Canvas
	value      string
	style      gfx.TextStyle
	cell       float32
	generation uint32
	cols       int
	tabWidth   int
	wrap       bool
	rows       map[int32]codeShapedRow
	// limit is the most rows the cache keeps: the visible rows of the last
	// paint, and never fewer than the old fixed bound. A viewport taller than
	// that bound used to clear the cache in the middle of the paint, so every
	// row missed again on the next frame.
	limit    int
	hit      gfx.TextHit
	hitRow   int32
	hitValid bool
	styles   []codeShapedStyle
}

type codeShapedRow struct {
	value  string
	carets []gfx.FittedMonoCaret
}

func (editor *CodeEditor) configureShaping(canvas *gfx.Canvas, snapshot CodeEditorSnapshot, size, cell float32) {
	if !snapshot.ShapedBidi || len(snapshot.Representations) != 0 || snapshot.ShowWhitespace ||
		snapshot.ShowIndentGuides || snapshot.ShowEOL || snapshot.Decorations != nil {
		editor.shaping = nil
		return
	}
	style := gfx.TextStyle{Size: size, Mono: true, MonoFace: snapshot.MonoFace}
	generation := canvas.FontGeneration()
	s := editor.shaping
	if s == nil || s.canvas != canvas || s.value != editor.text || s.style != style || s.cell != cell || s.generation != generation ||
		s.cols != editor.cols || s.tabWidth != editor.tabWidth || s.wrap != editor.wrap {
		s = &codeShaping{canvas: canvas, value: editor.text, style: style, cell: cell, generation: generation,
			cols: editor.cols, tabWidth: editor.tabWidth, wrap: editor.wrap, rows: make(map[int32]codeShapedRow)}
		editor.shaping = s
	}
	limit := 64
	if visible := int(editor.content.Height/editor.lineHeight) + 2; visible > limit {
		limit = visible
	}
	s.limit = limit
	if len(s.rows) > limit {
		clear(s.rows)
	}
	if s.hitValid && s.hit.Offset != snapshot.Caret {
		s.hitValid = false
	}
}

func (editor *CodeEditor) shapedRow(row codeRow) []gfx.FittedMonoCaret {
	s := editor.shaping
	// Bound both the cost of shaping a single line and the number of cached rows.
	if s == nil || row.end <= row.start || row.end-row.start > 4096 {
		return nil
	}
	if shaped, ok := s.rows[row.start]; ok {
		return shaped.carets
	}
	shaped := editor.measureShapedRow(row)
	s.rows[row.start] = shaped
	return shaped.carets
}

func (editor *CodeEditor) measureShapedRow(row codeRow) codeShapedRow {
	s := editor.shaping
	value := editor.rowText(row)
	var prefix string
	if editor.wrap {
		// Give each plain continuation the base direction of its logical line.
		// Explicit embedding and isolate state cannot be reconstructed by a mark.
		line := editor.text[row.lineStart:]
		if len(line) > 4097 {
			line = line[:4097]
		}
		if end := strings.IndexByte(line, '\n'); end >= 0 {
			line = line[:end]
		}
		if len(line) > 4096 || strings.ContainsAny(line, "\u202a\u202b\u202c\u202d\u202e\u2066\u2067\u2068\u2069") {
			return codeShapedRow{}
		}
		dir := gfx.TextDirectionOf(line)
		if dir != gfx.TextDirectionOf(value) {
			prefix = "\u200e"
			if dir == gfx.DirectionRTL {
				prefix = "\u200f"
			}
		}
	}
	label := prefix + value
	carets, ok := s.canvas.FittedBidiMonoCarets(nil, label, s.style, float32(row.cells)*s.cell)
	if !ok || len(carets) < 2 || carets[0].Offset != 0 || carets[len(carets)-1].Offset != len(label) {
		return codeShapedRow{}
	}
	if row.shown >= 0 || prefix != "" {
		// Retain only source grapheme boundaries. Every byte of an expanded tab
		// lies between the same two source offsets; the direction mark has none.
		source := editor.text[row.start:row.end]
		mapped := carets[:0]
		column, rendered, at := 0, len(prefix), 0
		for offset := 0; ; {
			for at < len(carets) && carets[at].Offset < rendered {
				at++
			}
			if at == len(carets) || carets[at].Offset != rendered {
				return codeShapedRow{}
			}
			caret := carets[at]
			caret.Offset = offset
			mapped = append(mapped, caret)
			if offset == len(source) {
				break
			}
			next, cells := editor.gridCluster(source, offset, column)
			if source[offset] == '\t' {
				rendered += cells
			} else {
				rendered += next - offset
			}
			column += cells
			offset = next
		}
		carets = mapped
	}
	return codeShapedRow{value: label, carets: carets}
}

func (editor *CodeEditor) shapedCaretX(row codeRow, offset int) (float32, bool) {
	if carets := editor.shapedRow(row); len(carets) != 0 {
		s := editor.shaping
		if s.hitValid && s.hitRow == row.start && s.hit.Offset == offset {
			return s.hit.X, true
		}
		local := max(0, min(offset-int(row.start), int(row.end-row.start)))
		at, exact := slices.BinarySearchFunc(carets, local, func(caret gfx.FittedMonoCaret, local int) int {
			return cmp.Compare(caret.Offset, local)
		})
		if !exact {
			at = max(0, at-1)
		}
		if at == 0 {
			return carets[0].Leading, true
		}
		return carets[at].Trailing, true
	}
	return 0, false
}

func (editor *CodeEditor) shapedHit(row codeRow, x float32) (gfx.TextHit, bool) {
	carets := editor.shapedRow(row)
	if len(carets) == 0 {
		return gfx.TextHit{}, false
	}
	best := gfx.TextHit{Offset: int(row.start), X: carets[0].Leading}
	gap := float32(-1)
	for i := 0; i+1 < len(carets); i++ {
		leading, trailing := carets[i].Leading, carets[i+1].Trailing
		left, right := min(leading, trailing), max(leading, trailing)
		if left == right {
			continue
		}
		distance := max(left-x, x-right, 0)
		if gap >= 0 && distance >= gap {
			continue
		}
		position, boundary := leading, i
		if max(x-trailing, trailing-x) < max(x-leading, leading-x) {
			position, boundary = trailing, i+1
		}
		best = gfx.TextHit{Offset: int(row.start) + carets[boundary].Offset, X: position}
		gap = distance
		if gap == 0 {
			break
		}
	}
	return best, true
}

func (editor *CodeEditor) rememberShapedHit(row codeRow, hit gfx.TextHit) {
	if s := editor.shaping; s != nil {
		s.hit, s.hitRow, s.hitValid = hit, row.start, true
	}
}

func codeShapedSpans(dst []gfx.TextSpan, carets []gfx.FittedMonoCaret, start, end int) []gfx.TextSpan {
	dst = dst[:0]
	if start >= end {
		return dst
	}
	for i := 0; i+1 < len(carets); i++ {
		if carets[i].Offset >= end {
			break
		}
		if carets[i+1].Offset <= start {
			continue
		}
		a, b := carets[i].Leading, carets[i+1].Trailing
		if a != b {
			dst = append(dst, gfx.TextSpan{X: min(a, b), Width: max(a, b) - min(a, b)})
		}
	}
	slices.SortFunc(dst, func(a, b gfx.TextSpan) int { return cmp.Compare(a.X, b.X) })
	kept := dst[:0]
	for _, span := range dst {
		if n := len(kept); n != 0 && span.X <= kept[n-1].X+kept[n-1].Width+0.001 {
			last := &kept[n-1]
			last.Width = max(last.X+last.Width, span.X+span.Width) - last.X
		} else {
			kept = append(kept, span)
		}
	}
	return kept
}

func (paint *codePaint) shapedRow(index int, row codeRow, y float32, carets []gfx.FittedMonoCaret) {
	start, end := int(row.start), int(row.end)
	left := paint.editor.content.X - paint.editor.scrollX
	fill := func(from, to int, color gfx.Color) {
		paint.shapedSpans = codeShapedSpans(paint.shapedSpans, carets, from-start, to-start)
		for _, span := range paint.shapedSpans {
			paint.canvas.Fill(gfx.Rect{X: left + span.X, Y: y, Width: span.Width, Height: paint.lineHeight}, color)
		}
	}
	for _, layer := range paint.markLayers {
		if layer.Color.Alpha <= 0 {
			continue
		}
		first := sort.Search(len(layer.Spans), func(i int) bool { return layer.Spans[i].End > start })
		for _, mark := range layer.Spans[first:] {
			if mark.Start >= end {
				break
			}
			fill(mark.Start, mark.End, layer.Color)
		}
	}
	for paint.markCursor < len(paint.marks) && paint.marks[paint.markCursor].End <= start {
		paint.markCursor++
	}
	for i := paint.markCursor; i < len(paint.marks) && paint.marks[i].Start < end; i++ {
		mark := paint.marks[i]
		color := codeMatchColor
		if mark == paint.current {
			color = codeCurrentColor
		}
		fill(mark.Start, mark.End, color)
	}
	if paint.bracketsOK {
		color := paint.emphasis
		color.Alpha *= 0.22
		for _, at := range paint.brackets {
			if at >= start && at < end {
				fill(at, at+1, color)
			}
		}
	}
	selection := func(from, to int) {
		if from >= to || from > end || to <= start {
			return
		}
		fill(from, to, paint.selectionBackground)
		if to > end && end < len(paint.value) && !paint.editor.softWrapped(index) {
			extra := paint.cellWidth / 2
			x := left + carets[len(carets)-1].Trailing
			if carets[0].Leading > carets[len(carets)-1].Trailing {
				x -= extra
			}
			paint.canvas.Fill(gfx.Rect{X: x, Y: y, Width: extra, Height: paint.lineHeight}, paint.selectionBackground)
		}
	}
	if len(paint.selectionRanges) == 0 {
		selection(paint.selectionStart, paint.selectionEnd)
	} else {
		first := sort.Search(len(paint.selectionRanges), func(i int) bool { return paint.selectionRanges[i].End > start })
		for _, span := range paint.selectionRanges[first:] {
			if span.Start > end {
				break
			}
			selection(span.Start, span.End)
		}
	}
	value := paint.editor.shaping.rows[row.start].value
	bounds := gfx.Rect{X: left, Y: y + (paint.lineHeight-paint.cellHeight)/2, Width: float32(row.cells) * paint.cellWidth, Height: paint.cellHeight}
	paint.shapedText(row, value, bounds, carets)
	if paint.showWrapSymbol && paint.editor.softWrapped(index) {
		paint.wrapSymbol(y)
	}
	paint.overlays(index, y)
}

type codeShapedStyle struct {
	left, right float32
	color       gfx.Color
	italic      bool
}

func (paint *codePaint) shapedText(row codeRow, value string, bounds gfx.Rect, carets []gfx.FittedMonoCaret) {
	style := gfx.TextStyle{Size: paint.size, Mono: true, MonoFace: paint.face, Color: paint.foreground}
	run := gfx.TextRun{Bytes: int32(len(value)), Cells: row.cells, Ligatures: true, Color: paint.foreground}
	start, end := int(row.start), int(row.end)
	highlight := paint.lineRuns(row)
	selection := sort.Search(len(paint.selectionRanges), func(i int) bool { return paint.selectionRanges[i].End > start })
	selected := paint.selectionStart < paint.selectionEnd && paint.selectionStart < end && paint.selectionEnd > start
	if len(paint.selectionRanges) > 0 {
		selected = selection < len(paint.selectionRanges) && paint.selectionRanges[selection].Start < end
	}
	if !selected && len(highlight) == 0 {
		paint.canvas.LabelRuns(value, bounds, style, paint.cellWidth, []gfx.TextRun{run})
		return
	}

	// Assign styles in source order, then paint disjoint visual intervals.
	// Each interval draws the complete fitted label, preserving shaping and joins.
	s := paint.editor.shaping
	intervals := s.styles[:0]
	for i := 0; i+1 < len(carets); i++ {
		from, to := start+carets[i].Offset, start+carets[i+1].Offset
		local := from - int(row.lineStart)
		for len(highlight) > 0 && highlight[0].End <= local {
			highlight = highlight[1:]
		}
		color, italic := paint.foreground, false
		if len(highlight) > 0 && highlight[0].Start <= local {
			color, italic = highlight[0].Color, highlight[0].Italic
			if paint.editor.disabled {
				color.Alpha *= 0.5
			}
		}
		if selected {
			inSelection := paint.selectionStart < to && paint.selectionEnd > from
			if len(paint.selectionRanges) > 0 {
				for selection < len(paint.selectionRanges) && paint.selectionRanges[selection].End <= from {
					selection++
				}
				inSelection = selection < len(paint.selectionRanges) && paint.selectionRanges[selection].Start < to
			}
			if inSelection {
				color = paint.selectionForeground
			}
		}
		a, b := carets[i].Leading, carets[i+1].Trailing
		left, right := max(0, min(a, b)), min(bounds.Width, max(a, b))
		if right > left {
			intervals = append(intervals, codeShapedStyle{left, right, color, italic})
		}
	}
	s.styles = intervals
	slices.SortFunc(intervals, func(a, b codeShapedStyle) int { return cmp.Compare(a.left, b.left) })
	pos := float32(0)
	visibleLeft := paint.editor.content.X - bounds.X
	visibleRight := visibleLeft + paint.editor.content.Width
	draw := func(left, right float32, color gfx.Color, italic bool) {
		if right <= left || right <= visibleLeft || left >= visibleRight {
			return
		}
		run.Color, run.Italic, style.Color = color, italic, color
		if left == 0 && right == bounds.Width {
			paint.canvas.LabelRuns(value, bounds, style, paint.cellWidth, []gfx.TextRun{run})
			return
		}
		paint.canvas.Clip(gfx.Rect{X: bounds.X + left, Y: bounds.Y, Width: right - left, Height: bounds.Height})
		paint.canvas.LabelRuns(value, bounds, style, paint.cellWidth, []gfx.TextRun{run})
		paint.canvas.Unclip()
	}
	for i := 0; i < len(intervals); {
		span := intervals[i]
		i++
		for i < len(intervals) && intervals[i].left <= span.right && intervals[i].color == span.color && intervals[i].italic == span.italic {
			span.right = max(span.right, intervals[i].right)
			i++
		}
		draw(pos, span.left, paint.foreground, false)
		draw(max(pos, span.left), span.right, span.color, span.italic)
		pos = max(pos, span.right)
	}
	draw(pos, bounds.Width, paint.foreground, false)
}
