package base

import (
	"sort"

	"github.com/vivaui/viva/gfx"
)

// reorderPointer follows a press on a row of a List with Reorder, which turns into a
// drag once the pointer has moved far enough with it, and drops the rows it carries
// when it is let go. It runs before the rows are declared, from what the last paint
// found under the pointer.
func (s *ListState) reorderPointer(Surface *gfx.Canvas) {
	in := &s.list
	drag := &in.drag
	if s.Reorder == nil || Surface == nil {
		drag.reset()
		return
	}
	if drag.active {
		drag.to, drag.valid = drag.Next, drag.nextOK // what the rows under the pointer found
		drag.nextOK = false
		band := min(s.rowHeight(drag.from), in.View.Height/4)
		dragEdgeScroll(Surface, in.Scroll, in.View, drag.at, band, &drag.scrolled)
	}
	_, event, ok := Surface.PointerIn(in.rows)
	if !ok && !s.Handle.IsZero() {
		// A drag that starts on the handle carries the handle as its target.
		_, event, ok = Surface.PointerIn(s.Handle)
	}
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
		row := -1
		fromHandle := !s.Handle.IsZero()
		if fromHandle {
			if at, inside := event.Target.In(s.Handle); inside {
				row = at
			}
		} else if at := s.indexOf(event.Target); at >= 0 {
			row = at
		}
		if row >= 0 && !s.isHeader(row) {
			// Not consumed: a press that does not move is still the row's click.
			drag.pressed, drag.from, drag.start, drag.at = true, row, point, point
			drag.handle = fromHandle
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
		Surface.Consume()
	case gfx.PointerUp:
		active := drag.active
		rows, to, valid := drag.rows, drag.to, drag.valid
		if !active {
			drag.reset()
			return // a click
		}
		// A drag is not also a click on the row it ends on: its moves were consumed,
		// which already says so, and so does its release.
		Surface.Consume()
		if valid && !movesNothing(rows, to) {
			if drag.handle {
				// The keyboard stays on the grip of what was dragged, where the
				// next paint names it.
				newRow := to
				if to > rows[0] {
					newRow = to - len(rows)
				}
				Surface.RequestFocus(s.Handle.At(newRow))
			}
			moved := append([]int(nil), rows...) // the drag's own rows are reused
			reorder := s.Reorder
			Surface.AfterPaint(func() { reorder(moved, to) })
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
	f := d.Frame
	to, y := in.declared[0].index, f.Nodes[in.declared[0].Node].Rect.Y
	for i := range in.declared {
		row := &in.declared[i]
		r := f.Nodes[row.Node].Rect
		if drag.at.Y < r.Y+r.Height/2 {
			to, y = row.index, r.Y
			break
		}
		to, y = row.index+1, r.Y+r.Height
	}
	drag.Next, drag.nextOK = to, true
	if !drag.valid || drag.to != to || movesNothing(drag.rows, to) {
		return
	}
	y = min(max(y, d.Rect.Y+1), d.Rect.Y+d.Rect.Height-1)
	d.Canvas.Fill(Rect{X: d.Rect.X, Y: y - 1, Width: d.Rect.Width, Height: 2}, d.Theme.Primary)
}

// reorderByHandle is Up or Down on a row's handle, as the grip of a reorder list
// takes it: the rows the drag would carry move one row and the keyboard follows the
// handle where it goes. At the first or the last row it moves nothing.
func (s *ListState) reorderByHandle(row int, up bool) (gfx.EventResult, bool) {
	result, handled := s.reorderByKey(row, up)
	if !handled || result.Focus.IsZero() {
		return result, handled
	}
	newRow := row - 1
	if !up {
		newRow = row + 1
	}
	result.Focus, result.FocusVisible = s.Handle.At(newRow), true
	return result, handled
}

// reorderHandleToEdge is Home or End on a row's handle: its rows go to the top or
// the bottom of the list.
func (s *ListState) reorderHandleToEdge(row int, top bool) (gfx.EventResult, bool) {
	var drag listDrag
	drag.from = row
	s.carriedRows(&drag)
	rows := drag.rows
	to := s.list.n
	if top {
		to = 0
	}
	if movesNothing(rows, to) {
		return gfx.EventResult{Handled: true}, true
	}
	s.Reorder(rows, to)
	newRow := 0
	if !top {
		newRow = s.list.n - len(rows)
	}
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.Handle.At(newRow), FocusVisible: true}, true
}

// reorderByKey is Alt with Up or Down on a row of a List with Reorder: the rows the
// drag would carry go one row up or down, as a drop would put them, and the keyboard
// stays on the row. At the first or the last row it moves nothing.
func (s *ListState) reorderByKey(row int, up bool) (gfx.EventResult, bool) {
	var drag listDrag
	drag.from = row
	s.carriedRows(&drag)
	rows := drag.rows
	to := rows[len(rows)-1] + 2 // before the row after the one below the run
	if up {
		to = rows[0] - 1
	}
	if to < 0 || to > s.list.n || s.isHeader(min(to, s.list.n-1)) {
		return gfx.EventResult{Handled: true}, true
	}
	focus := s.key(row) // a keyed row keeps its identity where it goes
	if s.Key == nil {
		focus = row + 1 // an index names the row: it is one place on
		if up {
			focus = row - 1
		}
	}
	s.Reorder(rows, to)
	return gfx.EventResult{Handled: true, Repaint: true, Focus: s.list.rows.At(focus), FocusVisible: true}, true
}
