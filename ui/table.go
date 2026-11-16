package ui

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// TableSource is where a Table reads its rows from, a row at a time and only the
// rows that are on screen, so a table of ten thousand rows costs what the visible
// ones cost. Implement it on a pointer (or use *TableRows), because a value
// converted to an interface is boxed.
type TableSource interface {
	Len() int
	// Cell is the text of one cell. It is called for each visible cell once a frame.
	Cell(row, column int) string
}

// TableRows is a TableSource for text that is already in memory.
type TableRows [][]string

func (t *TableRows) Len() int { return len(*t) }
func (t *TableRows) Cell(row, column int) string {
	if cells := (*t)[row]; column < len(cells) {
		return cells[column]
	}
	return ""
}

// TableSpec declares a Table.
type TableSpec struct {
	// Rows names the rows: row i is Rows.At(i). Header names the header cells of
	// the columns in Sortable: column c is Header.At(c). Header may be zero when
	// nothing is sortable.
	Rows, Header gfx.Scope
	// Columns are the widths and alignments, as for components.Table: a column
	// without a width (the last) takes what is left.
	Columns []components.TableColumn
	// Headers are the column titles; none means no header row.
	Headers []string
	// Sortable says which columns can be sorted by clicking their header (a nil
	// slice, none); the Table shows the arrow of the one TableState says is sorted.
	Sortable []bool
	Source   TableSource
	// RowHeight is the height of every row, header included; zero takes the
	// theme's components.TableRowHeight and TableHeaderHeight.
	RowHeight float32
	// FontSize is the size of body and header text; zero takes the theme's
	// Metrics.FontSizeM.
	FontSize float32
	// CellPadding is the horizontal padding inside every cell; zero takes the
	// theme's Metrics.GapItem.
	CellPadding float32
	// Striped shades every second row.
	Striped bool
	// FrozenColumns pins the first columns that have explicit widths to the
	// table's left edge while the rest scroll under OffsetX: the vertical
	// identity rows keep, a header's sort hits keep their place in paint
	// order, and the horizontal scrollbar and drag mapping account for the
	// frozen width. Zero, the default, keeps every column moving with the
	// scroll, as tables always have; a table that does not scroll sideways
	// ignores it.
	FrozenColumns int
	// Scrollbar is the identity of the vertical bar a table that scrolls itself
	// draws under its columns; zero paints the bar without an action.
	Scrollbar gfx.ID
	// HorizontalScrollbar is the identity of the bar below the body when the
	// columns overflow their viewport; zero paints that bar without an action.
	HorizontalScrollbar gfx.ID
	// ScrollbarLabel names the vertical bar for assistive technology; empty is
	// "Table rows". HorizontalScrollbarLabel does the same for the horizontal
	// bar, whose default is "Table columns".
	ScrollbarLabel           string
	HorizontalScrollbarLabel string
	// ScrollbarSize is the bars' thickness; zero keeps the thin hover default.
	ScrollbarSize float32
	// ScrollbarGutter is the strip reserved beside or below the cells for an
	// overflowing bar; zero lets the bars sit over their edges. The columns lose
	// the width of a vertical gutter, and the rows the height of a horizontal one.
	ScrollbarGutter float32
	// HideHoverScrollbars hides the un-named vertical hover overlay for an
	// application that paints its own vertical bar beside the table. It does not
	// change scrollbar geometry, the horizontal overlay, scrolling, or any bar
	// with an explicit Scrollbar ID. The default paints the overlay as usual.
	HideHoverScrollbars bool
}

// tableRoute is a Table of the last paint that scrolls itself, and where: the
// page gives it the wheel and the keys that reach it.
type tableRoute struct {
	state      *TableState
	bounds     Rect
	scrolls    bool
	sideways   bool
	bar        gfx.ID
	horizontal gfx.ID
}

// TableState is what a Table keeps between frames, in the application's state: the
// selected row, the sort, and, when the table has a height of its own, how far it is
// scrolled. The zero value is a table with nothing selected, unsorted, at the top.
type TableState struct {
	// Scroll is the scroll position of a table with a height of its own (a Height,
	// Fill or Grow that is less than its rows need), and, when its explicit column
	// widths are wider than it, how far across it is scrolled. Give config.OnScroll
	// state.Wheel, or call it for the table under the pointer. A table in a
	// ScrollArea is as tall as its rows, as wide as its area makes it, and needs
	// none of it.
	Scroll           ScrollState
	selected         int32 // 1 + the row, or 0
	sortColumn       int32 // 1 + the column, or 0
	descending       bool
	reveal           bool      // scroll the selected row into view at the next paint
	rows, page       int32     // as of the last paint: how many rows, how many fit at once
	rowFirst, rowEnd int32     // the declared source rows: rowFirst <= i < rowEnd
	scope            gfx.Scope // the rows' identities, to tell a key meant for the table
	// cellLefts are the painted columns' left edges and the last column's right
	// edge, and cellTop and cellRowHeight the first visible row's top and every
	// row's height, as of the last paint; see CellRect. They keep their
	// capacity across frames, so a warm table records them without allocating.
	cellLefts     []float32
	cellTop       float32
	cellRowHeight float32
}

// Selected is the selected row, or -1.
func (s *TableState) Selected() int { return int(s.selected) - 1 }

// Select selects a row; a negative row clears the selection.
func (s *TableState) Select(row int) { s.selected = int32(max(row, -1) + 1) }

// Sort is the sorted column and its direction; ok is false when nothing is sorted.
func (s *TableState) Sort() (column int, descending, ok bool) {
	return int(s.sortColumn) - 1, s.descending, s.sortColumn > 0
}

// SortBy is what a click on a column's header does: sort by it ascending, or reverse
// it when it is the sorted one already. The application orders its rows by what Sort
// says.
func (s *TableState) SortBy(column int) {
	if int(s.sortColumn) == column+1 {
		s.descending = !s.descending
		return
	}
	s.sortColumn, s.descending = int32(column)+1, false
}

// ClearSort leaves no column sorted.
func (s *TableState) ClearSort() { s.sortColumn, s.descending = 0, false }

// Wheel applies a scroll event to a table that scrolls itself; see ScrollState.Wheel.
func (s *TableState) Wheel(event gfx.ScrollEvent) gfx.EventResult { return s.Scroll.Wheel(event) }

// Key moves the selection when a row of the table has the keyboard: Up and Down by a
// row, PageUp and PageDown by what is on screen, Home and End to the first and last
// row. The new row is selected, takes the keyboard focus and is scrolled into view
// (in the table's own scroll position, or in the nearest ScrollArea around it). Give
// it to Config.OnKey, or call it from yours; it ignores every other key and any other
// focus, and reports Handled only for the keys it used.
//
//	config.OnKey = m.table.Key
func (s *TableState) Key(event gfx.KeyEvent) gfx.EventResult {
	if s.rows == 0 || event.Modifiers&(gfx.ModifierControl|gfx.ModifierMeta|gfx.ModifierAlt) != 0 {
		return gfx.EventResult{}
	}
	at, ok := event.Target.In(s.scope)
	if !ok {
		return gfx.EventResult{}
	}
	last, page := int(s.rows)-1, max(int(s.page)-1, 1)
	row := at
	switch event.Key {
	case gfx.KeyUp:
		row--
	case gfx.KeyDown:
		row++
	case gfx.KeyPageUp:
		row -= page
	case gfx.KeyPageDown:
		row += page
	case gfx.KeyHome:
		row = 0
	case gfx.KeyEnd:
		row = last
	default:
		return gfx.EventResult{}
	}
	row = min(max(row, 0), last)
	s.Select(row)
	s.reveal = true
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.scope.At(row), FocusVisible: true}
}

type tableDecl struct {
	spec  TableSpec
	state *TableState
	rows  int
}

// Table adds a table painted by components.Table, with a header row of sortable
// column titles, rows that show selection and hover, and optional stripes. It is as
// wide as its container and as tall as its header and rows, so in a ScrollArea only
// the rows on screen are read and painted; given a smaller height it scrolls its
// rows under a fixed header, by state.Scroll.
//
// A table of explicit column widths wider than itself also scrolls left and
// right under its fixed header, the header riding along with its rows: the
// horizontal wheel, Shift with the wheel, or a horizontal scrollbar
// (HorizontalScrollbar) moves the columns, and state.Scroll reports how far
// across it is. FrozenColumns pins the first columns to the table's left edge
// while the rest scroll: the pinned band covers what passes under it, a rule
// at its right edge keeps the two apart, hits over the band still name the
// row and column under the pointer whatever the offset, and the horizontal
// scrollbar maps exactly the columns that move — MaxX reaches every scrolled
// pixel while the offset never moves the pinned ones. A table whose last
// column has no width keeps the layout it always had: the last column takes
// what is left, however narrow that is. A table inside a ScrollArea is never
// scrolled across by itself; the area owns its width, sideways or not.
//
//	tbl := f.Table(&state, ui.TableSpec{Rows: rows, Header: heads, Columns: cols, Headers: titles, Sortable: sortable, Source: &data})
//	if row, ok := tbl.RowClicked(); ok { state.Select(row) }
//	if col, ok := tbl.HeaderClicked(); ok { state.SortBy(col); data.Sort(state.Sort()) }
func (f *frame) Table(state *TableState, spec TableSpec) Box {
	b := f.box(kindTable)
	n := &f.nodes[b.i]
	n.aux = int32(len(f.widgets.tables))
	rows := 0
	if spec.Source != nil {
		rows = max(spec.Source.Len(), 0)
	}
	f.widgets.tables = append(f.widgets.tables, tableDecl{spec: spec, state: state, rows: rows})
	return b
}

// RowClicked reports which row of a Table this paint is delivering a click on.
func (b Box) RowClicked() (int, bool) {
	n := b.n()
	if n.kind != kindTable || b.f.canvas == nil {
		return 0, false
	}
	return b.f.canvas.ClickedIn(b.f.widgets.tables[n.aux].spec.Rows)
}

// HeaderClicked reports which column header of a Table this paint is delivering a
// click on.
func (b Box) HeaderClicked() (int, bool) {
	n := b.n()
	if n.kind != kindTable || b.f.canvas == nil {
		return 0, false
	}
	return b.f.canvas.ClickedIn(b.f.widgets.tables[n.aux].spec.Header)
}

func (t *tableDecl) rowHeight(theme Theme) float32 {
	if t.spec.RowHeight > 0 {
		return t.spec.RowHeight
	}
	return components.TableRowHeight(theme)
}

func (t *tableDecl) headerHeight(theme Theme) float32 {
	if len(t.spec.Headers) == 0 {
		return 0
	}
	if t.spec.RowHeight > 0 {
		return t.spec.RowHeight
	}
	return components.TableHeaderHeight(theme)
}

// measureTable: as wide as its fixed columns, as tall as its header and rows.
func (f *frame) measureTable(n *node) {
	t := &f.widgets.tables[n.aux]
	var width float32
	for _, column := range t.spec.Columns {
		width += max(column.Width, 0)
	}
	n.pref = [2]float32{width, t.headerHeight(f.theme) + float32(t.rows)*t.rowHeight(f.theme)}
}

// tableColumnsWidth is the total width a table's columns declare: the fixed
// widths of every column, zero for the flexible one that takes what is left.
func tableColumnsWidth(columns []components.TableColumn) float32 {
	width := float32(0)
	for _, column := range columns {
		width += max(column.Width, 0)
	}
	return width
}

// frozenColumnCount is how many leading columns a table with FrozenColumns
// count pins: the count itself, cut at the first column without a width of
// its own, which takes what is left and pins nothing after it.
func frozenColumnCount(columns []components.TableColumn, count int) int {
	if count <= 0 {
		return 0
	}
	frozen := min(count, len(columns))
	for index := 0; index < frozen; index++ {
		if columns[index].Width <= 0 {
			return index
		}
	}
	return frozen
}

// frozenColumnsWidth is the combined width of the columns a table freezes.
// Every one of them must declare an explicit width, so a flexible column ends
// the frozen band: it takes what is left and pins nothing. The count was cut
// at it already. Zero when the table pins nothing.
func frozenColumnsWidth(columns []components.TableColumn, count int) float32 {
	width := float32(0)
	for index := 0; index < count && index < len(columns); index++ {
		width += columns[index].Width
	}
	return width
}

// tableFrozenSeparator is how wide the rule at a frozen band's right edge is.
const tableFrozenSeparator = float32(2)

// scrollAncestor is the state and viewport of the ScrollArea nearest around element n.
func (f *frame) scrollAncestor(n *node) (*ScrollState, Rect) {
	for p := n.parent; p >= 0; p = f.nodes[p].parent {
		if ext := f.nodes[p].ext; ext != 0 && f.exts[ext].scrollState != nil {
			return f.exts[ext].scrollState, f.nodes[p].rect
		}
	}
	return nil, Rect{}
}

// paintTable paints the header, then the rows that are on screen, with the same
// components.Table calls an application would make by hand. A table as tall as its
// rows inside a clip (a ScrollArea) keeps its header at the top of what is visible
// while its rows scroll under it.
func (f *frame) paintTable(n *node) {
	t := &f.widgets.tables[n.aux]
	r := n.rect
	rowHeight, headerHeight := t.rowHeight(f.theme), t.headerHeight(f.theme)
	canvas := f.canvas
	body := Rect{X: r.X, Y: r.Y + headerHeight, Width: r.Width, Height: max(r.Height-headerHeight, 0)}
	content := float32(t.rows) * rowHeight
	gutter := max(t.spec.ScrollbarGutter, 0)
	// A table whose last column has no width keeps the layout it always had: the
	// last column takes what is left, however narrow that is. Only explicit widths
	// that do not fit move the whole grid across, with the header riding along —
	// and never inside a ScrollArea, whose viewport (sideways or not) owns the
	// table's width; such a table is only ever clipped.
	declaredWidth := tableColumnsWidth(t.spec.Columns)
	flexibleLast := len(t.spec.Columns) == 0 || t.spec.Columns[len(t.spec.Columns)-1].Width <= 0
	selfContained := true
	if ancestor, _ := f.scrollAncestor(n); ancestor != nil {
		selfContained = false
	}
	verticalGutter := float32(0)
	if body.Height+0.5 < content {
		verticalGutter = min(gutter, body.Width)
		body.Width -= verticalGutter
	}
	sideways := selfContained && !flexibleLast && declaredWidth > body.Width+0.5
	horizontalGutter := float32(0)
	if sideways {
		horizontalGutter = min(gutter, body.Height)
		body.Height -= horizontalGutter
	}
	scrolls := body.Height+0.5 < content
	contentWidth := body.Width
	if !flexibleLast && declaredWidth > contentWidth {
		contentWidth = declaredWidth
	}
	if f.page != nil {
		f.page.tables = append(f.page.tables, tableRoute{
			state: t.state, bounds: r, scrolls: scrolls, sideways: sideways,
			bar: t.spec.Scrollbar, horizontal: t.spec.HorizontalScrollbar,
		})
	}
	var offset, offsetX float32
	s := &t.state.Scroll
	s.sideways = true // TableState.Wheel accepts both axes; only the overflows below can move.
	s.Content, s.Viewport = content, min(body.Height, content)
	if scrolls {
		s.Viewport = body.Height
	}
	s.ContentWidth, s.ViewportWidth = contentWidth, body.Width
	if !sideways {
		s.OffsetX = 0 // an outer ScrollArea owns the table's geometry instead
	}
	t.state.rows, t.state.scope = int32(t.rows), t.spec.Rows
	if t.state.reveal {
		f.revealSelected(n, t, body, rowHeight, headerHeight, scrolls)
	}
	s.Offset = min(max(s.Offset, 0), s.Max())
	s.OffsetX = min(max(s.OffsetX, 0), s.MaxX())
	if scrolls {
		offset = snapScroll(s.Offset)
	}
	if sideways {
		offsetX = snapScroll(s.OffsetX)
	}
	left := body.X - offsetX
	// Frozen columns pin a band of the body's leading width while the rest
	// scrolls under the offset. A band with no width, or as wide as the body,
	// pins nothing — there would be no window left to scroll in.
	frozenWidth := float32(0)
	frozenColumns := 0
	if sideways {
		frozenColumns = frozenColumnCount(t.spec.Columns, t.spec.FrozenColumns)
		frozenWidth = frozenColumnsWidth(t.spec.Columns, frozenColumns)
		if frozenWidth <= 0 || frozenWidth >= body.Width {
			frozenWidth, frozenColumns = 0, 0
		}
	}
	seam := body.X + frozenWidth

	// The rows on screen: the table's body, cut by every clip it sits in.
	top := body.Y - offset
	lo, hi := body.Y, body.Y+body.Height
	clipTop := f.bounds.Y
	for p := n.parent; p >= 0; p = f.nodes[p].parent {
		if f.nodes[p].flags&flagClip != 0 {
			clip := f.nodes[p].rect
			lo, hi = max(lo, clip.Y), min(hi, clip.Y+clip.Height)
			clipTop = max(clipTop, clip.Y)
		}
	}
	// A header that has scrolled above the visible area stays at its top, until the
	// last row would push it out.
	headerY := r.Y
	sticky := !scrolls && headerHeight > 0 && r.Y < clipTop
	if sticky {
		headerY = min(clipTop, r.Y+r.Height-headerHeight)
		lo = max(lo, headerY+headerHeight)
	}
	t.state.page = int32(max(int((hi-lo)/rowHeight), 1))
	first, last := 0, 0
	if hi > lo && t.rows > 0 {
		first = min(max(int(math.Floor(float64((lo-top)/rowHeight))), 0), t.rows)
		last = min(max(int(math.Ceil(float64((hi-top)/rowHeight))), first), t.rows)
	}
	t.state.rowFirst, t.state.rowEnd = int32(first), int32(last)
	// The cells CellRect answers with, laid out once from the same columns and
	// the same top the rows are painted from (see recordTableCells).
	f.recordTableCells(t, body, left, top, contentWidth, rowHeight, frozenColumns, frozenWidth)

	if !scrolls && !sticky {
		// Height: r.Height lets paintTableHeader keep its rule of the header's
		// bottom rule needing room below it.
		f.paintTableHeader(t, Rect{X: left, Y: r.Y, Width: contentWidth, Height: r.Height}, body, headerHeight, rowHeight, sideways && frozenWidth == 0, frozenWidth)
	}
	if last > first {
		extra := float32(0)
		if top+float32(last)*rowHeight < r.Y+r.Height {
			extra = 1 // a divider under the last row painted, when something follows or the table is taller
		}
		if scrolls || sideways {
			canvas.Clip(body)
		} else if sticky {
			below := headerY + headerHeight
			canvas.Clip(Rect{X: r.X, Y: below, Width: body.Width, Height: max(r.Y+r.Height-below, 0)})
		}
		// The scrolling pass paints every column of the on-screen rows, and the
		// clip does the work of the segmentation: the frozen columns fall
		// outside the window the clip leaves and stay out of it.
		if sideways && frozenWidth > 0 {
			canvas.Clip(Rect{X: seam, Y: body.Y, Width: max(body.X+body.Width-seam, 0), Height: body.Height})
		}
		f.paintTableRows(t, Rect{X: left, Y: top + float32(first)*rowHeight, Width: contentWidth, Height: float32(last-first)*rowHeight + extra}, first, last, top, contentWidth, rowHeight)
		if sideways && frozenWidth > 0 {
			// The frozen band rides above, opaque, on the unscrolled origin:
			// the same rows again, clipped to the band, with the columns
			// painted from the body's left edge rather than the offset one.
			canvas.Unclip()
			canvas.Clip(Rect{X: body.X, Y: body.Y, Width: frozenWidth, Height: body.Height})
			canvas.Fill(Rect{X: body.X, Y: top + float32(first)*rowHeight, Width: frozenWidth, Height: float32(last-first)*rowHeight + extra}, f.theme.Background)
			f.paintTableRows(t, Rect{X: body.X, Y: top + float32(first)*rowHeight, Width: contentWidth, Height: float32(last-first)*rowHeight + extra}, first, last, top, contentWidth, rowHeight)
			canvas.Unclip()
		}
		if scrolls || sideways || sticky {
			canvas.Unclip()
		}
		if sideways && frozenWidth > 0 {
			canvas.Fill(Rect{X: seam - tableFrozenSeparator, Y: top + float32(first)*rowHeight, Width: tableFrozenSeparator, Height: float32(last-first)*rowHeight + extra}, f.theme.Border)
		}
	}
	if scrolls || sticky {
		if headerHeight > 0 {
			canvas.Fill(Rect{X: r.X, Y: headerY, Width: body.Width, Height: headerHeight}, f.theme.Background)
		}
		f.paintTableHeader(t, Rect{X: left, Y: headerY, Width: contentWidth, Height: r.Height}, body, headerHeight, rowHeight, sideways && frozenWidth == 0, frozenWidth)
	}
	if scrolls || sideways {
		f.paintTableScrollbars(t, r, body, gfx.Size{Width: contentWidth, Height: content}, offsetX, offset, frozenWidth)
	}
}

// paintTableRows paints the on-screen rows of a table once, with the same
// components.Table calls an application would make by hand. The caller has
// already clipped to the band the rows belong to, and rows keeps the whole
// grid's geometry: every column paints, and the clip does the segmentation.
func (f *frame) paintTableRows(t *tableDecl, rows Rect, first, last int, top, contentWidth, rowHeight float32) {
	canvas := f.canvas
	table := components.BeginTable(canvas, f.theme, rows, t.spec.Columns).
		RowHeight(rowHeight).FontSize(t.spec.FontSize).CellPadding(t.spec.CellPadding)
	w := &f.widgets
	if cap(w.cells) < len(t.spec.Columns) {
		w.cells = make([]string, len(t.spec.Columns))
	}
	cells := w.cells[:len(t.spec.Columns)]
	for row := first; row < last; row++ {
		for column := range cells {
			cells[column] = t.spec.Source.Cell(row, column)
		}
		if t.spec.Striped && row%2 == 1 {
			stripe := f.theme.Muted
			stripe.Alpha *= 0.6
			canvas.Fill(Rect{X: rows.X, Y: top + float32(row)*rowHeight, Width: contentWidth, Height: rowHeight}, stripe)
		}
		// A press on the bar owns the pointer: the row under it neither
		// highlights nor takes a click while the bar is held.
		id := t.spec.Rows.At(row)
		if t.state.Scroll.dragging {
			id = gfx.ID{}
		}
		table.RowState(id, cells, components.TableRowState{Selected: row == t.state.Selected()})
	}
}

// paintTableScrollbars paints and drags the bars of a Table that scrolls itself.
// They share the body's content extents and offsets; a bar sits in the gutter the
// spec asked for and over the body's edge when it did not.
func (f *frame) paintTableScrollbars(t *tableDecl, r, body Rect, content gfx.Size, offsetX, offsetY, frozenWidth float32) {
	revealed := f.page == nil || f.page.pointerOver(r) || t.state.Scroll.dragging
	label := t.spec.ScrollbarLabel
	if label == "" {
		label = "Table rows"
	}
	horizontalLabel := t.spec.HorizontalScrollbarLabel
	if horizontalLabel == "" {
		horizontalLabel = "Table columns"
	}
	size := t.spec.ScrollbarSize
	if size <= 0 && f.page != nil {
		size = f.page.config.ScrollbarSize
	}
	// The bars sit at the table's own edges, over the body's clips: the component
	// places their tracks from these bounds alone. The area they own is the whole
	// table below its header, the body plus the gutters it reserved, so a bar
	// lands in its gutter symmetrically with the other one and the corner between
	// two bars stays clear. A gutter also moves the bars beside the viewport: the
	// component takes the bars' strips out of it, matching the strip the columns
	// and rows have already given up, so a drag maps the body it scrolls exactly.
	// Without a gutter the bars keep sitting over the table's edges, as they had.
	// An un-owned vertical hover thumb may duplicate an application's adjacent
	// bar. Omit its component declaration: an explicit ID still paints and keeps
	// its hit and drag, while the area's geometry and horizontal bar are unchanged.
	verticalID, verticalLabel := t.spec.Scrollbar, label
	if t.spec.HideHoverScrollbars && verticalID.IsZero() {
		verticalLabel = ""
	}
	areaBounds := Rect{X: r.X, Y: body.Y, Width: r.Width, Height: r.Y + r.Height - body.Y}
	area := components.BeginScrollArea(f.canvas, f.theme, areaBounds, content).
		Offset(offsetX, offsetY).
		Frozen(frozenWidth).
		ScrollbarSize(size).
		HoverScrollbars(revealed)
	if t.spec.ScrollbarGutter > 0 {
		area = area.ReserveScrollbars()
	}
	area = area.
		VerticalScrollbar(verticalID, verticalLabel, "").
		HorizontalScrollbar(t.spec.HorizontalScrollbar, horizontalLabel, "")
	geometry := area.Geometry()
	area.EndContent()
	if !t.spec.Scrollbar.IsZero() && geometry.MaxOffsetY > 0 {
		f.dragScrollbar(&t.state.Scroll, t.spec.Scrollbar, geometry, axisY)
	}
	if !t.spec.HorizontalScrollbar.IsZero() && geometry.MaxOffsetX > 0 {
		f.dragScrollbar(&t.state.Scroll, t.spec.HorizontalScrollbar, geometry, axisX)
	}
}

// revealSelected scrolls the selected row into view after a key moved it: the table's
// own position when it scrolls itself, otherwise the ScrollArea around it (which takes
// effect at the next frame, so one is asked for).
func (f *frame) revealSelected(n *node, t *tableDecl, body Rect, rowHeight, headerHeight float32, scrolls bool) {
	t.state.reveal = false
	row := t.state.Selected()
	if row < 0 || row >= t.rows {
		return
	}
	rowTop := float32(row) * rowHeight
	if scrolls {
		s := &t.state.Scroll
		switch {
		case rowTop < s.Offset:
			s.Offset = rowTop
		case rowTop+rowHeight > s.Offset+body.Height:
			s.Offset = rowTop + rowHeight - body.Height
		}
		return
	}
	outer, view := f.scrollAncestor(n)
	if outer == nil {
		return
	}
	y := body.Y + rowTop
	if above := view.Y + headerHeight; y < above {
		outer.Offset = max(outer.Offset-(above-y), 0)
	} else if below := view.Y + view.Height; y+rowHeight > below {
		outer.Offset += y + rowHeight - below
	}
	if f.canvas != nil {
		f.canvas.RequestFrame()
	}
}

// paintTableHeader paints the header row and, over it, the hit regions and arrow
// of the sortable columns. A sideways table clips it to the body's columns, and
// one with frozen columns paints it in two passes clipped to each band, so the
// header of the frozen columns rides above what scrolls under it.
func (f *frame) paintTableHeader(t *tableDecl, r, viewport Rect, headerHeight, rowHeight float32, sideways bool, frozenWidth float32) {
	if headerHeight <= 0 {
		return
	}
	canvas := f.canvas
	if sideways {
		// The header scrolls with the body, clipped to the same columns; without
		// an overflow it is laid out over the table, as it always was.
		canvas.Clip(Rect{X: viewport.X, Y: r.Y, Width: viewport.Width, Height: headerHeight})
		defer canvas.Unclip()
	}
	if frozenWidth > 0 {
		// Frozen columns are only of a sideways table, whose clip is already
		// on. The scrolling pass narrows it to the window past the frozen
		// band; the band then paints its header above, opaque, from the
		// body's unscrolled origin, and rules itself off from the columns
		// that pass under it.
		seam := viewport.X + frozenWidth
		canvas.Clip(Rect{X: seam, Y: r.Y, Width: max(viewport.X+viewport.Width-seam, 0), Height: headerHeight})
		f.paintTableHeaderRow(t, r, headerHeight, rowHeight)
		canvas.Unclip()
		canvas.Clip(Rect{X: viewport.X, Y: r.Y, Width: frozenWidth, Height: headerHeight})
		canvas.Fill(Rect{X: viewport.X, Y: r.Y, Width: frozenWidth, Height: headerHeight}, f.theme.Background)
		pinned := r
		pinned.X = viewport.X
		f.paintTableHeaderRow(t, pinned, headerHeight, rowHeight)
		canvas.Fill(Rect{X: seam - tableFrozenSeparator, Y: r.Y, Width: tableFrozenSeparator, Height: headerHeight}, f.theme.Border)
		canvas.Unclip()
		return
	}
	f.paintTableHeaderRow(t, r, headerHeight, rowHeight)
}

// paintTableHeaderRow paints the header cells of r once, with the hover fills,
// the sort arrows and the hit regions of the sortable columns.
func (f *frame) paintTableHeaderRow(t *tableDecl, r Rect, headerHeight, rowHeight float32) {
	canvas := f.canvas
	bounds := Rect{X: r.X, Y: r.Y, Width: r.Width, Height: headerHeight}
	cells := components.TableCellBounds(t.spec.Columns, bounds, f.widgets.rects[:0])
	f.widgets.rects = cells[:0]
	hover := f.theme.Hover
	for column, cell := range cells {
		if column < len(t.spec.Sortable) && t.spec.Sortable[column] && !t.spec.Header.IsZero() && canvas.Hovered() == t.spec.Header.At(column) {
			canvas.Fill(cell, hover)
		}
	}
	headerRow := Rect{X: r.X, Y: r.Y, Width: r.Width, Height: headerHeight + 1}
	if r.Height <= headerHeight {
		headerRow.Height = headerHeight
	}
	table := components.BeginTable(canvas, f.theme, headerRow, t.spec.Columns).
		RowHeight(rowHeight).HeaderHeight(headerHeight).FontSize(t.spec.FontSize).CellPadding(t.spec.CellPadding)
	table.Header(t.spec.Headers)

	sorted, descending, isSorted := t.state.Sort()
	metrics := f.theme.Metrics
	fontSize := t.spec.FontSize
	if fontSize <= 0 {
		fontSize = metrics.FontSizeM
	}
	for column, cell := range cells {
		if column >= len(t.spec.Sortable) || !t.spec.Sortable[column] || t.spec.Header.IsZero() {
			continue
		}
		title := ""
		if column < len(t.spec.Headers) {
			title = t.spec.Headers[column]
		}
		if isSorted && sorted == column {
			arrow := "↑"
			if descending {
				arrow = "↓"
			}
			width := canvas.TextWidthWeight(title, fontSize, gfx.WeightMedium)
			text := Rect{X: cell.X + metrics.PadControlX, Width: max(cell.Width-metrics.PadControlX*2, 0)}
			x := text.X + min(width, text.Width) + 4
			switch t.spec.Columns[column].Alignment {
			case components.TableAlignCenter:
				x = text.X + (text.Width+min(width, text.Width))/2 + 4
			case components.TableAlignRight:
				x = text.X + text.Width - min(width, text.Width) - 4 - 12
			}
			canvas.LabelCenter(arrow, Rect{X: x, Y: cell.Y, Width: 12, Height: cell.Height}, gfx.TextStyle{Size: fontSize, Weight: gfx.WeightMedium, Color: f.theme.MutedForeground})
		}
		canvas.HitWithLabel(t.spec.Header.At(column), cell, title)
	}
}
