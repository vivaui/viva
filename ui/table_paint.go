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
		RowHeight(rowHeight).FontSize(t.GetSpec().FontSize).CellPadding(t.GetSpec().CellPadding)
	w := base.WidgetsOf(f)
	if cap(w.GetCells()) < len(t.GetSpec().Columns) {
		w.SetCells(make([]string, len(t.GetSpec().Columns)))
	}
	cells := w.GetCells()[:len(t.GetSpec().Columns)]
	for row := First; row < Last; row++ {
		for column := range cells {
			cells[column] = t.GetSpec().Source.Cell(row, column)
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
	for column, cell := range cells {
		if column < len(t.GetSpec().Sortable) && t.GetSpec().Sortable[column] && !t.GetSpec().Header.IsZero() && canvas.Hovered() == t.GetSpec().Header.At(column) {
			canvas.Fill(cell, hover)
		}
	}
	headerRow := Rect{X: r.X, Y: r.Y, Width: r.Width, Height: headerHeight + 1}
	if r.Height <= headerHeight {
		headerRow.Height = headerHeight
	}
	table := components.BeginTable(canvas, f.Palette, headerRow, t.GetSpec().Columns).
		RowHeight(rowHeight).HeaderHeight(headerHeight).FontSize(t.GetSpec().FontSize).CellPadding(t.GetSpec().CellPadding)
	table.Header(t.GetSpec().Headers)

	sorted, descending, isSorted := t.GetState().Sort()
	metrics := f.Palette.Metrics
	fontSize := t.GetSpec().FontSize
	if fontSize <= 0 {
		fontSize = metrics.FontSizeM
	}
	for column, cell := range cells {
		if column >= len(t.GetSpec().Sortable) || !t.GetSpec().Sortable[column] || t.GetSpec().Header.IsZero() {
			continue
		}
		title := ""
		if column < len(t.GetSpec().Headers) {
			title = t.GetSpec().Headers[column]
		}
		if isSorted && sorted == column {
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
		canvas.HitWithLabel(t.GetSpec().Header.At(column), cell, title)
	}
}
