package components

import "github.com/vivaui/viva/gfx"

// CodeMergeRow aligns one line of each source. A missing line is -1. The
// caller owns this alignment and must update it when the editable result gains
// or loses lines; the component does not interpret or resolve conflicts.
type CodeMergeRow struct {
	CurrentLine, ResultLine, IncomingLine int
	CurrentChanged, IncomingChanged       bool
}

// CodeMergeAction is a caller-owned action on an aligned row of either source.
type CodeMergeAction struct {
	Row      int
	Incoming bool
	ID       gfx.ID
	Label    string
}

// CodeMergeSnapshot describes a three-way aligned comparison. Editor supplies
// the result text and its native caret, selection, composition and hit IDs;
// Editor.Result is ignored. Current and Incoming are the caller's split source
// lines. Action IDs are optional gutter regions; the caller maps their pointer
// coordinates to an aligned row with ActionAt and decides what accepting means.
type CodeMergeSnapshot struct {
	Current, Incoming []string
	Rows              []CodeMergeRow
	Editor            CodeDiffSnapshot
	CurrentActionID   gfx.ID
	IncomingActionID  gfx.ID
	Actions           []CodeMergeAction
}

// CodeMerge shares CodeDiff's grid rendering, native text placement, selection,
// keyboard movement and scrolling, but owns an independent three-pane layout.
// Its center pane is the only editable surface.
type CodeMerge struct {
	CodeDiff
	rows           []CodeMergeRow
	aligned        []CodeDiffRow
	incomingShown  []string
	incomingKey    codeDiffLeftKey
	widthKey       codeMergeWidthKey
	width          float32
	currentAction  gfx.Rect
	incomingAction gfx.Rect
}

type codeMergeWidthKey struct {
	current, incoming       *string
	currentLen, incomingLen int
	value                   string
	tabWidth                int
	cellWidth               float32
}

// Paint draws Current, Result and Incoming on one aligned vertical grid.
func (merge *CodeMerge) Paint(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, snapshot CodeMergeSnapshot) gfx.Rect {
	diff := &merge.CodeDiff
	diff.canvas, diff.theme, diff.snapshot = canvas, theme, snapshot.Editor
	diff.snapshot.Layout = CodeDiffSideBySide
	merge.rows = snapshot.Rows
	diff.caretShown = false
	merge.currentAction, merge.incomingAction = gfx.Rect{}, gfx.Rect{}
	if bounds.Width <= 0 || bounds.Height <= 0 {
		return bounds
	}
	for len(merge.aligned) < len(snapshot.Rows) {
		merge.aligned = append(merge.aligned, CodeDiffRow{})
	}
	merge.aligned = merge.aligned[:len(snapshot.Rows)]
	for index, row := range snapshot.Rows {
		kind := CodeDiffUnchanged
		if row.CurrentChanged || row.IncomingChanged {
			kind = CodeDiffChanged
		}
		merge.aligned[index] = CodeDiffRow{Kind: kind, LeftLine: row.CurrentLine, RightLine: row.ResultLine}
	}
	diff.snapshot.Result = CodeDiffResult{Left: snapshot.Current, Rows: merge.aligned}
	canvas.Border(bounds, 1, theme.Border, theme.Card)
	size := diff.fontSize()
	diff.cellWidth, diff.cellHeight = canvas.MonoCellSize(size, snapshot.Editor.MonoFace)
	if diff.cellWidth <= 0 || diff.cellHeight <= 0 {
		return bounds
	}
	diff.rowHeight = max(diff.cellHeight, float32(int(size*1.5+0.5)))
	diff.tabWidth = codeDiffTabWidth(snapshot.Editor)
	if !diff.valueSet || diff.value != snapshot.Editor.Value || diff.valueTab != diff.tabWidth {
		diff.lines = splitCodeDiffLines(snapshot.Editor.Value, diff.lines[:0])
		diff.lineStarts = codeDiffLineStarts(diff.lines, diff.lineStarts[:0])
		diff.rightShown = expandedLines(diff.lines, diff.tabWidth, diff.rightShown[:0])
		diff.value, diff.valueSet, diff.valueTab = snapshot.Editor.Value, true, diff.tabWidth
	}
	diff.expandLeft()
	incomingKey := codeDiffLeftKey{count: len(snapshot.Incoming), tabWidth: diff.tabWidth, set: true}
	if incomingKey.count > 0 {
		incomingKey.first = &snapshot.Incoming[0]
	}
	if merge.incomingKey != incomingKey {
		merge.incomingShown = expandedLines(snapshot.Incoming, diff.tabWidth, merge.incomingShown[:0])
		merge.incomingKey = incomingKey
	}
	numberCell, _ := canvas.MonoCellSize(theme.Metrics.FontSizeS, snapshot.Editor.MonoFace)
	digits := decimalDigits(max(len(snapshot.Current), len(snapshot.Incoming), len(diff.lines)))
	diff.gutter = max(float32(codeDiffGutterWidth), 17+float32(digits)*numberCell+6)
	if len(snapshot.Actions) > 0 {
		diff.gutter += 80
	}
	widthKey := codeMergeWidthKey{currentLen: len(snapshot.Current), incomingLen: len(snapshot.Incoming), value: snapshot.Editor.Value, tabWidth: diff.tabWidth, cellWidth: diff.cellWidth}
	if widthKey.currentLen > 0 {
		widthKey.current = &snapshot.Current[0]
	}
	if widthKey.incomingLen > 0 {
		widthKey.incoming = &snapshot.Incoming[0]
	}
	if merge.widthKey != widthKey || merge.width == 0 {
		widest := 0
		for _, line := range snapshot.Current {
			widest = max(widest, gridWidth(line, diff.tabWidth))
		}
		for _, line := range snapshot.Incoming {
			widest = max(widest, gridWidth(line, diff.tabWidth))
		}
		for _, line := range diff.lines {
			widest = max(widest, gridWidth(line, diff.tabWidth))
		}
		merge.width = float32(widest) * diff.cellWidth
		merge.widthKey = widthKey
	}
	diff.lineWidth = merge.width
	contentHeight := float32(len(snapshot.Rows)) * diff.rowHeight
	paneWidth := max((bounds.Width-2*codeDiffPaneGap-scrollAreaBarSize)/3, 1)
	diff.gutter = min(diff.gutter, max(paneWidth-codeDiffPadX, 1))
	codeWidth := max(paneWidth-diff.gutter-codeDiffPadX, 1)
	contentWidth := bounds.Width + max(diff.lineWidth-codeWidth+codeDiffPadX, 0)
	area := BeginScrollArea(canvas, theme, bounds, gfx.Size{Width: contentWidth, Height: contentHeight}).
		ReserveScrollbars().Offset(diff.scrollX, diff.scrollY).
		VerticalScrollbar(snapshot.Editor.VerticalScrollbarID, snapshot.Editor.Label, "").
		HorizontalScrollbar(snapshot.Editor.HorizontalScrollbarID, snapshot.Editor.Label, "")
	if !snapshot.Editor.ViewportID.IsZero() {
		area = area.Viewport(snapshot.Editor.ViewportID, snapshot.Editor.Label)
	}
	if snapshot.Editor.Disabled || snapshot.Editor.ReadOnly {
		area = area.Disabled()
	}
	diff.Geometry = area.BeginContent()
	diff.scrollX, diff.scrollY = diff.Geometry.OffsetX, diff.Geometry.OffsetY
	viewport := diff.Geometry.Viewport
	paneWidth = max((viewport.Width-2*codeDiffPaneGap)/3, 1)
	diff.gutter = min(diff.gutter, max(paneWidth-codeDiffPadX, 1))
	currentPane := gfx.Rect{X: viewport.X, Y: viewport.Y, Width: paneWidth, Height: viewport.Height}
	resultPane := gfx.Rect{X: currentPane.X + paneWidth + codeDiffPaneGap, Y: viewport.Y, Width: paneWidth, Height: viewport.Height}
	incomingPane := gfx.Rect{X: resultPane.X + paneWidth + codeDiffPaneGap, Y: viewport.Y,
		Width: max(viewport.X+viewport.Width-(resultPane.X+paneWidth+codeDiffPaneGap), 1), Height: viewport.Height}
	for _, pane := range [...]gfx.Rect{currentPane, resultPane, incomingPane} {
		canvas.Fill(pane, theme.Background)
	}
	diff.rightCode = gfx.Rect{X: resultPane.X + diff.gutter, Y: resultPane.Y,
		Width: max(resultPane.Width-diff.gutter, 1), Height: resultPane.Height}
	diff.visibleRows = max(int(viewport.Height/diff.rowHeight), 0)
	merge.currentAction = gfx.Rect{X: currentPane.X, Y: viewport.Y, Width: min(diff.gutter, currentPane.Width), Height: viewport.Height}
	merge.incomingAction = gfx.Rect{X: incomingPane.X, Y: viewport.Y, Width: min(diff.gutter, incomingPane.Width), Height: viewport.Height}
	currentShown := diff.leftShown
	first := max(int(diff.scrollY/diff.rowHeight), 0)
	for rowIndex := first; rowIndex < len(snapshot.Rows); rowIndex++ {
		y := viewport.Y + float32(rowIndex)*diff.rowHeight - diff.scrollY
		if y >= viewport.Y+viewport.Height {
			break
		}
		row := snapshot.Rows[rowIndex]
		kind := CodeDiffUnchanged
		if row.CurrentChanged {
			kind = CodeDiffChanged
		}
		diff.paintRowSide(canvas, gfx.Rect{X: currentPane.X, Y: y, Width: currentPane.Width, Height: diff.rowHeight},
			currentPane, kind, row.CurrentLine, true, rowIndex)
		kind = CodeDiffUnchanged
		if row.CurrentChanged || row.IncomingChanged {
			kind = CodeDiffChanged
		}
		diff.paintRowSide(canvas, gfx.Rect{X: resultPane.X, Y: y, Width: resultPane.Width, Height: diff.rowHeight},
			resultPane, kind, row.ResultLine, false, rowIndex)
		kind = CodeDiffUnchanged
		if row.IncomingChanged {
			kind = CodeDiffChanged
		}
		diff.snapshot.Result.Left, diff.leftShown = snapshot.Incoming, merge.incomingShown
		diff.paintRowSide(canvas, gfx.Rect{X: incomingPane.X, Y: y, Width: incomingPane.Width, Height: diff.rowHeight},
			incomingPane, kind, row.IncomingLine, true, rowIndex)
		diff.snapshot.Result.Left, diff.leftShown = snapshot.Current, currentShown
	}
	if !snapshot.Editor.ID.IsZero() {
		canvas.HitWithOptions(snapshot.Editor.ID, diff.rightCode, snapshot.Editor.Label, gfx.HitOptions{
			Role: gfx.RoleTextField, Value: snapshot.Editor.Value, Hint: snapshot.Editor.Hint,
			Disabled: snapshot.Editor.Disabled, ReadOnly: snapshot.Editor.ReadOnly, Multiline: true,
			TextBounds: diff.rightCode, TextSize: size, TextColor: gfx.Color{},
			HideNativeOverlays: true, Placement: merge,
		})
	}
	if !snapshot.CurrentActionID.IsZero() {
		canvas.HitWithOptions(snapshot.CurrentActionID, merge.currentAction, snapshot.Editor.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Editor.Disabled})
	}
	if !snapshot.IncomingActionID.IsZero() {
		canvas.HitWithOptions(snapshot.IncomingActionID, merge.incomingAction, snapshot.Editor.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Editor.Disabled})
	}
	for _, action := range snapshot.Actions {
		if action.ID.IsZero() || action.Row < 0 || action.Row >= len(snapshot.Rows) {
			continue
		}
		pane := currentPane
		if action.Incoming {
			pane = incomingPane
		}
		y := pane.Y + float32(action.Row)*diff.rowHeight - diff.scrollY
		if y < pane.Y || y+diff.rowHeight > pane.Y+pane.Height {
			continue
		}
		button := gfx.Rect{X: pane.X + 4, Y: y + 1, Width: min(float32(72), max(pane.Width-8, 1)), Height: max(diff.rowHeight-2, 1)}
		canvas.Border(button, 1, theme.Border, theme.Card)
		canvas.Clip(button)
		canvas.Label(action.Label, button, gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.Foreground})
		canvas.Unclip()
		canvas.HitWithOptions(action.ID, button, action.Label, gfx.HitOptions{Role: gfx.RoleButton, Disabled: snapshot.Editor.Disabled})
	}
	area.EndContent()
	return bounds
}

// GutterAt identifies the row and source line of either action region.
func (merge *CodeMerge) GutterAt(point gfx.Point) (row, line int, incoming, ok bool) {
	row, incoming, ok = merge.ActionAt(point)
	if !ok {
		return 0, 0, false, false
	}
	if incoming {
		return row, merge.incomingLine(row), true, true
	}
	return row, merge.aligned[row].LeftLine, false, true
}

func (merge *CodeMerge) incomingLine(row int) int {
	if row < 0 || row >= len(merge.rows) {
		return -1
	}
	return merge.rows[row].IncomingLine
}

// ActionAt maps a gutter pointer to its aligned row and side. The caller owns
// the action and the meaning of each row; empty action regions return false.
func (merge *CodeMerge) ActionAt(point gfx.Point) (row int, incoming, ok bool) {
	region := merge.currentAction
	if point.X >= merge.incomingAction.X && point.X < merge.incomingAction.X+merge.incomingAction.Width {
		region, incoming = merge.incomingAction, true
	}
	if merge.rowHeight <= 0 || region.Width <= 0 || point.X < region.X || point.X >= region.X+region.Width || point.Y < region.Y || point.Y >= region.Y+region.Height {
		return 0, false, false
	}
	row = int((point.Y - region.Y + merge.Geometry.OffsetY) / merge.rowHeight)
	return row, incoming, row >= 0 && row < len(merge.aligned)
}
