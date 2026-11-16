package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// CellRect is where source cell (row, column) was painted by the last paint,
// in window coordinates, and false for a cell the table did not paint: a row
// out of view, a column past the ones painted, or any cell before the table's
// first paint. It is the geometry counterpart of RowID, both answers the last
// paint declared, so a caller that puts something over a cell — an editor —
// reads it between frames without repeating the table's arithmetic.
//
// Rows and columns are source indices, as RowID's are: scrolling and sorting do
// not renumber them, and a row keeps its source index as it moves on screen.
// The rectangle is the whole cell and not the part the table's clip leaves
// visible, because an overlay is meant to cover the cell: a row half scrolled
// out at the top of the body still answers with its full height. A frozen
// column answers where that column was pinned; a column scrolling under the
// band keeps its scrolled place, which may lie outside the table.
//
// The header is not a cell and has no source row: it has no rectangle here.
// Declaring the same state in more than one table leaves the calls describing
// only the last table painted.
func (s *TableState) CellRect(row, column int) (gfx.Rect, bool) {
	if row < int(s.rowFirst) || row >= int(s.rowEnd) || column < 0 || column+1 >= len(s.cellLefts) {
		return gfx.Rect{}, false
	}
	left, right := s.cellLefts[column], s.cellLefts[column+1]
	if right <= left || s.cellRowHeight <= 0 {
		return gfx.Rect{}, false
	}
	return gfx.Rect{
		X: left, Y: s.cellTop + float32(row)*s.cellRowHeight,
		Width: right - left, Height: s.cellRowHeight,
	}, true
}

// recordTableCells keeps the cells of the last paint for CellRect: the left
// edge of every painted column, plus the right edge of the last, and the top
// and height of a row. The columns come from components.TableCellBounds, the
// same layout the rows and the header are painted from, so a table pays one
// pass over its columns a frame and no per-cell allocation. A frozen column's
// edge is taken from the body's unscrolled origin, where the band paints it;
// the rest follow the offset the rows were painted at.
func (f *frame) recordTableCells(t *tableDecl, body Rect, left, top, contentWidth, rowHeight float32, frozenColumns int, frozenWidth float32) {
	state := t.state
	state.cellLefts = state.cellLefts[:0]
	state.cellTop, state.cellRowHeight = top, rowHeight
	if len(t.spec.Columns) == 0 || rowHeight <= 0 {
		return
	}
	cells := components.TableCellBounds(t.spec.Columns, Rect{Width: contentWidth, Height: rowHeight}, f.widgets.rects[:0])
	f.widgets.rects = cells[:0]
	for index, cell := range cells {
		x := left + cell.X
		if frozenWidth > 0 && index < frozenColumns {
			x = body.X + cell.X
		}
		state.cellLefts = append(state.cellLefts, x)
	}
	if len(cells) > 0 {
		last := cells[len(cells)-1]
		x := left + last.X + last.Width
		if frozenWidth > 0 && len(cells)-1 < frozenColumns {
			x = body.X + last.X + last.Width
		}
		state.cellLefts = append(state.cellLefts, x)
	}
}
