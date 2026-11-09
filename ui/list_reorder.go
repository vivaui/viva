package ui

import (
	"sort"

	"github.com/vivaui/viva/gfx"
)

// reorderPointer follows a press on a row of a List with Reorder, which turns into a
// drag once the pointer has moved far enough with it, and drops the rows it carries
// when it is let go. It runs before the rows are declared, from what the last paint
// found under the pointer.
func (s *ListState) reorderPointer(canvas *gfx.Canvas) {
	in := &s.list
	drag := &in.drag
	if s.Reorder == nil || canvas == nil {
		drag.reset()
		return
	}
	if drag.active {
		drag.to, drag.valid = drag.next, drag.nextOK // what the rows under the pointer found
		drag.nextOK = false
		band := min(s.rowHeight(drag.from), in.view.Height/4)
		dragEdgeScroll(canvas, in.scroll, in.view, drag.at, band, &drag.scrolled)
	}
	_, event, ok := canvas.PointerIn(in.rows)
	if !ok {
		return
	}
	point := gfx.Point{X: event.X, Y: event.Y}
	switch event.Kind {
	case gfx.PointerDown:
		drag.reset()
		if event.Button > gfx.PointerButtonPrimary {
			return
		}
		if row := s.indexOf(event.Target); row >= 0 && !s.isHeader(row) {
			// Not consumed: a press that does not move is still the row's click.
			drag.pressed, drag.from, drag.start, drag.at = true, row, point, point
		}
	case gfx.PointerMove:
		if !drag.pressed {
			return
		}
		dx, dy := point.X-drag.start.X, point.Y-drag.start.Y
		if !drag.active && dx*dx+dy*dy < dragStartDistance*dragStartDistance {
			return
		}
		if !drag.active {
			drag.active = true
			s.carriedRows(drag)
		}
		drag.at = point
		canvas.Consume()
	case gfx.PointerUp:
		active := drag.active
		rows, to, valid := drag.rows, drag.to, drag.valid
		if !active {
			drag.reset()
			return // a click
		}
		// A drag is not also a click on the row it ends on: its moves were consumed,
		// which already says so, and so does its release.
		canvas.Consume()
		if valid && !movesNothing(rows, to) {
			moved := append([]int(nil), rows...) // the drag's own rows are reused
			reorder := s.Reorder
			canvas.AfterPaint(func() { reorder(moved, to) })
		}
		drag.reset()
	}
}

// carriedRows sets the rows a drag from drag.from carries: those chosen, when it is
// one of them, else that row.
func (s *ListState) carriedRows(drag *listDrag) {
	drag.rows = drag.rows[:0]
	if s.Selection == nil || !s.Chosen(drag.from) {
		drag.rows = append(drag.rows, drag.from)
		return
	}
	for j := 0; j < s.list.n; j++ {
		if !s.isHeader(j) && s.Chosen(j) {
			drag.rows = append(drag.rows, j)
		}
	}
}

// movesNothing says that putting rows (ascending) before row to leaves them where
// they are: they are one run, and to is at or beside it.
func movesNothing(rows []int, to int) bool {
	return len(rows) > 0 && rows[len(rows)-1]-rows[0]+1 == len(rows) && to >= rows[0] && to <= rows[len(rows)-1]+1
}

// carries reports whether row is one of the rows being dragged.
func (d *listDrag) carries(row int) bool {
	at := sort.SearchInts(d.rows, row)
	return at < len(d.rows) && d.rows[at] == row
}

// drawOver finds, as the area is painted over its rows, where the rows being dragged
// would go for the pointer where it is, and shows it as a line between rows.
func (in *listInternals) drawOver(d *DrawContext) {
	drag := &in.drag
	if !drag.active || len(in.declared) == 0 {
		return
	}
	f := d.frame
	to, y := in.declared[0].index, f.nodes[in.declared[0].node].rect.Y
	for i := range in.declared {
		row := &in.declared[i]
		r := f.nodes[row.node].rect
		if drag.at.Y < r.Y+r.Height/2 {
			to, y = row.index, r.Y
			break
		}
		to, y = row.index+1, r.Y+r.Height
	}
	drag.next, drag.nextOK = to, true
	if !drag.valid || drag.to != to || movesNothing(drag.rows, to) {
		return
	}
	y = min(max(y, d.Rect.Y+1), d.Rect.Y+d.Rect.Height-1)
	d.Canvas.Fill(Rect{X: d.Rect.X, Y: y - 1, Width: d.Rect.Width, Height: 2}, d.Theme.Primary)
}
