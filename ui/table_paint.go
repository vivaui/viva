package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// paintTableRows paints the on-screen rows of a table once, with the same
// components.Table calls an application would make by hand. The caller has
// already clipped to the band the rows belong to, and rows keeps the whole
// grid's geometry: every column paints, and the clip does the segmentation.
func paintTableRows(f *Frame, t *TableDecl, rows Rect, First, Last int, top, contentWidth, rowHeight float32) {
	canvas := f.Surface
	table := components.BeginTable(canvas, f.Palette, rows, t.GetSpec().Columns).
		RowHeight(rowHeight).FontSize(t.GetFontSize()).CellPadding(t.GetSpec().CellPadding)
	if t.GetForegroundSet() {
		table = table.Foreground(t.GetForeground())
	}
	w := base.WidgetsOf(f)
	if cap(w.GetCells()) < len(t.GetSpec().Columns) {
		w.SetCells(make([]string, len(t.GetSpec().Columns)))
	}
	cells := w.GetCells()[:len(t.GetSpec().Columns)]
	lead := t.GetLead()
	for row := First; row < Last; row++ {
		for column := range cells {
			if column < lead {
				cells[column] = "" // the column of checkboxes has no text
				continue
			}
			cells[column] = t.GetSpec().Source.Cell(row, column-lead)
		}
		if t.GetSpec().Striped && row%2 == 1 {
			stripe := f.Palette.Muted
			stripe.Alpha *= 0.6
			canvas.Fill(Rect{X: rows.X, Y: top + float32(row)*rowHeight, Width: contentWidth, Height: rowHeight}, stripe)
		}
		// A press on the bar owns the pointer: the row under it neither
		// highlights nor takes a click while the bar is held.
		id := t.GetSpec().Rows.At(row)
		if t.GetState().Scroll.GetDragging() {
			id = gfx.ID{}
		}
		table.RowState(id, cells, components.TableRowState{Selected: row == t.GetState().Selected()})
		if lead > 0 && !id.IsZero() {
			paintRowCheckbox(f, t, row, Rect{X: rows.X, Y: top + float32(row)*rowHeight, Width: t.GetSpec().Columns[0].Width, Height: rowHeight})
		}
	}
}

// paintTableScrollbars paints and drags the bars of a Table that scrolls itself.
// They share the body's content extents and offsets; a bar sits in the gutter the
// spec asked for and over the body's edge when it did not.
func paintTableScrollbars(f *Frame, t *TableDecl, r, body Rect, Content gfx.Size, offsetX, offsetY, frozenWidth float32) {
	revealed := f.GetPage() == nil || f.GetPage().PointerOver(r) || t.GetState().Scroll.GetDragging()
	label := t.GetSpec().ScrollbarLabel
	if label == "" {
		label = "Table rows"
	}
	horizontalLabel := t.GetSpec().HorizontalScrollbarLabel
	if horizontalLabel == "" {
		horizontalLabel = "Table columns"
	}
	size := t.GetSpec().ScrollbarSize
	if size <= 0 && f.GetPage() != nil {
		size = f.GetPage().GetConfig().ScrollbarSize
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
	verticalID, verticalLabel := t.GetSpec().Scrollbar, label
	if t.GetSpec().HideHoverScrollbars && verticalID.IsZero() {
		verticalLabel = ""
	}
	areaBounds := Rect{X: r.X, Y: body.Y, Width: r.Width, Height: r.Y + r.Height - body.Y}
	area := components.BeginScrollArea(f.Surface, f.Palette, areaBounds, Content).
		Offset(offsetX, offsetY).
		Frozen(frozenWidth).
		ScrollbarSize(size).
		HoverScrollbars(revealed)
	if t.GetSpec().ScrollbarGutter > 0 {
		area = area.ReserveScrollbars()
	}
	area = area.
		VerticalScrollbar(verticalID, verticalLabel, "").
		HorizontalScrollbar(t.GetSpec().HorizontalScrollbar, horizontalLabel, "")
	geometry := area.Geometry()
	area.EndContent()
	if !t.GetSpec().Scrollbar.IsZero() && geometry.MaxOffsetY > 0 {
		f.DragScrollbar(&t.GetState().Scroll, t.GetSpec().Scrollbar, geometry, AxisY)
	}
	if !t.GetSpec().HorizontalScrollbar.IsZero() && geometry.MaxOffsetX > 0 {
		f.DragScrollbar(&t.GetState().Scroll, t.GetSpec().HorizontalScrollbar, geometry, AxisX)
	}
}

// paintTableHeaderRow paints the header cells of r once, with the hover fills,
// the sort arrows and the hit regions of the sortable columns.
func paintTableHeaderRow(f *Frame, t *TableDecl, r Rect, headerHeight, rowHeight float32) {
	canvas := f.Surface
	bounds := Rect{X: r.X, Y: r.Y, Width: r.Width, Height: headerHeight}
	cells := components.TableCellBounds(t.GetSpec().Columns, bounds, base.WidgetsOf(f).GetRects()[:0])
	base.WidgetsOf(f).SetRectsCells(cells[:0])
	hover := f.Palette.Hover
	lead := t.GetLead()
	for column, cell := range cells {
		if column >= lead && column < len(t.GetSpec().Columns) && t.GetSpec().Columns[column].Sortable && !t.GetSpec().Header.IsZero() && canvas.Hovered() == t.GetSpec().Header.At(column-lead) {
			canvas.Fill(cell, hover)
		}
	}
	headerRow := Rect{X: r.X, Y: r.Y, Width: r.Width, Height: headerHeight + 1}
	if r.Height <= headerHeight {
		headerRow.Height = headerHeight
	}
	table := components.BeginTable(canvas, f.Palette, headerRow, t.GetSpec().Columns).
		RowHeight(rowHeight).HeaderHeight(headerHeight).FontSize(t.GetFontSize()).CellPadding(t.GetSpec().CellPadding)
	if t.GetForegroundSet() {
		table = table.Foreground(t.GetForeground())
	}
	table.Header(t.GetTitles())

	sorted, descending, isSorted := t.GetState().Sort()
	metrics := f.Palette.Metrics
	fontSize := t.GetFontSize()
	if fontSize <= 0 {
		fontSize = metrics.FontSizeM
	}
	if lead > 0 && len(cells) > 0 {
		paintHeaderCheckbox(f, t, cells[0])
	}
	for column, cell := range cells {
		if column < lead || column >= len(t.GetSpec().Columns) || !t.GetSpec().Columns[column].Sortable || t.GetSpec().Header.IsZero() {
			continue
		}
		title := t.GetSpec().Columns[column].Title
		if isSorted && sorted == column-lead {
			arrow := "↑"
			if descending {
				arrow = "↓"
			}
			width := canvas.TextWidthWeight(title, fontSize, gfx.WeightMedium)
			text := Rect{X: cell.X + metrics.PadControlX, Width: max(cell.Width-metrics.PadControlX*2, 0)}
			x := text.X + min(width, text.Width) + 4
			switch t.GetSpec().Columns[column].Alignment {
			case components.TableAlignCenter:
				x = text.X + (text.Width+min(width, text.Width))/2 + 4
			case components.TableAlignRight:
				x = text.X + text.Width - min(width, text.Width) - 4 - 12
			}
			canvas.LabelCenter(arrow, Rect{X: x, Y: cell.Y, Width: 12, Height: cell.Height}, gfx.TextStyle{Size: fontSize, Weight: gfx.WeightMedium, Color: f.Palette.MutedForeground})
		}
		canvas.HitWithLabel(t.GetSpec().Header.At(column-lead), cell, title)
	}
}

// checkSize is the side of a table's checkbox.
const checkSize = 16

// paintRowCheckbox paints the checkbox of a row in the cell of the column of
// checkboxes; a click on it was applied where the table was declared.
func paintRowCheckbox(f *Frame, t *TableDecl, row int, cell Rect) {
	canvas := f.Surface
	id := base.CheckID(canvas, t.GetSpec().Rows, row)
	label := t.GetSpec().SelectRowLabel
	if label == "" {
		label = "Select row"
	}
	box := Rect{X: cell.X + (cell.Width-checkSize)/2, Y: cell.Y + (cell.Height-checkSize)/2, Width: checkSize, Height: checkSize}
	components.BeginCheckbox(canvas, f.Palette, id, box, t.GetSpec().Checked.Has(row)).Label(label).Paint()
}

// paintHeaderCheckbox paints the checkbox of the header: ticked when every row is,
// mixed when some are.
func paintHeaderCheckbox(f *Frame, t *TableDecl, cell Rect) {
	canvas := f.Surface
	id := base.HeaderCheckID(canvas, t.GetSpec().Rows)
	label := t.GetSpec().SelectAllLabel
	if label == "" {
		label = "Select all rows"
	}
	box := Rect{X: cell.X + (cell.Width-checkSize)/2, Y: cell.Y + (cell.Height-checkSize)/2, Width: checkSize, Height: checkSize}
	checkbox := components.BeginCheckbox(canvas, f.Palette, id, box, t.AllChecked()).Label(label)
	if t.SomeChecked() && !t.AllChecked() {
		checkbox = checkbox.Indeterminate()
	}
	checkbox.Paint()
}
