package ui

import (
	"github.com/vivaui/viva/components"
)

// recordTableCells keeps the cells of the last paint for CellRect: the left
// edge of every painted column, plus the right edge of the last, and the top
// and height of a row. The columns come from components.TableCellBounds, the
// same layout the rows and the header are painted from, so a table pays one
// pass over its columns a frame and no per-cell allocation. A frozen column's
// edge is taken from the body's unscrolled origin, where the band paints it;
// the rest follow the offset the rows were painted at.
func (f *frame) RecordTableCells(t *tableDecl, body Rect, left, top, contentWidth, rowHeight float32, frozenColumns int, frozenWidth float32) {
	state := t.state
	state.cellLefts = state.cellLefts[:0]
	state.cellTop, state.cellRowHeight = top, rowHeight
	if len(t.spec.Columns) == 0 || rowHeight <= 0 {
		return
	}
	cells := components.TableCellBounds(t.spec.Columns, Rect{Width: contentWidth, Height: rowHeight}, widgetsOf(f).rects[:0])
	widgetsOf(f).rects = cells[:0]
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
