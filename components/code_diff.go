package components

import "github.com/vivaui/viva/gfx"

// CodeDiffLayout selects an aligned two-pane, stacked, or unified comparison.
type CodeDiffLayout uint8

const (
	CodeDiffSideBySide CodeDiffLayout = iota
	CodeDiffStacked
	CodeDiffUnified
)

// CodeDiffGutterAction is a caller-owned button on one aligned source row.
// The caller assigns the ID from its gfx.Tree and handles the resulting click.
type CodeDiffGutterAction struct {
	Row   int
	Left  bool
	ID    gfx.ID
	Label string
}

// CodeDiffSnapshot describes an aligned two-pane source comparison. The left
// side is read-only. The right side reuses the caller's native editor state:
// Value, caret, selection and composition are UTF-8 byte offsets.
type CodeDiffSnapshot struct {
	Result CodeDiffResult
	Layout CodeDiffLayout

	ID       gfx.ID
	Label    string
	Hint     string
	Value    string
	Language string
	// Highlight colors the changed rows, as for a CodeEditor.
	Highlight    func(language, line string, dst []CodeEditorRun) []CodeEditorRun
	AddedColor   gfx.Color
	RemovedColor gfx.Color
	// FontSize is the size of the code, in logical pixels; zero takes the theme's.
	FontSize float32
	// MonoFace is the fixed-pitch font the code is set in.
	MonoFace gfx.MonoFace
	// TabWidth is the width of a tab stop in cells; zero is four.
	TabWidth         int
	Disabled         bool
	ReadOnly         bool
	Focused          bool
	ShowCaret        bool
	Caret            int
	SelectionStart   int
	SelectionEnd     int
	CompositionStart int
	CompositionEnd   int
	// CaretWidth is the caret's thickness in logical pixels; zero uses the
	// two-pixel editor default. CaretColor paints it; zero takes the text color.
	CaretWidth            float32
	CaretColor            gfx.Color
	VerticalScrollbarID   gfx.ID
	HorizontalScrollbarID gfx.ID
	ViewportID            gfx.ID
	// LeftGutterID and RightGutterID declare accessible, clickable gutter
	// regions. GutterAt maps a pointer to the aligned row and source line.
	LeftGutterID, RightGutterID gfx.ID
	Actions                     []CodeDiffGutterAction
}

// CodeDiff paints a line-aligned source comparison on the same grid of
// character cells as a CodeEditor. Its right pane is an input surface whose
// editing state remains owned by the caller. Pointer placement and IME location
// use the same byte offsets as a CodeEditor.
type CodeDiff struct {
	Geometry   ScrollAreaGeometry
	scrollX    float32
	scrollY    float32
	rowHeight  float32
	cellWidth  float32
	cellHeight float32
	lineWidth  float32
	gutter     float32
	tabWidth   int
	// widthKey names the text lineWidth was measured for, so it is measured again only when
	// the comparison, the edited text or the tab width changes, not on every scroll.
	widthKey codeDiffWidthKey

	canvas      *gfx.Canvas
	theme       gfx.Theme
	snapshot    CodeDiffSnapshot
	value       string
	valueSet    bool
	valueTab    int
	lines       []string
	lineStarts  []int
	rightShown  []string
	leftShown   []string
	leftKey     codeDiffLeftKey
	scratchLine []string
	scratchPos  []int
	marks       []CodeEditorRun
	runs        []gfx.TextRun
	numbers     lineNumbers

	rightCode               gfx.Rect
	caret                   gfx.Rect
	caretShown              bool
	goalColumn              int
	goalValid               bool
	verticalDrag            bool
	horizontalDrag          bool
	grab                    float32
	dragX                   float32
	dragY                   float32
	dragScrollX             float32
	dragScrollY             float32
	visibleRows             int
	leftGutter, rightGutter gfx.Rect
	visualRows              []codeDiffVisualRow
	visualKey               codeDiffVisualKey
	inlineRow               int
}

type codeDiffVisualKey struct {
	first *CodeDiffRow
	count int
}

type codeDiffVisualRow struct {
	row, line int
	left      bool
}

type codeDiffWidthKey struct {
	left          *string
	leftLen, rows int
	value         string
	tabWidth      int
	cellWidth     float32
	measured      bool
}

type codeDiffLeftKey struct {
	first    *string
	count    int
	tabWidth int
	set      bool
}

const (
	codeDiffGutterWidth = 48
	codeDiffPaneGap     = 9
	codeDiffPadX        = 8
)

// codeDiffTabWidth is the tab stop of a snapshot.
func codeDiffTabWidth(snapshot CodeDiffSnapshot) int {
	switch {
	case snapshot.TabWidth <= 0:
		return 4
	case snapshot.TabWidth > 16:
		return 16
	}
	return snapshot.TabWidth
}

// Paint renders an aligned comparison inside bounds. Both columns share the
// vertical and horizontal offsets and the two scrollbars.
func (diff *CodeDiff) Paint(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot CodeDiffSnapshot) gfx.Rect {
	diff.canvas, diff.theme, diff.snapshot = canvas, theme, snapshot
	diff.caretShown = false
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}
	canvas.Border(bounds, 1, theme.Border, theme.Card)
	size := snapshot.FontSize
	if size <= 0 {
		size = theme.Metrics.FontSizeM
	}
	diff.cellWidth, diff.cellHeight = canvas.MonoCellSize(size, snapshot.MonoFace)
	if diff.cellWidth <= 0 || diff.cellHeight <= 0 {
		return bounds
	}
	diff.rowHeight = max(diff.cellHeight, float32(int(size*1.5+0.5)))
	diff.tabWidth = codeDiffTabWidth(snapshot)
	if !diff.valueSet || diff.value != snapshot.Value || diff.valueTab != diff.tabWidth {
		diff.lines = splitCodeDiffLines(snapshot.Value, diff.lines[:0])
		diff.lineStarts = codeDiffLineStarts(diff.lines, diff.lineStarts[:0])
		diff.rightShown = expandedLines(diff.lines, diff.tabWidth, diff.rightShown[:0])
		diff.value, diff.valueSet, diff.valueTab = snapshot.Value, true, diff.tabWidth
	}
	diff.expandLeft()
	if snapshot.Layout == CodeDiffUnified {
		key := codeDiffVisualKey{count: len(snapshot.Result.Rows)}
		if key.count > 0 {
			key.first = &snapshot.Result.Rows[0]
		}
		if diff.visualKey != key {
			diff.visualRows = diff.visualRows[:0]
			for index, row := range snapshot.Result.Rows {
				if row.Kind != CodeDiffUnchanged && row.LeftLine >= 0 {
					diff.visualRows = append(diff.visualRows, codeDiffVisualRow{row: index, line: row.LeftLine, left: true})
				}
				if row.RightLine >= 0 {
					diff.visualRows = append(diff.visualRows, codeDiffVisualRow{row: index, line: row.RightLine})
				}
			}
			diff.visualKey = key
		}
	}

	// The gutter is as wide as the numbers it holds need, and no narrower than
	// it always was.
	numberCell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeS, snapshot.MonoFace)
	digits := decimalDigits(max(len(snapshot.Result.Left), len(diff.lines)))
	diff.gutter = max(float32(codeDiffGutterWidth), 17+float32(digits)*numberCell+6)
	if len(snapshot.Actions) > 0 {
		diff.gutter += 80
	}

	effectiveRows := len(snapshot.Result.Rows)
	if snapshot.Layout == CodeDiffUnified {
		effectiveRows = len(diff.visualRows)
	}
	contentHeight := float32(effectiveRows) * diff.rowHeight
	if snapshot.Layout == CodeDiffStacked {
		contentHeight += bounds.Height / 2
	}
	diff.lineWidth = diff.longestLineWidth()
	_, hasVertical := diff.scrollbars(bounds, contentHeight)
	outerWidth := bounds.Width
	if hasVertical {
		outerWidth = max(outerWidth-scrollAreaBarSize, 1)
	}
	half := max((outerWidth-codeDiffPaneGap)/2, 1)
	if snapshot.Layout != CodeDiffSideBySide {
		half = outerWidth
	}
	codeViewportWidth := max(half-diff.gutter-codeDiffPadX, 1)
	contentWidth := outerWidth + max(diff.lineWidth-codeViewportWidth+codeDiffPadX, 0)
	area := BeginScrollArea(canvas, theme, bounds, gfx.Size{Width: contentWidth, Height: contentHeight}).
		ReserveScrollbars().
		Offset(diff.scrollX, diff.scrollY).
		VerticalScrollbar(snapshot.VerticalScrollbarID, snapshot.Label, "").
		HorizontalScrollbar(snapshot.HorizontalScrollbarID, snapshot.Label, "")
	if !snapshot.ViewportID.IsZero() {
		area = area.Viewport(snapshot.ViewportID, snapshot.Label)
	}
	if snapshot.ReadOnly || snapshot.Disabled {
		area = area.Disabled()
	}
	diff.Geometry = area.BeginContent()
	diff.scrollX, diff.scrollY = diff.Geometry.OffsetX, diff.Geometry.OffsetY

	paneTop := diff.Geometry.Viewport.Y
	paneHeight := diff.Geometry.Viewport.Height
	half = max((diff.Geometry.Viewport.Width-codeDiffPaneGap)/2, 1)
	rightLimit := diff.Geometry.Viewport.X + diff.Geometry.Viewport.Width
	leftPane := gfx.Rect{X: diff.Geometry.Viewport.X, Y: paneTop, Width: half, Height: paneHeight}
	rightPane := gfx.Rect{X: bounds.X + half + codeDiffPaneGap, Y: paneTop, Width: max(rightLimit-(bounds.X+half+codeDiffPaneGap), 1), Height: paneHeight}
	if snapshot.Layout == CodeDiffStacked {
		leftPane.Width, leftPane.Height = diff.Geometry.Viewport.Width, max((paneHeight-codeDiffPaneGap)/2, 1)
		rightPane = gfx.Rect{X: leftPane.X, Y: paneTop + leftPane.Height + codeDiffPaneGap,
			Width: leftPane.Width, Height: max(paneHeight-leftPane.Height-codeDiffPaneGap, 1)}
	} else if snapshot.Layout == CodeDiffUnified {
		leftPane, rightPane = diff.Geometry.Viewport, diff.Geometry.Viewport
	}
	canvas.Fill(leftPane, theme.Background)
	if snapshot.Layout != CodeDiffUnified {
		canvas.Fill(rightPane, theme.Background)
		if snapshot.Layout == CodeDiffSideBySide {
			canvas.Fill(gfx.Rect{X: bounds.X + half + codeDiffPaneGap/2, Y: paneTop, Width: 1, Height: paneHeight}, theme.Border)
		}
	}

	diff.rightCode = gfx.Rect{X: rightPane.X + diff.gutter, Y: rightPane.Y, Width: max(rightPane.Width-diff.gutter, 1), Height: rightPane.Height}
	diff.leftGutter = gfx.Rect{X: leftPane.X, Y: leftPane.Y, Width: min(diff.gutter, leftPane.Width), Height: leftPane.Height}
	diff.rightGutter = gfx.Rect{X: rightPane.X, Y: rightPane.Y, Width: min(diff.gutter, rightPane.Width), Height: rightPane.Height}
	if snapshot.Layout == CodeDiffUnified {
		diff.leftGutter = gfx.Rect{}
	}
	diff.visibleRows = max(int(rightPane.Height/diff.rowHeight), 0)
	first := max(int(diff.Geometry.OffsetY/diff.rowHeight), 0)
	for rowIndex := first; rowIndex < effectiveRows; rowIndex++ {
		y := float32(rowIndex)*diff.rowHeight - diff.Geometry.OffsetY
		if y >= rightPane.Height && (snapshot.Layout != CodeDiffStacked || y >= leftPane.Height) {
			break
		}
		if y+diff.rowHeight <= 0 {
			continue
		}
		if snapshot.Layout == CodeDiffUnified {
			visual := diff.visualRows[rowIndex]
			row := snapshot.Result.Rows[visual.row]
			rowBounds := gfx.Rect{X: rightPane.X, Y: rightPane.Y + y, Width: rightPane.Width, Height: diff.rowHeight}
			diff.inlineRow = visual.row
			diff.paintRowSide(canvas, rowBounds, rightPane, row.Kind, visual.line, visual.left, rowIndex)
			continue
		}
		rowBounds := gfx.Rect{Y: y, Width: bounds.Width, Height: diff.rowHeight}
		rowBounds.X, rowBounds.Y = leftPane.X, leftPane.Y+y
		rowBounds.Width = leftPane.Width
		kind, left, right := diff.rowAt(rowIndex)
		diff.inlineRow = rowIndex
		if snapshot.Layout == CodeDiffStacked {
			canvas.Clip(leftPane)
		}
		diff.paintRowSide(canvas, rowBounds, leftPane, kind, left, true, rowIndex)
		if snapshot.Layout == CodeDiffStacked {
			canvas.Unclip()
		}
		rowBounds.X, rowBounds.Y, rowBounds.Width = rightPane.X, rightPane.Y+y, rightPane.Width
		if snapshot.Layout == CodeDiffStacked {
			canvas.Clip(rightPane)
		}
		diff.paintRowSide(canvas, rowBounds, rightPane, kind, right, false, rowIndex)
		if snapshot.Layout == CodeDiffStacked {
			canvas.Unclip()
		}
	}

	if !snapshot.ID.IsZero() {
		canvas.HitWithOptions(snapshot.ID, diff.rightCode, snapshot.Label, gfx.HitOptions{
			Role: gfx.RoleTextField, Value: snapshot.Value, Hint: snapshot.Hint,
			Disabled: snapshot.Disabled, ReadOnly: snapshot.ReadOnly, Multiline: true,
			TextBounds: diff.rightCode, TextSize: size, TextColor: gfx.Color{},
			HideNativeOverlays: true,
			Placement:          diff,
		})
	}
	if !snapshot.LeftGutterID.IsZero() && snapshot.Layout != CodeDiffUnified {
		canvas.HitWithOptions(snapshot.LeftGutterID, diff.leftGutter, snapshot.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Disabled})
	}
	if !snapshot.RightGutterID.IsZero() {
		canvas.HitWithOptions(snapshot.RightGutterID, diff.rightGutter, snapshot.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Disabled})
	}
	diff.paintActions(canvas, theme, leftPane, rightPane, snapshot)
	area.EndContent()
	return bounds
}

func (diff *CodeDiff) paintActions(canvas *gfx.Canvas, theme gfx.Theme, leftPane, rightPane gfx.Rect, snapshot CodeDiffSnapshot) {
	for _, action := range snapshot.Actions {
		if action.ID.IsZero() || action.Row < 0 || action.Row >= len(snapshot.Result.Rows) {
			continue
		}
		pane := rightPane
		visualRow := action.Row
		if action.Left {
			pane = leftPane
		}
		if snapshot.Layout == CodeDiffUnified {
			visualRow = diff.visualIndex(action.Row, action.Left)
		}
		if visualRow < 0 {
			continue
		}
		y := pane.Y + float32(visualRow)*diff.rowHeight - diff.scrollY
		if y < pane.Y || y+diff.rowHeight > pane.Y+pane.Height {
			continue
		}
		button := gfx.Rect{X: pane.X + 4, Y: y + 1, Width: min(float32(72), max(pane.Width-8, 1)), Height: max(diff.rowHeight-2, 1)}
		canvas.Border(button, 1, theme.Border, theme.Card)
		canvas.Clip(button)
		canvas.Label(action.Label, button, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.Foreground})
		canvas.Unclip()
		canvas.HitWithOptions(action.ID, button, action.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Disabled})
	}
}

// expandedLines appends to dst each line with its tabs expanded.
func expandedLines(lines []string, tabWidth int, dst []string) []string {
	for _, line := range lines {
		dst = append(dst, gridExpand(line, tabWidth))
	}
	return dst
}

// expandLeft keeps the left source's lines with their tabs expanded, for as long
// as the source and the tab width are the ones they were made for.
func (diff *CodeDiff) expandLeft() {
	key := codeDiffLeftKey{count: len(diff.snapshot.Result.Left), tabWidth: diff.tabWidth, set: true}
	if key.count > 0 {
		key.first = &diff.snapshot.Result.Left[0]
	}
	if diff.leftKey == key {
		return
	}
	diff.leftKey = key
	diff.leftShown = expandedLines(diff.snapshot.Result.Left, diff.tabWidth, diff.leftShown[:0])
}

// longestLineWidth is the width of the widest line of either pane, over the whole file, so the
// horizontal scroll range does not change with the vertical position.
func (diff *CodeDiff) longestLineWidth() float32 {
	key := codeDiffWidthKey{leftLen: len(diff.snapshot.Result.Left), rows: len(diff.snapshot.Result.Rows), value: diff.value, tabWidth: diff.tabWidth, cellWidth: diff.cellWidth, measured: true}
	if key.leftLen > 0 {
		key.left = &diff.snapshot.Result.Left[0]
	}
	if diff.widthKey == key {
		return diff.lineWidth
	}
	diff.widthKey = key
	widest := 0
	for _, line := range diff.snapshot.Result.Left {
		widest = max(widest, gridWidth(line, diff.tabWidth))
	}
	for _, line := range diff.lines {
		widest = max(widest, gridWidth(line, diff.tabWidth))
	}
	return float32(widest) * diff.cellWidth
}

func (diff *CodeDiff) rowAt(index int) (CodeDiffKind, int, int) {
	if index >= 0 && index < len(diff.snapshot.Result.Rows) {
		row := diff.snapshot.Result.Rows[index]
		return row.Kind, row.LeftLine, row.RightLine
	}
	return CodeDiffUnchanged, -1, -1
}

func (diff *CodeDiff) rowLines(index int) (int, int) {
	if diff.snapshot.Layout == CodeDiffUnified {
		if index < 0 || index >= len(diff.visualRows) {
			return -1, -1
		}
		visual := diff.visualRows[index]
		if visual.left {
			return visual.line, -1
		}
		return -1, visual.line
	}
	if index < 0 || index >= len(diff.snapshot.Result.Rows) {
		return -1, -1
	}
	row := diff.snapshot.Result.Rows[index]
	return row.LeftLine, row.RightLine
}

func (diff *CodeDiff) paintRowSide(canvas *gfx.Canvas, rowBounds, pane gfx.Rect, kind CodeDiffKind, line int, left bool, rowIndex int) {
	changed := kind != CodeDiffUnchanged
	countsSide := left && (kind == CodeDiffRemoved || kind == CodeDiffChanged) ||
		!left && (kind == CodeDiffAdded || kind == CodeDiffChanged)
	if changed && countsSide {
		color := diff.snapshot.RemovedColor
		sign := "−"
		if !left {
			color = diff.snapshot.AddedColor
			sign = "+"
		}
		if color == (gfx.Color{}) {
			if left {
				color = diff.theme.Danger
			} else {
				color = diff.theme.Success
			}
		}
		background := color
		background.Alpha *= .13
		canvas.Fill(rowBounds, background)
		gutter := color
		gutter.Alpha *= .85
		canvas.Fill(gfx.Rect{X: rowBounds.X, Y: rowBounds.Y, Width: 2, Height: rowBounds.Height}, gutter)
		canvas.Label(sign, gfx.Rect{X: rowBounds.X + 4, Y: rowBounds.Y, Width: 14, Height: rowBounds.Height}, gfx.TextStyle{
			Size: diff.theme.Metrics.FontSizeS, Weight: gfx.WeightSemibold, Color: color, Mono: true,
		})
	}
	if line >= 0 {
		diff.paintLineNumber(canvas, pane, rowBounds, line)
	}
	var value, shown string
	if left {
		if line >= 0 && line < len(diff.snapshot.Result.Left) {
			value, shown = diff.snapshot.Result.Left[line], diff.leftShown[line]
		}
	} else if line >= 0 && line < len(diff.lines) {
		value, shown = diff.lines[line], diff.rightShown[line]
	}
	code := gfx.Rect{X: pane.X + diff.gutter, Y: rowBounds.Y, Width: max(pane.Width-diff.gutter-codeDiffPadX, 1), Height: rowBounds.Height}
	canvas.Clip(code)
	origin := code.X - diff.Geometry.OffsetX + codeDiffPadX // where column 0 of the line is
	lineStart := 0
	selectionStart, selectionEnd := 0, 0
	if !left && line >= 0 && line < len(diff.lines) {
		lineStart = diff.lineStarts[line]
		selectionStart = diff.snapshot.SelectionStart - lineStart
		selectionEnd = diff.snapshot.SelectionEnd - lineStart
	}
	if kind == CodeDiffChanged && line >= 0 && (!left || line < len(diff.snapshot.Result.Left)) {
		if left || line < len(diff.snapshot.Result.Right) && value == diff.snapshot.Result.Right[line] {
			diff.paintInline(canvas, diff.inlineRow, value, left, origin, code)
		}
	}
	if !left && line >= 0 && line < len(diff.lines) {
		diff.paintRightSelection(canvas, line, origin, rowBounds)
	}
	if value != "" {
		spec := gridRunSpec{
			tabWidth: diff.tabWidth, foreground: diff.theme.Foreground,
			firstColumn:    int(max(diff.Geometry.OffsetX, 0) / diff.cellWidth),
			selectionStart: selectionStart, selectionEnd: selectionEnd,
		}
		spec.lastColumn = spec.firstColumn + int(code.Width/diff.cellWidth) + 3
		_, spec.selectionForeground = selectionColors(diff.theme)
		if selectionEnd <= selectionStart {
			spec.selectionStart, spec.selectionEnd = 0, 0
		}
		if changed && diff.snapshot.Highlight != nil && len(value) <= CodeEditorMaxHighlightedLine {
			spec.highlight = cleanHighlight(value, diff.snapshot.Highlight(diff.snapshot.Language, value, diff.marks[:0]))
			diff.marks = spec.highlight[:0]
		}
		runs, from, to, startColumn, endColumn := gridRuns(diff.runs[:0], value, &spec)
		diff.runs = runs
		if len(runs) > 0 {
			bounds := gfx.Rect{
				X: origin + float32(startColumn)*diff.cellWidth, Y: rowBounds.Y + (rowBounds.Height-diff.cellHeight)/2,
				Width: float32(max(endColumn-startColumn, 1)) * diff.cellWidth, Height: diff.cellHeight,
			}
			canvas.LabelRuns(shown[from:to], bounds, gfx.TextStyle{Size: diff.fontSize(), Color: diff.theme.Foreground, Mono: true, MonoFace: diff.snapshot.MonoFace}, diff.cellWidth, runs)
		}
	}
	if !left {
		diff.paintRightComposition(canvas, line, origin, rowBounds)
		diff.paintRightCaret(canvas, line, origin, rowBounds, rowIndex)
	}
	canvas.Unclip()
}

func (diff *CodeDiff) paintInline(canvas *gfx.Canvas, row int, value string, left bool, origin float32, bounds gfx.Rect) {
	changes := diff.snapshot.Result.Inline
	low, high := 0, len(changes)
	for low < high {
		middle := (low + high) / 2
		if changes[middle].Row < row {
			low = middle + 1
		} else {
			high = middle
		}
	}
	for _, change := range changes[low:] {
		if change.Row > row {
			break
		}
		start, end := change.LeftStart, change.LeftEnd
		color := diff.snapshot.RemovedColor
		if !left {
			start, end = change.RightStart, change.RightEnd
			color = diff.snapshot.AddedColor
		}
		if start >= end || end > len(value) {
			continue
		}
		if color == (gfx.Color{}) {
			if left {
				color = diff.theme.Danger
			} else {
				color = diff.theme.Success
			}
		}
		color.Alpha *= .28
		first := gridWidth(value[:start], diff.tabWidth)
		last := gridWidth(value[:end], diff.tabWidth)
		startX := max(origin+float32(first)*diff.cellWidth, bounds.X)
		endX := min(origin+float32(max(last, first+1))*diff.cellWidth, bounds.X+bounds.Width)
		if endX > startX {
			canvas.Fill(gfx.Rect{X: startX, Y: bounds.Y, Width: endX - startX, Height: bounds.Height}, color)
		}
	}
}

func (diff *CodeDiff) visualIndex(row int, left bool) int {
	low, high := 0, len(diff.visualRows)
	for low < high {
		middle := (low + high) / 2
		if diff.visualRows[middle].row < row {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low < len(diff.visualRows) && diff.visualRows[low].row == row && diff.visualRows[low].left == left {
		return low
	}
	if low+1 < len(diff.visualRows) && diff.visualRows[low+1].row == row && diff.visualRows[low+1].left == left {
		return low + 1
	}
	return -1
}

// GutterAt identifies the aligned row and source line at point. A missing line
// on one side is returned as -1; callers can use Hunks for row-based actions.
func (diff *CodeDiff) GutterAt(point gfx.Point) (row, line int, left, ok bool) {
	side := diff.rightGutter
	if point.X >= diff.leftGutter.X && point.X < diff.leftGutter.X+diff.leftGutter.Width {
		left, side = true, diff.leftGutter
	}
	if diff.rowHeight <= 0 || point.X < side.X || point.X >= side.X+side.Width || point.Y < side.Y || point.Y >= side.Y+side.Height {
		return 0, 0, false, false
	}
	row = int((point.Y - side.Y + diff.Geometry.OffsetY) / diff.rowHeight)
	if diff.snapshot.Layout == CodeDiffUnified {
		if row < 0 || row >= len(diff.visualRows) {
			return 0, 0, false, false
		}
		visual := diff.visualRows[row]
		return visual.row, visual.line, visual.left, true
	}
	if row < 0 || row >= len(diff.snapshot.Result.Rows) {
		return 0, 0, false, false
	}
	_, line = diff.rowLines(row)
	if left {
		line, _ = diff.rowLines(row)
	}
	return row, line, left, true
}

// RevealRow scrolls an aligned row into the shared viewport without changing
// the horizontal position. It is suitable for navigating to a hunk's Start.
func (diff *CodeDiff) RevealRow(row int) {
	if row < 0 || row >= len(diff.snapshot.Result.Rows) || diff.rowHeight <= 0 {
		return
	}
	if diff.snapshot.Layout == CodeDiffUnified {
		visualRow := diff.visualIndex(row, true)
		if visualRow < 0 {
			visualRow = diff.visualIndex(row, false)
		}
		if visualRow < 0 {
			return
		}
		row = visualRow
	}
	top := float32(row) * diff.rowHeight
	height := diff.Geometry.Viewport.Height
	if diff.snapshot.Layout == CodeDiffStacked {
		height = diff.rightCode.Height
	}
	if top < diff.scrollY {
		diff.scrollY = top
	} else if bottom := top + diff.rowHeight; bottom > diff.scrollY+height {
		diff.scrollY = bottom - height
	}
	diff.clampScroll()
}

func (diff *CodeDiff) fontSize() float32 {
	if diff.snapshot.FontSize > 0 {
		return diff.snapshot.FontSize
	}
	return diff.theme.Metrics.FontSizeM
}

// paintLineNumber draws the number of line at the right of the pane's gutter.
func (diff *CodeDiff) paintLineNumber(canvas *gfx.Canvas, pane, rowBounds gfx.Rect, line int) {
	number := diff.numbers.text(line)
	size := diff.theme.Metrics.FontSizeS
	cell, height := canvas.MonoCellSize(size, diff.snapshot.MonoFace)
	width := float32(len(number)) * cell
	bounds := gfx.Rect{X: pane.X + diff.gutter - 6 - width, Y: rowBounds.Y + (rowBounds.Height-height)/2, Width: width, Height: height}
	runs := [1]gfx.TextRun{{Bytes: int32(len(number)), Cells: int32(len(number)), Color: diff.theme.MutedForeground}}
	canvas.LabelRuns(number, bounds, gfx.TextStyle{Size: size, Color: diff.theme.MutedForeground, Mono: true, MonoFace: diff.snapshot.MonoFace}, cell, runs[:])
}

// rightRange is the part of the selection or composition [start, end) that lies
// on right line line, as columns of the line, and false if none of it does.
func (diff *CodeDiff) rightRange(line, start, end int) (first, last int, ok bool) {
	if line < 0 || line >= len(diff.lines) {
		return 0, 0, false
	}
	lineStart := diff.lineStarts[line]
	lineEnd := lineStart + len(diff.lines[line])
	start, end = max(start, lineStart), min(end, lineEnd)
	if start >= end {
		return 0, 0, false
	}
	text := diff.lines[line]
	return gridColumn(text, start-lineStart, diff.tabWidth), gridColumn(text, end-lineStart, diff.tabWidth), true
}

func (diff *CodeDiff) paintRightSelection(canvas *gfx.Canvas, line int, origin float32, row gfx.Rect) {
	first, last, ok := diff.rightRange(line, diff.snapshot.SelectionStart, diff.snapshot.SelectionEnd)
	if !ok {
		return
	}
	background, _ := selectionColors(diff.theme)
	extra := float32(0)
	if end := diff.lineStarts[line] + len(diff.lines[line]); diff.snapshot.SelectionEnd > end && line < len(diff.lines)-1 {
		extra = diff.cellWidth / 2 // the line break is selected too
	}
	canvas.Fill(gfx.Rect{X: origin + float32(first)*diff.cellWidth, Y: row.Y + 1, Width: float32(last-first)*diff.cellWidth + extra, Height: row.Height - 2}, background)
}

func (diff *CodeDiff) paintRightComposition(canvas *gfx.Canvas, line int, origin float32, row gfx.Rect) {
	first, last, ok := diff.rightRange(line, diff.snapshot.CompositionStart, diff.snapshot.CompositionEnd)
	if !ok {
		return
	}
	canvas.Fill(gfx.Rect{X: origin + float32(first)*diff.cellWidth, Y: row.Y + row.Height - 3, Width: float32(last-first) * diff.cellWidth, Height: 1}, diff.theme.Foreground)
}

func (diff *CodeDiff) paintRightCaret(canvas *gfx.Canvas, line int, origin float32, row gfx.Rect, rowIndex int) {
	focused := !diff.snapshot.Disabled && (diff.snapshot.Focused || !diff.snapshot.ID.IsZero() && canvas.Focused() == diff.snapshot.ID)
	if !focused || !diff.snapshot.ShowCaret || line < 0 || line >= len(diff.lines) {
		return
	}
	caret := diff.snapshot.Caret
	lineStart := diff.lineStarts[line]
	lineEnd := lineStart + len(diff.lines[line])
	if caret < lineStart || caret > lineEnd {
		return
	}
	column := gridColumn(diff.lines[line], caret-lineStart, diff.tabWidth)
	x := origin + float32(column)*diff.cellWidth
	caretWidth := editorCaretWidth(diff.snapshot.CaretWidth)
	tint := caretColor(diff.snapshot.CaretColor, diff.theme.Foreground)
	canvas.Fill(gfx.Rect{X: x, Y: row.Y + (row.Height-diff.cellHeight)/2, Width: caretWidth, Height: diff.cellHeight}, tint)
	diff.caret = gfx.Rect{X: x, Y: row.Y, Width: caretWidth, Height: row.Height}
	diff.caretShown = int(diff.Geometry.OffsetY/diff.rowHeight) <= rowIndex &&
		rowIndex < int(diff.Geometry.OffsetY/diff.rowHeight)+diff.visibleRows
}

// OffsetAt implements gfx.TextPlacement.
func (diff *CodeDiff) OffsetAt(point gfx.Point) (int, bool) {
	if diff.canvas == nil || len(diff.lines) == 0 || diff.rightCode.Width <= 0 || diff.cellWidth <= 0 {
		return 0, false
	}
	if point.X < diff.rightCode.X {
		point.X = diff.rightCode.X
	}
	if point.X > diff.rightCode.X+diff.rightCode.Width {
		point.X = diff.rightCode.X + diff.rightCode.Width
	}
	rows := len(diff.snapshot.Result.Rows)
	if diff.snapshot.Layout == CodeDiffUnified {
		rows = len(diff.visualRows)
	}
	row := int((point.Y - diff.rightCode.Y + diff.Geometry.OffsetY) / diff.rowHeight)
	row = max(0, min(row, max(rows-1, 0)))
	line := diff.rightLineNear(row, rows)
	if line < 0 {
		return 0, false
	}
	x := (point.X - diff.rightCode.X - codeDiffPadX + diff.Geometry.OffsetX) / diff.cellWidth
	return diff.lineStarts[line] + gridOffset(diff.lines[line], x, diff.tabWidth), true
}

// CaretBounds implements gfx.TextPlacement.
func (diff *CodeDiff) CaretBounds() (gfx.Rect, bool) {
	return diff.caret, diff.caretShown
}

func (diff *CodeDiff) rightLineNear(row, rows int) int {
	for candidate := row; candidate < rows; candidate++ {
		_, line := diff.rowLines(candidate)
		if line >= 0 && line < len(diff.lines) {
			return line
		}
	}
	for candidate := row - 1; candidate >= 0; candidate-- {
		_, line := diff.rowLines(candidate)
		if line >= 0 && line < len(diff.lines) {
			return line
		}
	}
	return -1
}

// Move maps a multiline movement key to a UTF-8 byte offset on the editable
// right source. The application remains responsible for applying the returned
// offset to its text.Editor. Up and Down keep the column a run of them began at;
// Home goes to the first character of the line and then to its start.
func (diff *CodeDiff) Move(value string, caret int, movement TextareaMovement) int {
	lines, starts := diff.linesForValue(value)
	caret = textareaBoundary(value, caret)
	line, local := codeDiffLineAt(starts, lines, caret)
	switch movement {
	case TextareaMoveTextStart:
		diff.goalValid = false
		return 0
	case TextareaMoveTextEnd:
		diff.goalValid = false
		return len(value)
	case TextareaMoveLineStart:
		diff.goalValid = false
		first := 0
		for first < len(lines[line]) && (lines[line][first] == ' ' || lines[line][first] == '\t') {
			first++
		}
		if local == first {
			first = 0
		}
		return starts[line] + first
	case TextareaMoveLineEnd:
		diff.goalValid = false
		return starts[line] + len(lines[line])
	case TextareaMoveUp, TextareaMoveDown, TextareaMovePageUp, TextareaMovePageDown:
	default:
		return caret
	}
	step := 1
	if movement == TextareaMovePageUp || movement == TextareaMovePageDown {
		step = max(diff.visibleRows-1, 1)
	}
	if movement == TextareaMoveUp || movement == TextareaMovePageUp {
		step = -step
	}
	tab := diff.tabWidth
	if tab <= 0 {
		tab = codeDiffTabWidth(diff.snapshot)
	}
	if !diff.goalValid {
		diff.goalColumn = gridColumn(lines[line], local, tab)
		diff.goalValid = true
	}
	target := max(0, min(line+step, len(lines)-1))
	return starts[target] + gridOffset(lines[target], float32(diff.goalColumn), tab)
}

// RevealCaret scrolls the shared viewport enough to keep caret visible after a
// caller applies a keyboard movement to its native text.Editor.
func (diff *CodeDiff) RevealCaret(value string, caret int) {
	lines, starts := diff.linesForValue(value)
	caret = textareaBoundary(value, caret)
	line, local := codeDiffLineAt(starts, lines, caret)
	rows := len(diff.snapshot.Result.Rows)
	if diff.snapshot.Layout == CodeDiffUnified {
		rows = len(diff.visualRows)
	}
	row := -1
	for candidate := 0; candidate < rows; candidate++ {
		_, right := diff.rowLines(candidate)
		if right == line {
			row = candidate
			break
		}
	}
	if row < 0 || diff.rowHeight <= 0 {
		return
	}
	top, bottom := float32(row)*diff.rowHeight, float32(row+1)*diff.rowHeight
	height := max(diff.rightCode.Height, diff.rowHeight)
	if top < diff.scrollY {
		diff.scrollY = top
	} else if bottom > diff.scrollY+height {
		diff.scrollY = bottom - height
	}
	if diff.rightCode.Width > 0 && diff.cellWidth > 0 {
		x := float32(gridColumn(lines[line], local, diff.tabWidth))*diff.cellWidth + codeDiffPadX
		width := diff.rightCode.Width
		if x < diff.scrollX {
			diff.scrollX = max(x-codeDiffPadX, 0)
		} else if x > diff.scrollX+width-codeDiffPadX {
			diff.scrollX = x - width + codeDiffPadX
		}
	}
	diff.clampScroll()
}

func (diff *CodeDiff) linesForValue(value string) ([]string, []int) {
	if diff.valueSet && value == diff.value {
		return diff.lines, diff.lineStarts
	}
	diff.scratchLine = splitCodeDiffLines(value, diff.scratchLine)
	diff.scratchPos = codeDiffLineStarts(diff.scratchLine, diff.scratchPos)
	return diff.scratchLine, diff.scratchPos
}

// Scroll applies a wheel or touchpad scroll to both panes. Horizontal deltas use
// horizontal movement; Shift turns vertical delta into horizontal movement.
func (diff *CodeDiff) Scroll(event gfx.ScrollEvent) gfx.EventResult {
	diff.scrollX += event.DeltaX
	if event.Modifiers&gfx.ModifierShift != 0 {
		diff.scrollX += event.DeltaY
	} else {
		diff.scrollY += event.DeltaY
	}
	diff.clampScroll()
	return gfx.EventResult{Handled: true, Repaint: true}
}

// ScrollBy moves the shared viewport.
func (diff *CodeDiff) ScrollBy(dx, dy float32) {
	diff.scrollX += dx
	diff.scrollY += dy
	diff.clampScroll()
}

// ScrollTo sets the shared viewport offsets.
func (diff *CodeDiff) ScrollTo(x, y float32) {
	diff.scrollX, diff.scrollY = x, y
	diff.clampScroll()
}

func (diff *CodeDiff) clampScroll() {
	diff.scrollX = max(0, min(diff.scrollX, diff.Geometry.MaxOffsetX))
	diff.scrollY = max(0, min(diff.scrollY, diff.Geometry.MaxOffsetY))
}

// Pointer handles the two scrollbar drags. Text selection and caret placement
// remain native through the hit region and TextPlacement.
func (diff *CodeDiff) Pointer(event gfx.PointerEvent) gfx.EventResult {
	if diff.snapshot.Disabled {
		return gfx.EventResult{}
	}
	switch event.Kind {
	case gfx.PointerDown:
		diff.verticalDrag = event.Target == diff.snapshot.VerticalScrollbarID &&
			!diff.snapshot.VerticalScrollbarID.IsZero() && diff.Geometry.MaxOffsetY > 0
		diff.horizontalDrag = event.Target == diff.snapshot.HorizontalScrollbarID &&
			!diff.snapshot.HorizontalScrollbarID.IsZero() && diff.Geometry.MaxOffsetX > 0
		if !diff.verticalDrag && !diff.horizontalDrag {
			return gfx.EventResult{}
		}
		diff.grabScrollThumb(event)
	case gfx.PointerMove:
		if !diff.verticalDrag && !diff.horizontalDrag {
			return gfx.EventResult{}
		}
		diff.moveScrollThumb(event)
	default:
		if !diff.verticalDrag && !diff.horizontalDrag {
			return gfx.EventResult{}
		}
		diff.verticalDrag, diff.horizontalDrag = false, false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

func (diff *CodeDiff) grabScrollThumb(event gfx.PointerEvent) {
	if diff.verticalDrag {
		thumb := diff.Geometry.VerticalThumb
		diff.grab = thumb.Height / 2
		if event.Y >= thumb.Y && event.Y < thumb.Y+thumb.Height {
			diff.grab = event.Y - thumb.Y
		}
		diff.scrollY = diff.Geometry.VerticalOffsetForThumb(event.Y - diff.grab)
		diff.dragX, diff.dragY = event.X, event.Y
		diff.dragScrollX, diff.dragScrollY = diff.scrollX, diff.scrollY
		return
	}
	thumb := diff.Geometry.HorizontalThumb
	diff.grab = thumb.Width / 2
	if event.X >= thumb.X && event.X < thumb.X+thumb.Width {
		diff.grab = event.X - thumb.X
	}
	diff.scrollX = diff.Geometry.HorizontalOffsetForThumb(event.X - diff.grab)
	diff.dragX, diff.dragY = event.X, event.Y
	diff.dragScrollX, diff.dragScrollY = diff.scrollX, diff.scrollY
}

func (diff *CodeDiff) moveScrollThumb(event gfx.PointerEvent) {
	if diff.verticalDrag {
		travel := max(diff.Geometry.VerticalTrack.Height-scrollAreaBarInset*2-diff.Geometry.VerticalThumb.Height, 1)
		delta := event.Y - diff.dragY
		diff.scrollY = diff.dragScrollY + delta/travel*diff.Geometry.MaxOffsetY
	} else {
		travel := max(diff.Geometry.HorizontalTrack.Width-scrollAreaBarInset*2-diff.Geometry.HorizontalThumb.Width, 1)
		delta := event.X - diff.dragX
		diff.scrollX = diff.dragScrollX + delta/travel*diff.Geometry.MaxOffsetX
	}
	diff.clampScroll()
}

// ScrollForKey applies keyboard scrolling to the shared viewport.
func (diff *CodeDiff) ScrollForKey(key gfx.Key, extend bool) (gfx.EventResult, bool) {
	_ = extend
	step := diff.rowHeight * 2
	page := max(float32(diff.visibleRows)*diff.rowHeight*.9, step)
	switch key {
	case gfx.KeyLeft:
		diff.scrollX -= step
	case gfx.KeyRight:
		diff.scrollX += step
	case gfx.KeyUp:
		diff.scrollY -= step
	case gfx.KeyDown:
		diff.scrollY += step
	case gfx.KeyPageUp:
		diff.scrollY -= page
	case gfx.KeyPageDown:
		diff.scrollY += page
	case gfx.KeyHome:
		diff.scrollY = 0
	case gfx.KeyEnd:
		diff.scrollY = diff.Geometry.MaxOffsetY
	default:
		return gfx.EventResult{}, false
	}
	diff.clampScroll()
	return gfx.EventResult{Handled: true, Repaint: true}, true
}

// scrollbars reports which scrollbars a comparison in bounds needs, given the
// height of its rows, for the widest line and the gutter of the last paint.
func (diff *CodeDiff) scrollbars(bounds gfx.Rect, contentHeight float32) (horizontal, vertical bool) {
	vertical = contentHeight > bounds.Height
	for range 3 {
		outerWidth := bounds.Width
		if vertical {
			outerWidth = max(outerWidth-scrollAreaBarSize, 1)
		}
		paneWidth := max((outerWidth-codeDiffPaneGap)/2, 1)
		if diff.snapshot.Layout != CodeDiffSideBySide {
			paneWidth = outerWidth
		}
		codeWidth := max(paneWidth-diff.gutter-codeDiffPadX, 1)
		nextHorizontal := diff.lineWidth+codeDiffPadX > codeWidth
		availableHeight := bounds.Height
		if nextHorizontal {
			availableHeight = max(availableHeight-scrollAreaBarSize, 1)
		}
		nextVertical := contentHeight > availableHeight
		if nextHorizontal == horizontal && nextVertical == vertical {
			break
		}
		horizontal, vertical = nextHorizontal, nextVertical
	}
	return horizontal, vertical
}

func splitCodeDiffLines(value string, dst []string) []string {
	dst = dst[:0]
	start := 0
	for index := 0; index < len(value); index++ {
		if value[index] == '\n' {
			dst = append(dst, value[start:index])
			start = index + 1
		}
	}
	return append(dst, value[start:])
}

func codeDiffLineStarts(lines []string, dst []int) []int {
	dst = dst[:0]
	offset := 0
	for _, line := range lines {
		dst = append(dst, offset)
		offset += len(line) + 1
	}
	return dst
}

func codeDiffLineAt(starts []int, lines []string, offset int) (int, int) {
	for index := len(starts) - 1; index >= 0; index-- {
		if offset >= starts[index] {
			return index, min(max(offset-starts[index], 0), len(lines[index]))
		}
	}
	return 0, 0
}
