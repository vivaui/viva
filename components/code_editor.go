package components

import (
	"strings"

	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// CodeEditorRun colors one byte range of a line without changing its text
// geometry. Runs are ordered and non-overlapping; invalid and overlapping spans
// are ignored.
type CodeEditorRun struct {
	Start, End int
	Color      ui.Color
}

// CodeEditorMaxHighlightedLine is the longest line, in bytes, that is handed to
// the highlighter. A longer one, such as a minified file's only line, is painted
// plain: highlighting it would cost a megabyte of work for every frame.
const CodeEditorMaxHighlightedLine = 10_000

// CodeEditorSnapshot is one caller-owned presentation of a code editor. The
// text, selection and composition belong to the application's text.Editor; the
// offsets are bytes of Value, which text.Editor.ByteOffset gives.
type CodeEditorSnapshot struct {
	ID    ui.ID
	Label string
	Hint  string
	Value string
	// Placeholder is shown, dim, while Value is empty.
	Placeholder string
	Disabled    bool
	ReadOnly    bool
	Focused     bool
	// ShowCaret paints the caret of a focused editor; an application that blinks
	// it turns this on and off.
	ShowCaret        bool
	Caret            int
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
	// FontSize is the size of the code, in logical pixels; zero takes the theme's.
	FontSize float32
	// MonoFace is the fixed-pitch font the code is set in.
	MonoFace ui.MonoFace
	// TabWidth is the width of a tab stop in cells; zero is four.
	TabWidth int
	// Wrap folds a line longer than the box onto several rows. Without it each
	// line is one row and the view scrolls sideways, following the caret.
	Wrap bool
	// LineNumbers shows the number of each line in a gutter on the left.
	LineNumbers bool
	// CurrentLine tints the line the caret is on, while nothing is selected.
	CurrentLine bool
	// MatchBrackets outlines the bracket next to the caret and its partner.
	MatchBrackets bool
	// Language is handed to Highlight, which owns the grammar.
	Language  string
	Highlight func(language, line string, dst []CodeEditorRun) []CodeEditorRun
	// Marks are byte ranges of Value painted behind the text, such as the matches
	// of a search (find.Session.Matches is exactly this). They must be in order
	// and not overlap; empty ones are not drawn. The mark equal to CurrentMark is
	// painted brighter: the match the find bar is on.
	Marks       []text.Span
	CurrentMark text.Span
	// ScrollbarID enables a vertical scrollbar, and HorizontalScrollbarID a
	// horizontal one for text that does not wrap. Route pointer events to Pointer.
	ScrollbarID           ui.ID
	HorizontalScrollbarID ui.ID
}

// CodeEditor is the view of a code editor, held by the application next to its
// text.Editor and given the snapshot at every paint. It lays the text out on a
// grid of character cells in a fixed-pitch font and paints only the rows in
// view, in one label for each, whatever colors the highlighter gives them. It
// keeps what a steady paint would otherwise rebuild: the rows, the scroll
// position and the column of a run of Up and Down keys. The text and its undo
// history stay in the application's editor.
//
// The zero value is ready. It is not safe for concurrent use, and one CodeEditor
// serves one editor.
type CodeEditor struct {
	// The rows of text, for the geometry they were laid out at.
	text      string
	cols      int
	wrap      bool
	tabWidth  int
	fitted    bool
	rows      []codeRow
	shown     []string
	widest    int
	lineCount int

	// What the last paint showed.
	painted    bool
	content    ui.Rect
	cellWidth  float32
	lineHeight float32
	scrollX    float32
	scrollY    float32
	caretRect  ui.Rect
	caretShown bool
	disabled   bool

	// The caret the view last followed.
	followed      int
	followedRow   int
	followedValid bool

	// A run of Up and Down keeps the column the caret began at.
	goal       float32
	goalCaret  int
	goalActive bool
	// A caret at a soft wrap is drawn at the end of the row it got to by going
	// there, and otherwise at the start of the next.
	upstream      bool
	upstreamCaret int

	// Scratch kept between paints.
	runs     []ui.TextRun
	lineRuns []CodeEditorRun
	runLine  int
	numbers  lineNumbers

	// Scrollbars.
	barID, barXID ui.ID
	barV, barH    ScrollAreaGeometry
	drag          codeDrag
	grab          float32
}

type codeDrag uint8

const (
	codeDragNone codeDrag = iota
	codeDragVertical
	codeDragHorizontal
)

const (
	codeEditorPadY    = 8
	codeEditorBarSize = 12
	codeEditorMinRows = 64
)

// Paint paints the editor in bounds, whose height is at least 64, and returns
// the bounds it used.
func (editor *CodeEditor) Paint(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, snapshot CodeEditorSnapshot) ui.Rect {
	if bounds.Height < codeEditorMinRows {
		bounds.Height = codeEditorMinRows
	}
	editor.painted, editor.caretShown = false, false
	editor.disabled = snapshot.Disabled
	editor.barID, editor.barXID = snapshot.ScrollbarID, snapshot.HorizontalScrollbarID
	editor.barV, editor.barH = ScrollAreaGeometry{}, ScrollAreaGeometry{}
	if canvas == nil || bounds.Width <= 0 {
		return bounds
	}
	focused := !snapshot.Disabled && (snapshot.Focused || !snapshot.ID.IsZero() && canvas.Focused() == snapshot.ID)
	paintTextEntrySurface(canvas, theme, bounds, textEntryState{Focused: focused, Disabled: snapshot.Disabled})

	size := snapshot.FontSize
	if size <= 0 {
		size = theme.Metrics.FontSizeM
	}
	cellWidth, cellHeight := canvas.MonoCellSize(size, snapshot.MonoFace)
	if cellWidth <= 0 || cellHeight <= 0 {
		return bounds
	}
	lineHeight := max(cellHeight, float32(int(size*1.5+0.5)))
	tabWidth := snapshot.TabWidth
	switch {
	case tabWidth <= 0:
		tabWidth = 4
	case tabWidth > 16:
		tabWidth = 16
	}
	value := snapshot.Value

	// The gutter is as wide as the number of the last line, a cell either side.
	gutter := float32(0)
	if snapshot.LineNumbers {
		lines := editor.lineCount
		if !editor.fitted || editor.text != value {
			lines = strings.Count(value, "\n") + 1
		}
		gutter = float32(max(decimalDigits(lines), 2)+2) * cellWidth
	}
	inset := theme.Metrics.TextInsetX()
	barWidth, barHeight := float32(0), float32(0)
	if !snapshot.ScrollbarID.IsZero() {
		barWidth = codeEditorBarSize
	}
	if !snapshot.Wrap && !snapshot.HorizontalScrollbarID.IsZero() {
		barHeight = codeEditorBarSize
	}
	content := ui.Rect{
		X: bounds.X + inset + gutter, Y: bounds.Y + codeEditorPadY,
		Width:  max(0, bounds.Width-2*inset-gutter-barWidth),
		Height: max(0, bounds.Height-2*codeEditorPadY-barHeight),
	}
	cols := max(1, int(content.Width/cellWidth))
	editor.fit(value, cols, snapshot.Wrap, tabWidth)
	editor.content, editor.cellWidth, editor.lineHeight = content, cellWidth, lineHeight

	placeholder := value == ""
	caret := textareaBoundary(value, snapshot.Caret)
	selectionStart, selectionEnd := textareaRange(value, snapshot.SelectionStart, snapshot.SelectionEnd)
	compositionStart, compositionEnd := textareaRange(value, snapshot.CompositionStart, snapshot.CompositionEnd)
	caretRow := editor.rowOf(caret)
	caretColumn := editor.columnOf(editor.rows[caretRow], caret)
	if placeholder {
		editor.scrollX, editor.scrollY, editor.followedValid = 0, 0, false
	} else {
		editor.follow(caret, caretRow, caretColumn)
	}
	editor.painted = true

	foreground := theme.Foreground
	if snapshot.Disabled {
		foreground.Alpha *= 0.5
	}
	selectionBackground, selectionForeground := selectionColors(theme)
	if snapshot.Disabled {
		selectionBackground.Alpha *= 0.5
		selectionForeground.Alpha *= 0.5
	}
	paint := codePaint{
		canvas: canvas, theme: theme, editor: editor, value: value, size: size,
		cellWidth: cellWidth, cellHeight: cellHeight, lineHeight: lineHeight,
		foreground: foreground, selectionBackground: selectionBackground, selectionForeground: selectionForeground,
		selectionStart: selectionStart, selectionEnd: selectionEnd,
		compositionStart: compositionStart, compositionEnd: compositionEnd,
		caret: caret, caretRow: caretRow, caretColumn: caretColumn,
		showCaret: focused && snapshot.ShowCaret && !snapshot.Disabled,
		language:  snapshot.Language, highlight: snapshot.Highlight, face: snapshot.MonoFace,
		firstColumn: int(editor.scrollX / cellWidth), marks: snapshot.Marks, current: snapshot.CurrentMark,
	}
	paint.lastColumn = paint.firstColumn + cols + 8
	if snapshot.MatchBrackets && selectionStart == selectionEnd && !placeholder {
		if open, closer, ok := text.MatchBracket(value, caret); ok {
			paint.brackets, paint.bracketsOK = [2]int{open, closer}, true
		}
	}
	editor.runLine = -1

	firstRow := int(editor.scrollY / lineHeight)
	inner := ui.Rect{X: bounds.X + 1, Y: content.Y, Width: content.X + content.Width - bounds.X - 1, Height: content.Height}
	canvas.Clip(inner)
	if placeholder {
		style := ui.TextStyle{Size: size, Color: theme.MutedForeground, Mono: true, MonoFace: snapshot.MonoFace}
		if snapshot.Disabled {
			style.Color.Alpha *= 0.5
		}
		canvas.Label(snapshot.Placeholder, ui.Rect{X: content.X, Y: content.Y + (lineHeight-cellHeight)/2, Width: content.Width, Height: cellHeight}, style)
	}
	currentLine := -1
	if snapshot.CurrentLine && selectionStart == selectionEnd && !placeholder {
		currentLine = int(editor.rows[caretRow].line)
	}
	for index := firstRow; index < len(editor.rows); index++ {
		y := content.Y + float32(index)*lineHeight - editor.scrollY
		if y >= content.Y+content.Height {
			break
		}
		row := editor.rows[index]
		if int(row.line) == currentLine {
			tint := theme.Foreground
			tint.Alpha *= 0.05
			canvas.Fill(ui.Rect{X: inner.X, Y: y, Width: inner.Width, Height: lineHeight}, tint)
		}
		if snapshot.LineNumbers && editor.firstOfLine(index) {
			paint.lineNumber(row, y, bounds.X+inset, gutter, int(row.line) == int(editor.rows[caretRow].line))
		}
	}
	canvas.Clip(ui.Rect{X: content.X, Y: content.Y, Width: content.Width, Height: content.Height})
	paint.markCursor = paint.firstMark(firstRow)
	for index := firstRow; index < len(editor.rows); index++ {
		y := content.Y + float32(index)*lineHeight - editor.scrollY
		if y >= content.Y+content.Height {
			break
		}
		paint.row(index, y)
	}
	canvas.Unclip()
	canvas.Unclip()

	if !snapshot.ID.IsZero() {
		options := ui.HitOptions{
			Role: ui.RoleTextField, Cursor: ui.PointerCursorText, Value: value, Hint: snapshot.Hint,
			Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Multiline: true,
			TextBounds: content, TextSize: size, HideNativeOverlays: true, Placement: editor,
		}
		canvas.HitWithOptions(snapshot.ID, bounds, snapshot.Label, options)
	}
	if !snapshot.ScrollbarID.IsZero() {
		area := BeginScrollArea(canvas, theme, ui.Rect{X: bounds.X + bounds.Width - codeEditorBarSize - 1, Y: content.Y, Width: 10, Height: content.Height},
			ui.Size{Width: 10, Height: float32(len(editor.rows)) * lineHeight}).Offset(0, editor.scrollY).VerticalScrollbar(snapshot.ScrollbarID, snapshot.Label, "")
		if snapshot.Disabled {
			area = area.Disabled()
		}
		editor.barV = area.EndContent()
	}
	if barHeight > 0 {
		area := BeginScrollArea(canvas, theme, ui.Rect{X: content.X, Y: bounds.Y + bounds.Height - codeEditorBarSize - 1, Width: content.Width, Height: 10},
			ui.Size{Width: float32(editor.widest+1) * cellWidth, Height: 10}).Offset(editor.scrollX, 0).HorizontalScrollbar(snapshot.HorizontalScrollbarID, snapshot.Label, "")
		if snapshot.Disabled {
			area = area.Disabled()
		}
		editor.barH = area.EndContent()
	}
	return bounds
}

// Pointer handles a drag of either scrollbar without changing the text or the
// selection. Route pointer events to it.
func (editor *CodeEditor) Pointer(event ui.PointerEvent) ui.EventResult {
	if editor.disabled {
		return ui.EventResult{}
	}
	switch event.Kind {
	case ui.PointerDown:
		switch {
		case !editor.barID.IsZero() && event.Target == editor.barID && editor.barV.MaxOffsetY > 0:
			editor.grab = editor.barV.VerticalThumb.Height / 2
			if thumb := editor.barV.VerticalThumb; event.Y >= thumb.Y && event.Y < thumb.Y+thumb.Height {
				editor.grab = event.Y - thumb.Y
			}
			editor.drag = codeDragVertical
		case !editor.barXID.IsZero() && event.Target == editor.barXID && editor.barH.MaxOffsetX > 0:
			editor.grab = editor.barH.HorizontalThumb.Width / 2
			if thumb := editor.barH.HorizontalThumb; event.X >= thumb.X && event.X < thumb.X+thumb.Width {
				editor.grab = event.X - thumb.X
			}
			editor.drag = codeDragHorizontal
		default:
			return ui.EventResult{}
		}
	case ui.PointerMove:
		if editor.drag == codeDragNone {
			return ui.EventResult{}
		}
	default:
		if editor.drag == codeDragNone {
			return ui.EventResult{}
		}
		editor.drag = codeDragNone
		return ui.EventResult{Handled: true, Repaint: true}
	}
	if editor.drag == codeDragVertical {
		editor.scrollY = editor.clampScrollY(editor.barV.VerticalOffsetForThumb(event.Y - editor.grab))
	} else {
		editor.scrollX = editor.clampScrollX(editor.barH.HorizontalOffsetForThumb(event.X - editor.grab))
	}
	return ui.EventResult{Handled: true, Repaint: true}
}

// codePaint is what painting the rows of one frame shares.
type codePaint struct {
	canvas *ui.Canvas
	theme  ui.Theme
	editor *CodeEditor
	value  string

	size, cellWidth, cellHeight, lineHeight  float32
	foreground                               ui.Color
	selectionBackground, selectionForeground ui.Color
	selectionStart, selectionEnd             int
	compositionStart, compositionEnd         int
	caret, caretRow, caretColumn             int
	showCaret                                bool
	language                                 string
	highlight                                func(language, line string, dst []CodeEditorRun) []CodeEditorRun
	face                                     ui.MonoFace
	firstColumn, lastColumn                  int
	marks                                    []text.Span
	current                                  text.Span
	markCursor                               int
	brackets                                 [2]int
	bracketsOK                               bool
}

// firstMark is the index of the first mark that may reach the row at index.
func (paint *codePaint) firstMark(index int) int {
	if len(paint.marks) == 0 || index >= len(paint.editor.rows) {
		return 0
	}
	start := int(paint.editor.rows[index].start)
	low, high := 0, len(paint.marks)
	for low < high {
		middle := (low + high) / 2
		if paint.marks[middle].End <= start {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}

func (paint *codePaint) lineNumber(row codeRow, y, left, gutter float32, current bool) {
	number := paint.editor.numbers.text(int(row.line))
	color := paint.theme.MutedForeground
	if current {
		color = paint.theme.Foreground
	}
	if paint.editor.disabled {
		color.Alpha *= 0.5
	}
	width := float32(len(number)) * paint.cellWidth
	bounds := ui.Rect{X: left + gutter - paint.cellWidth - width, Y: y + (paint.lineHeight-paint.cellHeight)/2, Width: width, Height: paint.cellHeight}
	runs := [1]ui.TextRun{{Column: 0, Bytes: int32(len(number)), Cells: int32(len(number)), Color: color}}
	paint.canvas.LabelRuns(number, bounds, ui.TextStyle{Size: paint.size, Color: color, Mono: true, MonoFace: paint.face}, paint.cellWidth, runs[:])
}

// decimalDigits is how many digits n has, without building a string.
func decimalDigits(n int) int {
	digits := 1
	for ; n >= 10; n /= 10 {
		digits++
	}
	return digits
}

// The amber of a find match, as literals: a package variable built by a call
// would cost every application that imports the package an init function.
var (
	codeMatchColor   = ui.Color{Red: 250.0 / 255, Green: 204.0 / 255, Blue: 21.0 / 255, Alpha: 0.30}
	codeCurrentColor = ui.Color{Red: 250.0 / 255, Green: 204.0 / 255, Blue: 21.0 / 255, Alpha: 0.62}
)

// row paints one row of the text: what goes behind it, the text, and the caret.
func (paint *codePaint) row(index int, y float32) {
	editor, canvas := paint.editor, paint.canvas
	row := editor.rows[index]
	left := editor.content.X - editor.scrollX
	rowStart, rowEnd := int(row.start), int(row.end)

	marks := columnWalker{editor: editor, row: row, at: rowStart}
	// A fill is cut to the text area, so that a long line with many marks paints
	// only the ones in view.
	fill := func(walker *columnWalker, start, end int, extra float32, color ui.Color) {
		first, last := walker.column(start), walker.column(end)
		x0 := left + float32(first)*paint.cellWidth
		x1 := left + float32(last)*paint.cellWidth + extra
		x0, x1 = max(x0, editor.content.X), min(x1, editor.content.X+editor.content.Width)
		if x1 > x0 {
			canvas.Rounded(ui.Rect{X: x0, Y: y, Width: x1 - x0, Height: paint.lineHeight}, 2, color)
		}
	}
	for paint.markCursor < len(paint.marks) && paint.marks[paint.markCursor].End <= rowStart {
		paint.markCursor++
	}
	for at := paint.markCursor; at < len(paint.marks) && paint.marks[at].Start < rowEnd; at++ {
		mark := paint.marks[at]
		start, end := max(mark.Start, rowStart), min(mark.End, rowEnd)
		if end <= start {
			continue
		}
		color := codeMatchColor
		if mark == paint.current {
			color = codeCurrentColor
		}
		fill(&marks, start, end, 0, color)
	}
	if paint.bracketsOK {
		outline := paint.theme.Foreground
		outline.Alpha *= 0.22
		for _, at := range paint.brackets {
			if at >= rowStart && at < rowEnd {
				fill(&marks, at, at+1, 0, outline)
			}
		}
	}

	if paint.selectionEnd > paint.selectionStart && paint.selectionStart <= rowEnd && paint.selectionEnd >= rowStart {
		walker := columnWalker{editor: editor, row: row, at: rowStart}
		extra := float32(0)
		if paint.selectionEnd > rowEnd && !editor.softWrapped(index) {
			extra = paint.cellWidth / 2 // the line break is selected too
		}
		fill(&walker, max(paint.selectionStart, rowStart), min(paint.selectionEnd, rowEnd), extra, paint.selectionBackground)
	}

	shown := editor.rowText(row)
	spec := gridRunSpec{
		tabWidth: editor.tabWidth, firstColumn: paint.firstColumn, lastColumn: paint.lastColumn,
		foreground: paint.foreground, disabled: editor.disabled,
		highlight: paint.lineRuns(row), highlightBase: rowStart - int(row.lineStart),
		selectionStart: paint.selectionStart - rowStart, selectionEnd: paint.selectionEnd - rowStart,
		selectionForeground: paint.selectionForeground,
	}
	runs, from, to, startColumn, endColumn := gridRuns(editor.runs[:0], paint.value[rowStart:rowEnd], &spec)
	editor.runs = runs
	if len(runs) > 0 {
		bounds := ui.Rect{
			X: left + float32(startColumn)*paint.cellWidth, Y: y + (paint.lineHeight-paint.cellHeight)/2,
			Width: float32(max(endColumn-startColumn, 1)) * paint.cellWidth, Height: paint.cellHeight,
		}
		canvas.LabelRuns(shown[from:to], bounds, ui.TextStyle{Size: paint.size, Color: paint.foreground, Mono: true, MonoFace: paint.face}, paint.cellWidth, runs)
	}

	if paint.compositionEnd > paint.compositionStart && paint.compositionStart <= rowEnd && paint.compositionEnd >= rowStart {
		walker := columnWalker{editor: editor, row: row, at: rowStart}
		first, last := walker.column(max(paint.compositionStart, rowStart)), walker.column(min(paint.compositionEnd, rowEnd))
		if last > first {
			canvas.Fill(ui.Rect{X: left + float32(first)*paint.cellWidth, Y: y + paint.lineHeight - 3, Width: float32(last-first) * paint.cellWidth, Height: 1}, paint.foreground)
		}
	}

	if index == paint.caretRow {
		x := left + float32(paint.caretColumn)*paint.cellWidth
		if x >= editor.content.X-1 && x <= editor.content.X+editor.content.Width {
			editor.caretRect, editor.caretShown = ui.Rect{X: x, Y: y, Width: 1, Height: paint.lineHeight}, true
			if paint.showCaret {
				canvas.Fill(ui.Rect{X: x, Y: y + (paint.lineHeight-paint.cellHeight)/2, Width: 1, Height: paint.cellHeight}, paint.foreground)
			}
		}
	}
}

// lineRuns is the highlight of the row's logical line, kept while its rows are
// painted one after the other: the runs in order and within the line, with the
// ones the highlighter got wrong dropped.
func (paint *codePaint) lineRuns(row codeRow) []CodeEditorRun {
	editor := paint.editor
	if editor.runLine == int(row.line) {
		return editor.lineRuns
	}
	editor.runLine = int(row.line)
	editor.lineRuns = editor.lineRuns[:0]
	if paint.highlight == nil {
		return nil
	}
	start := int(row.lineStart)
	end := len(paint.value)
	if newline := strings.IndexByte(paint.value[start:], '\n'); newline >= 0 {
		end = start + newline
	}
	line := paint.value[start:end]
	if len(line) > CodeEditorMaxHighlightedLine {
		return nil
	}
	kept := cleanHighlight(line, paint.highlight(paint.language, line, editor.lineRuns[:0]))
	editor.lineRuns = kept
	return kept
}

// CodeEditorCommand is an editing command a key asks a code editor for.
type CodeEditorCommand uint8

const (
	CodeEditorNoCommand CodeEditorCommand = iota
	// CodeEditorInsertTab is the Tab key: indent the selected lines, or insert a
	// tab or spaces to the next tab stop.
	CodeEditorInsertTab
	// CodeEditorOutdent is Shift+Tab and Command or Control+[.
	CodeEditorOutdent
	// CodeEditorIndent is Command or Control+].
	CodeEditorIndent
	// CodeEditorToggleComment is Command or Control+/.
	CodeEditorToggleComment
	// CodeEditorMoveLinesUp and CodeEditorMoveLinesDown are Alt+Up and Alt+Down.
	CodeEditorMoveLinesUp
	CodeEditorMoveLinesDown
	// CodeEditorDuplicateLinesUp and CodeEditorDuplicateLinesDown are
	// Shift+Alt+Up and Shift+Alt+Down.
	CodeEditorDuplicateLinesUp
	CodeEditorDuplicateLinesDown
	// CodeEditorDeleteLines is Shift+Command or Control+K.
	CodeEditorDeleteLines
)

// CodeEditorCommandForKey says which editing command a key press asks for in a
// code editor. Text insertion, deletion, movement and undo stay with the text
// system and TextareaMovementForKey. Pass the key event of the focused editor to
// it, then the command to CodeEditorCommand.Apply. Tab is a focus key to the
// runtime, so an application takes it here, in its key handler, to keep it in
// the editor.
func CodeEditorCommandForKey(event ui.KeyEvent) (CodeEditorCommand, bool) {
	if event.Release {
		return CodeEditorNoCommand, false
	}
	shift := event.Modifiers&ui.ModifierShift != 0
	alt := event.Modifiers&ui.ModifierAlt != 0
	primary := event.Modifiers&(ui.ModifierControl|ui.ModifierMeta) != 0
	switch {
	case event.Key == ui.KeyTab && !primary && !alt:
		if shift {
			return CodeEditorOutdent, true
		}
		return CodeEditorInsertTab, true
	case alt && !primary && event.Key == ui.KeyUp:
		if shift {
			return CodeEditorDuplicateLinesUp, true
		}
		return CodeEditorMoveLinesUp, true
	case alt && !primary && event.Key == ui.KeyDown:
		if shift {
			return CodeEditorDuplicateLinesDown, true
		}
		return CodeEditorMoveLinesDown, true
	case primary && !alt && !shift:
		switch event.Text {
		case "/":
			return CodeEditorToggleComment, true
		case "]":
			return CodeEditorIndent, true
		case "[":
			return CodeEditorOutdent, true
		}
	case primary && !alt && shift && strings.EqualFold(event.Text, "k"):
		return CodeEditorDeleteLines, true
	}
	return CodeEditorNoCommand, false
}

// Apply runs the command on editor, as one undo unit, with the options set by
// text.Editor.SetCode (the defaults when it has none), and reports whether it was
// a command at all.
func (command CodeEditorCommand) Apply(editor *text.Editor) bool {
	options, _ := editor.Code()
	switch command {
	case CodeEditorInsertTab:
		editor.InsertTabCode(options)
	case CodeEditorOutdent:
		editor.OutdentCode(options)
	case CodeEditorIndent:
		editor.IndentLinesCode(options)
	case CodeEditorToggleComment:
		editor.ToggleLineComment(options)
	case CodeEditorMoveLinesUp:
		editor.MoveLines(true)
	case CodeEditorMoveLinesDown:
		editor.MoveLines(false)
	case CodeEditorDuplicateLinesUp:
		editor.DuplicateLines(true)
	case CodeEditorDuplicateLinesDown:
		editor.DuplicateLines(false)
	case CodeEditorDeleteLines:
		editor.DeleteLines()
	default:
		return false
	}
	return true
}
