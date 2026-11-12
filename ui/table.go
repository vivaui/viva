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
	// Scrollbar is the identity of the vertical bar a table that scrolls itself
	// draws under its columns; zero paints the bar without an action. A table
	// inside a ScrollArea takes the area's bar instead and ignores this.
	Scrollbar gfx.ID
	// ScrollbarLabel names that bar for assistive technology; empty is "Table
	// rows".
	ScrollbarLabel string
	// ScrollbarSize is the bar's thickness; zero keeps the thin hover default.
	ScrollbarSize float32
	// ScrollbarGutter is the strip reserved beside the columns for the bar; zero
	// lets the bar sit over their edge. The columns lose that width.
	ScrollbarGutter float32
}

// tableRoute is a Table of the last paint that scrolls itself, and where: the
// page gives it the wheel and the keys that reach it.
type tableRoute struct {
	state   *TableState
	bounds  Rect
	scrolls bool
	bar     gfx.ID
}

// TableState is what a Table keeps between frames, in the application's state: the
// selected row, the sort, and, when the table has a height of its own, how far it is
// scrolled. The zero value is a table with nothing selected, unsorted, at the top.
type TableState struct {
	// Scroll is the scroll position of a table with a height of its own (a Height,
	// Fill or Grow that is less than its rows need). Give config.OnScroll
	// state.Wheel, or call it for the table under the pointer. A table in a
	// ScrollArea is as tall as its rows and needs none of it.
	Scroll     ScrollState
	selected   int32 // 1 + the row, or 0
	sortColumn int32 // 1 + the column, or 0
	descending bool
	reveal     bool      // scroll the selected row into view at the next paint
	rows, page int32     // as of the last paint: how many rows, how many fit at once
	scope      gfx.Scope // the rows' identities, to tell a key meant for the table
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
	scrolls := body.Height+0.5 < content
	gutter := float32(0)
	if scrolls {
		gutter = min(max(t.spec.ScrollbarGutter, 0), r.Width)
		body.Width = r.Width - gutter
	}
	if f.page != nil {
		f.page.tables = append(f.page.tables, tableRoute{state: t.state, bounds: r, scrolls: scrolls, bar: t.spec.Scrollbar})
	}
	var offset float32
	s := &t.state.Scroll
	s.Content, s.Viewport = content, min(body.Height, content)
	if scrolls {
		s.Viewport = body.Height
	}
	t.state.rows, t.state.scope = int32(t.rows), t.spec.Rows
	if t.state.reveal {
		f.revealSelected(n, t, body, rowHeight, headerHeight, scrolls)
	}
	s.Offset = min(max(s.Offset, 0), s.Max())
	if scrolls {
		offset = snapScroll(s.Offset)
	}

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

	if !scrolls && !sticky {
		f.paintTableHeader(t, Rect{X: r.X, Y: r.Y, Width: body.Width, Height: r.Height}, headerHeight, rowHeight)
	}
	if last > first {
		extra := float32(0)
		if top+float32(last)*rowHeight < r.Y+r.Height {
			extra = 1 // a divider under the last row painted, when something follows or the table is taller
		}
		if scrolls {
			canvas.Clip(body)
		} else if sticky {
			below := headerY + headerHeight
			canvas.Clip(Rect{X: r.X, Y: below, Width: body.Width, Height: max(r.Y+r.Height-below, 0)})
		}
		table := components.BeginTable(canvas, f.theme, Rect{X: r.X, Y: top + float32(first)*rowHeight, Width: body.Width, Height: float32(last-first)*rowHeight + extra}, t.spec.Columns).
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
				canvas.Fill(Rect{X: r.X, Y: top + float32(row)*rowHeight, Width: body.Width, Height: rowHeight}, stripe)
			}
			// A press on the bar owns the pointer: the row under it neither
			// highlights nor takes a click while the bar is held.
			id := t.spec.Rows.At(row)
			if t.state.Scroll.dragging {
				id = gfx.ID{}
			}
			table.RowState(id, cells, components.TableRowState{Selected: row == t.state.Selected()})
		}
		if scrolls || sticky {
			canvas.Unclip()
		}
	}
	if scrolls || sticky {
		if headerHeight > 0 {
			canvas.Fill(Rect{X: r.X, Y: headerY, Width: body.Width, Height: headerHeight}, f.theme.Background)
		}
		f.paintTableHeader(t, Rect{X: r.X, Y: headerY, Width: body.Width, Height: r.Height}, headerHeight, rowHeight)
	}
	if scrolls {
		f.paintTableScrollbar(t, body, content, offset)
	}
}

// paintTableScrollbar paints and drags the bar of a Table that scrolls itself.
// The bar is the hover style announced for the area: the page's pointer reveals
// it, and it sits in the gutter beside the columns when one is asked for.
func (f *frame) paintTableScrollbar(t *tableDecl, body Rect, content, offset float32) {
	bounds := Rect{X: body.X, Y: body.Y, Width: body.Width + float32(max(t.spec.ScrollbarGutter, 0)), Height: body.Height}
	if t.spec.ScrollbarGutter <= 0 {
		bounds.Width = body.Width
	}
	revealed := f.page == nil || f.page.pointerOver(bounds) || t.state.Scroll.dragging
	label := t.spec.ScrollbarLabel
	if label == "" {
		label = "Table rows"
	}
	size := t.spec.ScrollbarSize
	if size <= 0 && f.page != nil {
		size = f.page.config.ScrollbarSize
	}
	area := components.BeginScrollArea(f.canvas, f.theme, bounds, gfx.Size{Height: content}).
		Offset(0, offset).
		ScrollbarSize(size).
		HoverScrollbars(revealed).
		VerticalScrollbar(t.spec.Scrollbar, label, "")
	geometry := area.Geometry()
	area.EndContent()
	if !t.spec.Scrollbar.IsZero() && geometry.MaxOffsetY > 0 {
		f.dragScrollbar(&t.state.Scroll, t.spec.Scrollbar, geometry, axisY)
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
// of the sortable columns.
func (f *frame) paintTableHeader(t *tableDecl, r Rect, headerHeight, rowHeight float32) {
	if headerHeight <= 0 {
		return
	}
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
