package components

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// A CodeEditor lays its text out on a grid of character cells, as a terminal
// does: every character is one cell wide, a wide one (CJK, emoji) two, a
// combining mark none, and a tab runs to the next tab stop. Columns, caret
// movement, selections and indentation all become cell arithmetic, which needs no
// text measurement and so costs the same however long the text is.

// codeRow is one row of the grid: the bytes value[start:end] of one logical
// line, which a soft wrap splits into several rows.
type codeRow struct {
	start, end int32 // end is before the line break that ends the line
	lineStart  int32 // where the logical line starts
	line       int32 // the logical line's index, from zero
	cells      int32 // how many cells the row spans, with tabs expanded
	shown      int32 // index into CodeEditor.shown of the row's text with its tabs expanded, or -1
}

// codeCells is how many cells r takes at column (a tab fills to the next stop).
func codeCells(r rune, column, tabWidth int) int {
	if r == '\t' {
		return tabWidth - column%tabWidth
	}
	return text.CellWidth(r)
}

func decodeAt(value string, at int) (rune, int) {
	if r := value[at]; r < utf8.RuneSelf {
		return rune(r), 1
	}
	return utf8.DecodeRuneInString(value[at:])
}

// fit lays value out in rows of cols cells unless it already is.
func (editor *CodeEditor) fit(value string, cols int, wrap bool, tabWidth int) {
	if editor.fitted && editor.cols == cols && editor.wrap == wrap && editor.tabWidth == tabWidth && editor.text == value {
		return
	}
	editor.fitted, editor.cols, editor.wrap, editor.tabWidth, editor.text = true, cols, wrap, tabWidth, value
	editor.rows = editor.rows[:0]
	editor.shown = editor.shown[:0]
	editor.widest = 0
	if len(value) > 1<<30 {
		// Rows are indexed with 32 bits; a text this long is not an editor's.
		value = ""
		editor.text = ""
	}
	line := 0
	for start := 0; ; line++ {
		end := len(value)
		if newline := strings.IndexByte(value[start:], '\n'); newline >= 0 {
			end = start + newline
		}
		editor.appendLine(value, start, end, line)
		if end >= len(value) {
			break
		}
		start = end + 1
	}
	editor.lineCount = line + 1
	editor.runLine = -1
}

// appendLine appends the rows of the line value[start:end]. A soft wrap breaks
// before the word that does not fit, or inside it when it is longer than a row;
// the spaces that end a row hang past its right edge instead of wrapping.
func (editor *CodeEditor) appendLine(value string, start, end, line int) {
	rowStart, column := start, 0
	wordStart, wordColumn := -1, 0
	previousSpace := false
	for at := start; at < end; {
		r, size := decodeAt(value, at)
		width := codeCells(r, column, editor.tabWidth)
		space := r == ' ' || r == '\t'
		if !space && previousSpace {
			wordStart, wordColumn = at, column
		}
		if editor.wrap && !space && column+width > editor.cols && column > 0 {
			cut, cutColumn := at, column
			if wordStart > rowStart {
				cut, cutColumn = wordStart, wordColumn
			}
			editor.addRow(value, rowStart, cut, start, line, cutColumn)
			rowStart, wordStart = cut, -1
			column = editor.columnBetween(value, cut, at)
			width = codeCells(r, column, editor.tabWidth)
		}
		previousSpace = space
		column += width
		at += size
	}
	editor.addRow(value, rowStart, end, start, line, column)
}

// columnBetween is the width in cells of value[from:to] as a row beginning at
// from lays it out.
func (editor *CodeEditor) columnBetween(value string, from, to int) int {
	column := 0
	for at := from; at < to; {
		r, size := decodeAt(value, at)
		column += codeCells(r, column, editor.tabWidth)
		at += size
	}
	return column
}

func (editor *CodeEditor) addRow(value string, start, end, lineStart, line, cells int) {
	row := codeRow{start: int32(start), end: int32(end), lineStart: int32(lineStart), line: int32(line), cells: int32(cells), shown: -1}
	if strings.IndexByte(value[start:end], '\t') >= 0 {
		var expanded strings.Builder
		expanded.Grow(cells + 4)
		column := 0
		for at := start; at < end; {
			r, size := decodeAt(value, at)
			width := codeCells(r, column, editor.tabWidth)
			if r == '\t' {
				for range width {
					expanded.WriteByte(' ')
				}
			} else {
				expanded.WriteString(value[at : at+size])
			}
			column += width
			at += size
		}
		row.shown = int32(len(editor.shown))
		editor.shown = append(editor.shown, expanded.String())
	}
	editor.widest = max(editor.widest, cells)
	editor.rows = append(editor.rows, row)
}

// rowText is the text a row is drawn with: its own bytes, or with tabs expanded.
func (editor *CodeEditor) rowText(row codeRow) string {
	if row.shown >= 0 {
		return editor.shown[row.shown]
	}
	return editor.text[row.start:row.end]
}

// firstOfLine reports whether rows[index] begins its logical line.
func (editor *CodeEditor) firstOfLine(index int) bool {
	return index == 0 || editor.rows[index-1].line != editor.rows[index].line
}

// softWrapped reports whether rows[index] ends where the next row of the same
// line begins, rather than at the end of the line.
func (editor *CodeEditor) softWrapped(index int) bool {
	return index+1 < len(editor.rows) && editor.rows[index+1].line == editor.rows[index].line
}

// rowOf is the row the caret at offset is drawn on. A caret at a soft wrap is
// the end of one row and the start of the next; it is drawn at the start of the
// next unless it got to the end of the first by going there.
func (editor *CodeEditor) rowOf(offset int) int {
	index := sort.Search(len(editor.rows), func(i int) bool { return int(editor.rows[i].start) > offset }) - 1
	index = max(index, 0)
	if editor.upstream && editor.upstreamCaret == offset && index > 0 && int(editor.rows[index].start) == offset &&
		int(editor.rows[index-1].end) == offset && editor.rows[index-1].line == editor.rows[index].line {
		index--
	}
	return index
}

// columnOf is the cell column of byte offset on row, counted from its start.
func (editor *CodeEditor) columnOf(row codeRow, offset int) int {
	walker := columnWalker{editor: editor, row: row, at: int(row.start)}
	return walker.column(offset)
}

// columnWalker finds the columns of increasing offsets on one row in a single
// pass over it.
type columnWalker struct {
	editor *CodeEditor
	row    codeRow
	at     int
	cells  int
}

func (walker *columnWalker) column(offset int) int {
	if offset < walker.at {
		walker.at, walker.cells = int(walker.row.start), 0
	}
	target := min(offset, int(walker.row.end))
	for walker.at < target {
		r, size := decodeAt(walker.editor.text, walker.at)
		walker.cells += codeCells(r, walker.cells, walker.editor.tabWidth)
		walker.at += size
	}
	return walker.cells
}

// offsetAtColumn is the byte offset on row whose boundary is nearest to x, a
// distance in cells from the start of the row.
func (editor *CodeEditor) offsetAtColumn(row codeRow, x float32) int {
	column := 0
	for at := int(row.start); at < int(row.end); {
		r, size := decodeAt(editor.text, at)
		width := codeCells(r, column, editor.tabWidth)
		if x < float32(column)+float32(width)/2 {
			return at
		}
		column += width
		at += size
	}
	return int(row.end)
}

// OffsetAt is the byte offset of the caret position nearest to a window point,
// for the runtime to place the caret on a press or drag. It implements
// ui.TextPlacement.
func (editor *CodeEditor) OffsetAt(point ui.Point) (int, bool) {
	if !editor.painted || !editor.fitted || len(editor.rows) == 0 || editor.cellWidth <= 0 {
		return 0, false
	}
	index := int((point.Y - editor.content.Y + editor.scrollY) / editor.lineHeight)
	if point.Y < editor.content.Y-editor.scrollY {
		index = 0
	}
	index = max(0, min(index, len(editor.rows)-1))
	row := editor.rows[index]
	offset := editor.offsetAtColumn(row, (point.X-editor.content.X+editor.scrollX)/editor.cellWidth)
	editor.setUpstream(offset, offset == int(row.end) && editor.softWrapped(index))
	return offset, true
}

// CaretBounds is where the caret was painted, one pixel wide, in window
// coordinates; false while it is scrolled out of view. It implements
// ui.TextPlacement.
func (editor *CodeEditor) CaretBounds() (ui.Rect, bool) {
	return editor.caretRect, editor.painted && editor.caretShown
}

func (editor *CodeEditor) setUpstream(offset int, upstream bool) {
	editor.upstream, editor.upstreamCaret = upstream, offset
}

// arrive notes where a movement ended and returns it.
func (editor *CodeEditor) arrive(offset int, upstream bool) int {
	editor.setUpstream(offset, upstream)
	editor.goalActive = false
	return offset
}

// Move returns the byte offset of text that movement leads to from caret, laid
// out as the last paint laid it out: Up and Down keep the column a run of them
// began at, Home goes to the start of the row (to its first character first, in
// the first row of a line), End to its end, and Up on the first row or Down on
// the last go to the ends of the text. It records the column and where a caret
// at a soft wrap is drawn, so call it for every movement and then put the caret
// at the offset it returns. Before the first paint it returns caret.
func (editor *CodeEditor) Move(text string, caret int, movement TextareaMovement) int {
	caret = max(0, min(caret, len(text)))
	switch movement {
	case TextareaMoveTextStart:
		return editor.arrive(0, false)
	case TextareaMoveTextEnd:
		return editor.arrive(len(text), false)
	}
	if !editor.painted || len(editor.rows) == 0 || editor.text != text {
		return caret
	}
	index := editor.rowOf(caret)
	row := editor.rows[index]
	switch movement {
	case TextareaMoveLineStart:
		target := int(row.start)
		if editor.firstOfLine(index) {
			// The first character of the line, and from there the start of it.
			first := int(row.start)
			for first < int(row.end) && (editor.text[first] == ' ' || editor.text[first] == '\t') {
				first++
			}
			if caret != first {
				target = first
			}
		}
		return editor.arrive(target, false)
	case TextareaMoveLineEnd:
		return editor.arrive(int(row.end), editor.softWrapped(index))
	case TextareaMoveUp, TextareaMoveDown, TextareaMovePageUp, TextareaMovePageDown:
	default:
		return caret
	}
	step := 1
	if movement == TextareaMovePageUp || movement == TextareaMovePageDown {
		step = max(1, int(editor.content.Height/editor.lineHeight)-1)
	}
	if movement == TextareaMoveUp || movement == TextareaMovePageUp {
		step = -step
	}
	if !editor.goalActive || editor.goalCaret != caret {
		editor.goal = float32(editor.columnOf(row, caret))
	}
	target := index + step
	var offset int
	var upstream bool
	switch {
	case target < 0:
		offset = 0
	case target >= len(editor.rows):
		offset = len(text)
	default:
		offset = editor.offsetAtColumn(editor.rows[target], editor.goal)
		upstream = offset == int(editor.rows[target].end) && editor.softWrapped(target)
	}
	goal := editor.goal
	offset = editor.arrive(offset, upstream)
	editor.goal, editor.goalCaret, editor.goalActive = goal, offset, true
	return offset
}

// Reveal scrolls the next paint to show the caret at offset, as if the caret had
// just moved there.
func (editor *CodeEditor) Reveal(offset int) {
	editor.followed, editor.followedValid = offset, false
}

// ScrollBy scrolls the view by dx and dy pixels, right and down when positive,
// as a wheel does. The next paint keeps the offset until the caret moves.
func (editor *CodeEditor) ScrollBy(dx, dy float32) {
	if !editor.painted {
		return
	}
	editor.scrollY = editor.clampScrollY(editor.scrollY + dy)
	editor.scrollX = editor.clampScrollX(editor.scrollX + dx)
}

// Scroll applies a wheel or trackpad gesture: vertical by whole rows for a
// wheel and by the distance for a trackpad, horizontal when the text does not
// wrap. Route the window's scroll events over the editor to it.
func (editor *CodeEditor) Scroll(event ui.ScrollEvent) ui.EventResult {
	if !editor.painted {
		return ui.EventResult{}
	}
	dx, dy := event.StepX(editor.cellWidth), event.StepY(editor.lineHeight)
	if event.Modifiers&ui.ModifierShift != 0 && dx == 0 {
		dx, dy = dy, 0
	}
	if editor.wrap {
		dx = 0
	}
	beforeX, beforeY := editor.scrollX, editor.scrollY
	editor.ScrollBy(dx, dy)
	if editor.scrollX == beforeX && editor.scrollY == beforeY {
		return ui.EventResult{}
	}
	return ui.EventResult{Handled: true, Repaint: true}
}

func (editor *CodeEditor) clampScrollY(scroll float32) float32 {
	most := float32(len(editor.rows))*editor.lineHeight - editor.content.Height
	return max(0, min(scroll, most))
}

func (editor *CodeEditor) clampScrollX(scroll float32) float32 {
	if editor.wrap {
		return 0
	}
	most := float32(editor.widest+1)*editor.cellWidth - editor.content.Width
	return max(0, min(scroll, most))
}

// follow scrolls to keep the caret in view when the caret, or the row it is on,
// is not where the last paint left it. A wheel or a drag of a scrollbar moves the
// view and the next paint leaves it there until the caret moves.
func (editor *CodeEditor) follow(caret, row, column int) {
	if !editor.followedValid || editor.followed != caret || editor.followedRow != row {
		top := float32(row) * editor.lineHeight
		if top < editor.scrollY {
			editor.scrollY = top
		} else if bottom := top + editor.lineHeight; bottom > editor.scrollY+editor.content.Height {
			editor.scrollY = bottom - editor.content.Height
		}
		if !editor.wrap {
			left, right := float32(column)*editor.cellWidth, float32(column+1)*editor.cellWidth
			if left < editor.scrollX {
				editor.scrollX = max(0, left-editor.cellWidth*2)
			} else if right > editor.scrollX+editor.content.Width {
				editor.scrollX = right - editor.content.Width + editor.cellWidth*2
			}
		}
	}
	editor.followed, editor.followedRow, editor.followedValid = caret, row, true
	editor.scrollY = editor.clampScrollY(editor.scrollY)
	editor.scrollX = editor.clampScrollX(editor.scrollX)
}

// LineCount is the number of logical lines of the text last painted.
func (editor *CodeEditor) LineCount() int { return editor.lineCount }

// VisibleLines is the first and last logical lines (from zero) that the last
// paint showed at least in part, for a status bar or a minimap.
func (editor *CodeEditor) VisibleLines() (first, last int) {
	if !editor.painted || len(editor.rows) == 0 || editor.lineHeight <= 0 {
		return 0, 0
	}
	top := max(0, min(int(editor.scrollY/editor.lineHeight), len(editor.rows)-1))
	bottom := max(top, min(int((editor.scrollY+editor.content.Height)/editor.lineHeight), len(editor.rows)-1))
	return int(editor.rows[top].line), int(editor.rows[bottom].line)
}
