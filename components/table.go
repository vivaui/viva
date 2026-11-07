package components

import (
	"math"
	"sort"
	"strings"

	"github.com/vivaui/viva/ui"
)

// TableAlignment selects text placement within a table column.
type TableAlignment uint8

const (
	TableAlignLeft TableAlignment = iota
	TableAlignCenter
	TableAlignRight
)

// TableColumn is a statically typed column declaration. Width is measured in
// logical pixels; the final non-positive width consumes the remaining space.
type TableColumn struct {
	Width     float32
	Alignment TableAlignment
	// Inset reserves leading content space inside a left-aligned cell, for
	// example an application-painted file icon.
	Inset float32
}

// TableSelection is an optional text selection for a Table. Anchor and Focus
// are UTF-8 byte offsets into the serialized table text. Cells are represented
// as tab-separated columns and newline-separated rows; cells that contain tab,
// newline, carriage return or quote use quoted TSV fields. The zero value leaves
// text selection disabled and preserves the table's existing paint path.
type TableSelection struct {
	Enabled       bool
	Anchor, Focus int
}

// Range orders and clamps the endpoints against text.
func (selection TableSelection) Range(text []byte) (start, end int) {
	start, end = selection.Anchor, selection.Focus
	if start > end {
		start, end = end, start
	}
	start = tableTextBoundary(text, min(max(start, 0), len(text)))
	end = tableTextBoundary(text, min(max(end, 0), len(text)))
	return start, end
}

func tableTextBoundary(text []byte, offset int) int {
	for offset > 0 && offset < len(text) && text[offset]&0xc0 == 0x80 {
		offset--
	}
	return offset
}

// Table incrementally paints headers and rows. It draws no outer surface or
// border, like shadcn's Table: wrap it in a Card or a bordered container for
// that. Columns are borrowed only while a Header or Row call is executing.
type Table struct {
	canvas       *ui.Canvas
	theme        ui.Theme
	bounds       ui.Rect
	columns      []TableColumn
	y            float32
	rowHeight    float32
	headerHeight float32
	fontSize     float32
	cellPadding  float32
	headerSet    bool
	Selection    TableSelection
	text         *tableTextState
}

type tableTextState struct {
	plain          []byte
	rows           []tableTextRow
	cells          []tableTextCell
	selectionSpans []ui.TextSpan
}

type tableTextCell struct {
	bounds ui.Rect
	// textLo/textHi cover the serialized TSV field, including quotes. Paint
	// selections against contentLo/contentHi so quote bytes never produce a
	// phantom highlight in the visible cell.
	textLo    int
	textHi    int
	contentLo int
	contentHi int
	value     string
	weight    float32
	textX     float32
	textWidth float32
}

type tableTextRow struct {
	bounds ui.Rect
	lo, hi int
}

// TableRowState mirrors selected and expanded row data states. Expanded rows
// use the same muted treatment as the official has-aria-expanded rule.
type TableRowState struct {
	Selected bool
	Expanded bool
}

// TableRowLayout is the exact geometry painted for one row. Cells aliases
// caller-provided storage and is valid until that storage is reused.
type TableRowLayout struct {
	Bounds ui.Rect
	Cells  []ui.Rect
}

// TableRowHeight is the default height of a body or footer row: TableCell's p-2
// around one 20px line, scaled to the theme's control height.
func TableRowHeight(theme ui.Theme) float32 { return theme.Metrics.ControlHeight + 4 }

// TableHeaderHeight is the default height of the header row, TableHead's h-10.
func TableHeaderHeight(theme ui.Theme) float32 { return theme.Metrics.ControlHeight + 8 }

// BeginTable starts a table whose first painted row is at bounds.Y. Cells use the
// theme's GapItem of horizontal padding unless CellPadding overrides it. A cell
// whose text is wider than its column ends in an ellipsis, as it does in a native
// table; shadcn's cells never truncate, its auto-width columns grow with their
// content instead, which FitTableHeadings does for the headings of fixed-width
// columns.
func BeginTable(canvas *ui.Canvas, theme ui.Theme, bounds ui.Rect, columns []TableColumn) Table {
	return Table{
		canvas: canvas, theme: theme, bounds: bounds, columns: columns,
		y: bounds.Y, rowHeight: TableRowHeight(theme), headerHeight: TableHeaderHeight(theme),
	}
}

// TableTextState is the caller-owned text scratch of one table's selection:
// the serialized text and cell map that TextLen, OffsetAt and SelectedText
// answer from. Keep one for the table's lifetime and pass it to
// UseSelectionState before the first row of every paint; its buffers are
// reused across frames, so a steady table paints without allocating.
type TableTextState struct {
	text tableTextState
}

// UseSelectionState binds state as this paint's selection text, reusing its
// buffers across frames. Call it after setting Selection and before the first
// row; a nil state leaves every paint rebuilding the text afresh.
func (table *Table) UseSelectionState(state *TableTextState) {
	if state == nil {
		return
	}
	state.text.reset()
	table.text = &state.text
}

// reset empties the state while keeping its buffers' capacity.
func (text *tableTextState) reset() {
	text.plain = text.plain[:0]
	text.rows = text.rows[:0]
	text.cells = text.cells[:0]
	text.selectionSpans = text.selectionSpans[:0]
}

// RowHeight overrides the default body and footer row height for subsequent calls.
func (table Table) RowHeight(height float32) Table {
	if height > 0 {
		table.rowHeight = height
	}
	return table
}

// HeaderHeight overrides the default header row height for subsequent calls.
func (table Table) HeaderHeight(height float32) Table {
	if height > 0 {
		table.headerHeight = height
	}
	return table
}

// FontSize overrides the size of body and header text for subsequent calls. A
// zero or negative size keeps the theme's FontSizeM.
func (table Table) FontSize(size float32) Table {
	if size > 0 {
		table.fontSize = size
	}
	return table
}

// CellPadding overrides the horizontal padding inside every cell for subsequent
// calls. A zero or negative padding keeps the theme's GapItem.
func (table Table) CellPadding(padding float32) Table {
	if padding > 0 {
		table.cellPadding = padding
	}
	return table
}

// textSize is the size every cell's line is measured and painted at.
func (table *Table) textSize() float32 {
	if table.fontSize > 0 {
		return table.fontSize
	}
	return table.theme.Metrics.FontSizeM
}

// cellPad is the horizontal padding every cell's line is laid out with.
func (table *Table) cellPad() float32 {
	if table.cellPadding > 0 {
		return table.cellPadding
	}
	return table.theme.Metrics.GapItem
}

// Header paints a muted header row. Extra cells are ignored and absent cells
// remain blank.
func (table *Table) Header(cells []string) {
	if table.headerSet {
		return
	}
	table.headerSet = true
	table.paintRow(ui.ID{}, cells, TableRowState{}, tableRowHeader, nil)
}

// Row paints one body row. A non-zero ID makes the row interactive; selected
// rows use the blue accent surface.
func (table *Table) Row(id ui.ID, cells []string, selected bool) {
	table.RowState(id, cells, TableRowState{Selected: selected})
}

// RowAt paints body row index of the list scope, numbering its own identity, and
// reports whether this paint is delivering a click on it. It is Row for a table
// whose rows are items of a list:
//
//	if table.RowAt(rows, i, cells[i], i == selected) { selected = i }
func (table *Table) RowAt(list ui.Scope, index int, cells []string, selected bool) bool {
	table.RowState(list.At(index), cells, TableRowState{Selected: selected})
	clicked, ok := table.canvas.ClickedIn(list)
	return ok && clicked == index
}

// RowState paints a body row with explicit selected/expanded state.
func (table *Table) RowState(id ui.ID, cells []string, state TableRowState) {
	table.paintRow(id, cells, state, tableRowBody, nil)
}

// RowLayout paints one body row and appends its cell geometry to dst. The
// caller owns dst and may reuse it across rows to keep custom cell decoration
// allocation-free; a nil dst collects nothing.
func (table *Table) RowLayout(id ui.ID, cells []string, state TableRowState, dst []ui.Rect) TableRowLayout {
	return table.paintRow(id, cells, state, tableRowBody, dst)
}

// Footer paints the official muted, medium-weight footer row.
func (table *Table) Footer(cells []string) {
	table.paintRow(ui.ID{}, cells, TableRowState{}, tableRowFooter, nil)
}

// Caption paints the muted caption below the rows already emitted.
func (table *Table) Caption(text string) {
	if text == "" {
		return
	}
	table.canvas.Label(text, ui.Rect{X: table.bounds.X, Y: table.y + 16, Width: table.bounds.Width, Height: table.theme.Metrics.FontSizeM + 4}, ui.TextStyle{
		Size: table.theme.Metrics.FontSizeM, Weight: ui.WeightRegular, Color: table.theme.MutedForeground,
	})
}

type tableRowKind uint8

const (
	tableRowBody tableRowKind = iota
	tableRowHeader
	tableRowFooter
)

// rowHeightFor fits height into the room left below the cursor, so a row never
// paints past the table's bottom edge.
func (table *Table) rowHeightFor(height float32) float32 {
	bottom := table.bounds.Y + table.bounds.Height
	if table.y >= bottom || height <= 0 {
		return 0
	}
	return max(min(height, bottom-table.y), 0)
}

func (table *Table) paintRow(id ui.ID, cells []string, state TableRowState, kind tableRowKind, cellsOut []ui.Rect) TableRowLayout {
	if len(table.columns) == 0 {
		return TableRowLayout{}
	}
	bottom := table.bounds.Y + table.bounds.Height
	height := table.rowHeight
	if kind == tableRowHeader {
		height = table.headerHeight
	}
	height = table.rowHeightFor(height)
	if height <= 0 {
		return TableRowLayout{}
	}
	rowBounds := ui.Rect{X: table.bounds.X, Y: table.y, Width: table.bounds.Width, Height: height}
	var textState *tableTextState
	if table.Selection.Enabled {
		if table.text == nil {
			table.text = &tableTextState{}
		}
		textState = table.text
		if len(textState.rows) > 0 {
			textState.plain = append(textState.plain, '\n')
		}
	}
	rowCellLo := 0
	if textState != nil {
		rowCellLo = len(textState.cells)
	}
	// Only a caller that passed storage gets geometry back; every other row
	// must not grow a slice it has no use for.
	collect := cellsOut != nil
	cellsOut = cellsOut[:0]
	background := table.theme.Background
	foreground := table.theme.Foreground
	weight := ui.WeightRegular
	paintBackground := false
	if kind == tableRowHeader {
		weight = ui.WeightMedium
	} else if kind == tableRowFooter {
		background = table.theme.Muted
		background.Alpha *= 0.50
		weight = ui.WeightMedium
		paintBackground = true
	} else if state.Expanded && !state.Selected {
		// has-aria-expanded:bg-muted/50, which is what Hover is.
		background = table.theme.Hover
		paintBackground = true
	} else if state.Selected {
		background = table.theme.Muted
		paintBackground = true
	} else if !id.IsZero() && table.canvas.Pressed() == id {
		background = table.theme.Muted
		paintBackground = true
	} else if !id.IsZero() && table.canvas.Hovered() == id {
		background = table.theme.Hover
		paintBackground = true
	}
	if paintBackground {
		table.canvas.Fill(rowBounds, background)
	}
	if kind == tableRowBody && !id.IsZero() && table.canvas.FocusVisible() == id {
		focus := table.theme.Primary
		focus.Alpha = 0.22
		table.canvas.Fill(ui.Rect{X: rowBounds.X, Y: rowBounds.Y, Width: 2, Height: rowBounds.Height}, focus)
	}

	x := rowBounds.X
	remaining := rowBounds.Width
	size, pad := table.textSize(), table.cellPad()
	for index, column := range table.columns {
		width := tableColumnWidth(column, index == len(table.columns)-1, remaining)
		if width <= 0 {
			break
		}
		cellBounds := ui.Rect{X: x, Y: rowBounds.Y, Width: width, Height: rowBounds.Height}
		if collect {
			cellsOut = append(cellsOut, cellBounds)
		}
		value := ""
		if index < len(cells) {
			value = cells[index]
		}
		inset := float32(0)
		if column.Alignment == TableAlignLeft {
			inset = min(column.Inset, max(cellBounds.Width-pad*2, 0))
		}
		textBounds := ui.Rect{
			X:      cellBounds.X + pad + inset,
			Y:      cellBounds.Y,
			Width:  tableMax32(0, cellBounds.Width-pad*2-inset),
			Height: cellBounds.Height,
		}
		field := tableTextField{}
		if textState != nil {
			field = appendTableTextCell(textState, index, value)
		}
		text := ""
		if index < len(cells) {
			text = table.canvas.FitLine(value, size, weight, textBounds.Width)
		}
		selected := false
		style := ui.TextStyle{Size: size, Weight: weight, Color: foreground}
		if textState != nil {
			textWidth := table.canvas.TextWidthWeight(text, size, weight)
			textX := table.canvas.LabelOrigin(text, textBounds, style, tableLabelAlignment(column.Alignment))
			selected = table.paintCellSelection(textState, textBounds, tableTextCell{
				bounds: textBounds, textLo: field.lo, textHi: field.hi,
				contentLo: field.contentLo, contentHi: field.contentHi,
				value: value, weight: weight, textX: textX, textWidth: textWidth,
			})
			textState.cells = append(textState.cells, tableTextCell{
				bounds: textBounds, textLo: field.lo, textHi: field.hi,
				contentLo: field.contentLo, contentHi: field.contentHi,
				value: value, weight: weight, textX: textX, textWidth: textWidth,
			})
		}
		if text != "" {
			table.canvas.Clip(textBounds)
			tableLabel(table.canvas, text, textBounds, column.Alignment, style)
			table.canvas.Unclip()
			if selected {
				_, selectedColor := tableSelectionColors(table.theme)
				style.Color = selectedColor
				table.paintSelectedCellText(textState, text, textBounds, column.Alignment, style)
			}
		}
		x += width
		remaining -= width
	}
	if kind == tableRowBody && !id.IsZero() {
		table.canvas.HitWithLabel(id, rowBounds, tableRowLabel(cells))
	}
	table.y += height
	if table.y < bottom {
		// border-b is part of the row's own height, painted over its background.
		table.canvas.Fill(ui.Rect{X: rowBounds.X, Y: table.y - 1, Width: rowBounds.Width, Height: 1}, table.theme.Border)
	}
	if textState != nil {
		textState.rows = append(textState.rows, tableTextRow{
			bounds: rowBounds, lo: rowCellLo, hi: len(textState.cells),
		})
	}
	return TableRowLayout{Bounds: rowBounds, Cells: cellsOut}
}

type tableTextField struct {
	lo, hi               int
	contentLo, contentHi int
}

// appendTableTextCell appends one RFC 4180-style TSV field. Quoting only
// changes the copyable representation; cells continue to paint their original
// value, so embedded tabs, newlines and quotes copy losslessly.
func appendTableTextCell(text *tableTextState, index int, value string) tableTextField {
	if index > 0 {
		text.plain = append(text.plain, '\t')
	}
	field := tableTextField{lo: len(text.plain)}
	quoted := strings.ContainsAny(value, "\t\n\r\"")
	if quoted {
		text.plain = append(text.plain, '"')
	}
	field.contentLo = len(text.plain)
	if quoted {
		for index := 0; index < len(value); index++ {
			if value[index] == '"' {
				text.plain = append(text.plain, '"')
			}
			text.plain = append(text.plain, value[index])
		}
	} else {
		text.plain = append(text.plain, value...)
	}
	field.contentHi = len(text.plain)
	if quoted {
		text.plain = append(text.plain, '"')
	}
	field.hi = len(text.plain)
	return field
}

func tableLabelAlignment(alignment TableAlignment) ui.Alignment {
	switch alignment {
	case TableAlignCenter:
		return ui.AlignCenter
	case TableAlignRight:
		return ui.AlignRight
	default:
		return ui.AlignStart
	}
}

func (table *Table) paintCellSelection(
	text *tableTextState,
	bounds ui.Rect,
	cell tableTextCell,
) bool {
	if text == nil || cell.contentLo == cell.contentHi {
		return false
	}
	start, end := table.Selection.Anchor, table.Selection.Focus
	if start > end {
		start, end = end, start
	}
	lo, hi := max(start, cell.contentLo), min(end, cell.contentHi)
	if lo >= hi {
		return false
	}
	visualLo := tableSerializedColumnOffset(cell.value, lo-cell.contentLo)
	visualHi := tableSerializedColumnOffset(cell.value, hi-cell.contentLo)
	if visualLo >= visualHi {
		return false
	}
	text.selectionSpans = table.canvas.TextSelection(
		text.selectionSpans[:0], cell.value, table.textSize(), cell.weight,
		visualLo, visualHi,
	)
	table.canvas.Clip(ui.Rect{
		X: cell.textX, Y: bounds.Y, Width: cell.textWidth, Height: bounds.Height,
	})
	background, _ := tableSelectionColors(table.theme)
	for _, span := range text.selectionSpans {
		table.canvas.Fill(ui.Rect{
			X: cell.textX + span.X, Y: bounds.Y,
			Width: span.Width, Height: bounds.Height,
		}, background)
	}
	table.canvas.Unclip()
	return true
}

func (table *Table) paintSelectedCellText(
	text *tableTextState,
	value string,
	bounds ui.Rect,
	alignment TableAlignment,
	style ui.TextStyle,
) {
	if text == nil {
		return
	}
	textX := table.canvas.LabelOrigin(value, bounds, style, tableLabelAlignment(alignment))
	for _, span := range text.selectionSpans {
		if span.Width <= 0 {
			continue
		}
		table.canvas.Clip(ui.Rect{
			X: textX + span.X, Y: bounds.Y,
			Width: span.Width, Height: bounds.Height,
		})
		tableLabel(table.canvas, value, bounds, alignment, style)
		table.canvas.Unclip()
	}
}

// tableSerializedColumnOffset maps a byte boundary in a quoted TSV content
// segment back to the visible cell value. Only ASCII quotes need expansion, so
// non-ASCII UTF-8 bytes pass through unchanged.
func tableSerializedColumnOffset(value string, offset int) int {
	offset = min(max(offset, 0), len(value)+strings.Count(value, `"`))
	serialized := 0
	for index := 0; index < len(value); index++ {
		width := 1
		if value[index] == '"' {
			width = 2
		}
		if offset <= serialized {
			return index
		}
		if offset < serialized+width {
			return index
		}
		serialized += width
	}
	return len(value)
}

// tableColumnWidth is the width a column gets of the room remaining: its own, or
// all that is left for a column with no width (and for a last one too wide).
func tableColumnWidth(column TableColumn, last bool, remaining float32) float32 {
	width := column.Width
	if width <= 0 || last && width > remaining {
		width = remaining
	}
	return min(width, remaining)
}

// FitTableHeadings widens each fixed-width column of columns that is narrower than
// its heading to the width the heading needs, so that no heading is cut or ends in an
// ellipsis. It is what shadcn's TableHead gets from whitespace-nowrap in an auto-width
// table, whose column is never narrower than its heading. A column that takes the
// remaining space (Width <= 0) and a column without a heading keep their width; the
// heading of a column that is too wide for the room left is still ellipsized.
//
// Call it before BeginTable and before TableCellBounds with the columns and headings
// that are passed to them: the declared widths are the minimum, and the heading is
// measured in the language and weight it is painted in, so a translation that is
// longer than its column does not need a wider column written for it.
//
//	components.FitTableHeadings(canvas, theme, columns[:], headings[:])
//	table := components.BeginTable(canvas, theme, bounds, columns[:])
//	table.Header(headings[:])
func FitTableHeadings(canvas *ui.Canvas, theme ui.Theme, columns []TableColumn, headings []string) {
	pad := theme.Metrics.GapItem * 2
	for index := range columns {
		column := &columns[index]
		if column.Width <= 0 || index >= len(headings) || headings[index] == "" {
			continue
		}
		need := canvas.TextWidthWeight(headings[index], theme.Metrics.FontSizeM, ui.WeightMedium)
		inset := float32(0)
		if column.Alignment == TableAlignLeft {
			inset = column.Inset
		}
		// A heading is measured to the pixel, so round it up: it must fit, not tie.
		column.Width = max(column.Width, float32(math.Ceil(float64(need)))+pad+inset)
	}
}

// TableCellBounds appends to dst the cells of a row that fills row, as Header and
// Row paint them: fixed widths in order, a column without one taking what is left,
// cells that no longer fit left out. It lets a caller put something on a cell, a
// sort arrow or a hit region, without painting the row first.
func TableCellBounds(columns []TableColumn, row ui.Rect, dst []ui.Rect) []ui.Rect {
	x, remaining := row.X, row.Width
	for index, column := range columns {
		width := tableColumnWidth(column, index == len(columns)-1, remaining)
		if width <= 0 {
			break
		}
		dst = append(dst, ui.Rect{X: x, Y: row.Y, Width: width, Height: row.Height})
		x += width
		remaining -= width
	}
	return dst
}

// TextLen is the length in bytes of the tab-separated text represented by the
// rows in the last paint. It is zero unless Table.Selection.Enabled was set
// before the rows were painted.
func (table *Table) TextLen() int {
	if !table.Selection.Enabled || table.text == nil {
		return 0
	}
	return len(table.text.plain)
}

// SelectedText returns the selected TSV content. A visually ellipsized cell
// still contributes its complete value. A selection that touches a quoted
// field is expanded to that field's boundaries so copying never emits an
// unquoted partial field containing a tab, newline or quote.
func (table *Table) SelectedText(selection TableSelection) string {
	if table.text == nil {
		return ""
	}
	start, end := selection.Range(table.text.plain)
	for _, cell := range table.text.cells {
		quoted := cell.textLo != cell.contentLo || cell.textHi != cell.contentHi
		if !quoted {
			continue
		}
		if start > cell.textLo && start < cell.textHi {
			start = cell.textLo
		}
		if end > cell.textLo && end < cell.textHi {
			end = cell.textHi
		}
	}
	return string(table.text.plain[start:end])
}

// OffsetAt maps a window point to the nearest UTF-8 byte offset in the
// serialized TSV text painted most recently. It remains valid until the next
// paint.
func (table *Table) OffsetAt(point ui.Point) (int, bool) {
	if !table.Selection.Enabled || table.canvas == nil || table.text == nil ||
		len(table.text.rows) == 0 || len(table.text.cells) == 0 {
		return 0, false
	}
	rowIndex := nearestTableTextRow(table.text.rows, point.Y)
	row := &table.text.rows[rowIndex]
	cells := table.text.cells[row.lo:row.hi]
	if len(cells) == 0 {
		return row.lo, true
	}
	cellIndex := nearestTableTextCell(cells, point.X)
	cell := &cells[cellIndex]
	if cell.contentHi <= cell.contentLo || cell.bounds.Width <= 0 {
		return cell.textLo, true
	}
	localX := point.X - cell.textX
	rtl := ui.TextDirectionOf(cell.value) == ui.DirectionRTL
	if localX <= 0 {
		if rtl {
			return cell.contentHi, true
		}
		return cell.contentLo, true
	}
	if cell.textWidth <= 0 || localX >= cell.textWidth {
		if rtl {
			return cell.contentLo, true
		}
		return cell.contentHi, true
	}
	local := table.canvas.TextOffsetAt(cell.value, table.textSize(), cell.weight, localX)
	local = min(max(local, 0), len(cell.value))
	local += strings.Count(cell.value[:local], `"`)
	return min(cell.contentLo+local, cell.contentHi), true
}

func nearestTableTextRow(rows []tableTextRow, y float32) int {
	if y <= rows[0].bounds.Y {
		return 0
	}
	index := sort.Search(len(rows), func(index int) bool {
		return y < rows[index].bounds.Bottom()
	})
	if index < len(rows) {
		return index
	}
	return len(rows) - 1
}

func nearestTableTextCell(cells []tableTextCell, x float32) int {
	if x <= cells[0].bounds.X {
		return 0
	}
	index := sort.Search(len(cells), func(index int) bool {
		return x < cells[index].bounds.Right()
	})
	if index < len(cells) {
		return index
	}
	return len(cells) - 1
}

// tableSelectionColors keeps the table recipe self-contained while preserving
// the Primary fallback used by hand-built themes from before Selection existed.
func tableSelectionColors(theme ui.Theme) (background, foreground ui.Color) {
	background, foreground = theme.Selection, theme.SelectionForeground
	if background.Alpha == 0 {
		background = theme.Primary
	}
	if foreground.Alpha == 0 {
		foreground = theme.PrimaryForeground
	}
	return background, foreground
}

func tableLabel(canvas *ui.Canvas, value string, bounds ui.Rect, alignment TableAlignment, style ui.TextStyle) {
	switch alignment {
	case TableAlignCenter:
		canvas.LabelCenter(value, bounds, style)
	case TableAlignRight:
		width := canvas.TextWidthWeight(value, style.Size, style.Weight)
		if width < bounds.Width {
			bounds.X += bounds.Width - width
			bounds.Width = width
		}
		canvas.Label(value, bounds, style)
	default:
		canvas.Label(value, bounds, style)
	}
}

func tableRowLabel(cells []string) string {
	for _, cell := range cells {
		if cell != "" {
			return cell
		}
	}
	return "Table row"
}

func tableMax32(left, right float32) float32 {
	if left > right {
		return left
	}
	return right
}

// TableSortDirection is the ordering indicated by a sortable header. The
// application owns the comparison and the data; Table only paints its state.
type TableSortDirection uint8

const (
	TableSortNone TableSortDirection = iota
	TableSortAscending
	TableSortDescending
)

// TableSortState is caller-owned single-column ordering. Its zero value has no
// active column; Toggle selects ascending first, then alternates both ways.
type TableSortState struct {
	Column    int
	Direction TableSortDirection
}

func (state TableSortState) Toggle(column int) TableSortState {
	if column < 0 {
		return state
	}
	next := TableSortState{Column: column, Direction: TableSortAscending}
	if state.Column == column && state.Direction == TableSortAscending {
		next.Direction = TableSortDescending
	}
	return next
}

// TableHeader describes one header cell. A non-zero ID makes it sortable and
// keyboard-activatable; a zero ID remains an ordinary text heading. Description
// optionally localizes the accessible sort-state description (the default is
// English). Disabled headers keep their direction visible but cannot activate.
type TableHeader struct {
	ID       ui.ID
	Label    string
	Sort     TableSortDirection
	Disabled bool
	// FocusableWhenDisabled lets a header retain keyboard focus while its
	// application is refreshing. Activation is still disabled.
	FocusableWhenDisabled bool
	Description           string
}

const tableSortIconSize, tableSortIconGap = float32(14), float32(4)

// FitSortableTableHeadings is FitTableHeadings for SortableHeader. It reserves
// room for the direction icon on interactive columns as well as their labels.
func FitSortableTableHeadings(canvas *ui.Canvas, theme ui.Theme, columns []TableColumn, headings []TableHeader) {
	for index := range columns {
		column := &columns[index]
		if column.Width <= 0 || index >= len(headings) || headings[index].Label == "" {
			continue
		}
		need := canvas.TextWidthWeight(headings[index].Label, theme.Metrics.FontSizeM, ui.WeightMedium)
		need = float32(math.Ceil(float64(need))) + 2*theme.Metrics.GapItem
		if column.Alignment == TableAlignLeft {
			need += max(column.Inset, 0)
		}
		if !headings[index].ID.IsZero() {
			need += tableSortIconSize + tableSortIconGap
		}
		column.Width = max(column.Width, need)
	}
}

// SortableHeader paints one header row using the table's column widths,
// alignments, clipping and HeaderHeight. IDs activate through the usual click
// event pass, including Enter/Space on a focused header. The application toggles
// its TableSortState and sorts its complete loaded dataset before paginating.
// Like Header, a second header declaration on this table does nothing.
func (table *Table) SortableHeader(cells []TableHeader) {
	if table.headerSet {
		return
	}
	table.headerSet = true
	height := table.rowHeightFor(table.headerHeight)
	if len(table.columns) == 0 || height <= 0 {
		return
	}
	var textState *tableTextState
	if table.Selection.Enabled {
		if table.text == nil {
			table.text = &tableTextState{}
		}
		textState = table.text
		if len(textState.rows) > 0 {
			textState.plain = append(textState.plain, '\n')
		}
	}
	rowCellLo := 0
	if textState != nil {
		rowCellLo = len(textState.cells)
	}
	y := table.y
	x, remaining := table.bounds.X, table.bounds.Width
	for index, column := range table.columns {
		width := tableColumnWidth(column, index == len(table.columns)-1, remaining)
		if width <= 0 {
			break
		}
		cell := TableHeader{}
		if index < len(cells) {
			cell = cells[index]
		}
		field := tableTextField{}
		if textState != nil {
			field = appendTableTextCell(textState, index, cell.Label)
		}
		painted := table.paintSortableHeaderCell(
			ui.Rect{X: x, Y: y, Width: width, Height: height},
			column, cell, textState, field,
		)
		if textState != nil {
			textState.cells = append(textState.cells, painted)
		}
		x, remaining = x+width, remaining-width
	}
	table.y += height
	if table.y < table.bounds.Y+table.bounds.Height {
		table.canvas.Fill(ui.Rect{
			X: table.bounds.X, Y: table.y - 1, Width: table.bounds.Width, Height: 1,
		}, table.theme.Border)
	}
	if textState != nil {
		textState.rows = append(textState.rows, tableTextRow{
			bounds: ui.Rect{X: table.bounds.X, Y: y, Width: table.bounds.Width, Height: height},
			lo:     rowCellLo,
			hi:     len(textState.cells),
		})
	}
}

func (table *Table) paintSortableHeaderCell(
	bounds ui.Rect,
	column TableColumn,
	cell TableHeader,
	textState *tableTextState,
	field tableTextField,
) tableTextCell {
	canvas, theme := table.canvas, table.theme
	interactive := !cell.ID.IsZero()
	color := theme.Foreground
	if cell.Disabled {
		color.Alpha *= 0.5
	}
	if interactive {
		if !cell.Disabled && (canvas.Hovered() == cell.ID || canvas.Pressed() == cell.ID) {
			canvas.Fill(bounds, theme.Hover)
		}
		if !cell.Disabled && canvas.FocusVisible() == cell.ID {
			canvas.RoundedBorder(bounds.Inset(ui.All(1)), theme.Metrics.RadiusControl, 1, theme.Ring, ui.Color{})
		}
		description := cell.Description
		if description == "" {
			switch cell.Sort {
			case TableSortAscending:
				description = "Sorted ascending"
			case TableSortDescending:
				description = "Sorted descending"
			default:
				description = "Activate to sort"
			}
		}
		canvas.HitWithOptions(cell.ID, bounds, cell.Label, ui.HitOptions{Role: ui.RoleButton, Disabled: cell.Disabled, FocusableWhenDisabled: cell.FocusableWhenDisabled, Description: description})
	}
	canvas.Clip(bounds)
	defer canvas.Unclip()
	size, pad := table.textSize(), table.cellPad()
	textBounds := bounds.Inset(ui.Insets{Left: pad, Right: pad})
	if column.Alignment == TableAlignLeft {
		inset := min(max(column.Inset, 0), max(textBounds.Width, 0))
		textBounds.X, textBounds.Width = textBounds.X+inset, max(textBounds.Width-inset, 0)
	}
	style := ui.TextStyle{Size: size, Weight: ui.WeightMedium, Color: color}
	if !interactive {
		label := canvas.FitLine(cell.Label, size, ui.WeightMedium, textBounds.Width)
		painted := tableTextCell{
			bounds: textBounds, textLo: field.lo, textHi: field.hi,
			contentLo: field.contentLo, contentHi: field.contentHi,
			value: cell.Label, weight: ui.WeightMedium,
		}
		if textState != nil {
			painted.textWidth = canvas.TextWidthWeight(label, size, ui.WeightMedium)
			painted.textX = canvas.LabelOrigin(label, textBounds, style, tableLabelAlignment(column.Alignment))
			selected := table.paintCellSelection(textState, textBounds, painted)
			tableLabel(canvas, label, textBounds, column.Alignment, style)
			if selected {
				_, selectedColor := tableSelectionColors(theme)
				style.Color = selectedColor
				table.paintSelectedCellText(textState, label, textBounds, column.Alignment, style)
			}
		} else {
			tableLabel(canvas, label, textBounds, column.Alignment, style)
		}
		return painted
	}
	const iconSize, gap = tableSortIconSize, tableSortIconGap
	label := canvas.FitLine(cell.Label, size, ui.WeightMedium, max(textBounds.Width-iconSize-gap, 0))
	labelWidth := min(canvas.TextWidthWeight(label, size, ui.WeightMedium), max(textBounds.Width-iconSize-gap, 0))
	groupWidth := min(labelWidth+gap+iconSize, max(textBounds.Width, 0))
	switch column.Alignment {
	case TableAlignRight:
		textBounds.X += max(textBounds.Width-groupWidth, 0)
	case TableAlignCenter:
		textBounds.X += max(textBounds.Width-groupWidth, 0) / 2
	}
	textBounds.Width = labelWidth
	painted := tableTextCell{
		bounds: textBounds, textLo: field.lo, textHi: field.hi,
		contentLo: field.contentLo, contentHi: field.contentHi,
		value: cell.Label, weight: ui.WeightMedium, textWidth: labelWidth,
		textX: canvas.LabelOrigin(label, textBounds, style, ui.AlignStart),
	}
	canvas.Label(label, textBounds, style)
	if table.paintCellSelection(textState, textBounds, painted) {
		_, selectedColor := tableSelectionColors(theme)
		style.Color = selectedColor
		table.paintSelectedCellText(textState, label, textBounds, TableAlignLeft, style)
	}
	icon := tableUnsortedIcon
	if cell.Sort == TableSortAscending {
		icon = tableAscendingIcon
	} else if cell.Sort == TableSortDescending {
		icon = tableDescendingIcon
	}
	canvas.DrawIcon(icon, ui.Rect{
		X:     textBounds.X + labelWidth + gap,
		Y:     bounds.Y + (bounds.Height-iconSize)/2,
		Width: iconSize, Height: iconSize,
	}, color)
	return painted
}

var tableUnsortedIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 8, Y: 9}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 5}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 16, Y: 9}},
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 8, Y: 15}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 19}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 16, Y: 15}},
}}

var tableAscendingIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 12, Y: 19}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 5}},
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 7, Y: 10}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 5}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 17, Y: 10}},
}}

var tableDescendingIcon = ui.Icon{ViewBox: 24, Stroke: 2, Cap: ui.CapRound, Join: ui.JoinRound, Commands: []ui.PathCommand{
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 12, Y: 5}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 19}},
	{Kind: ui.PathCommandMove, Point: ui.Point{X: 7, Y: 14}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 12, Y: 19}}, {Kind: ui.PathCommandLine, Point: ui.Point{X: 17, Y: 14}},
}}
