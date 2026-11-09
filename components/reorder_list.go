package components

import "github.com/vivaui/viva/gfx"

// ReorderMove describes one completed move in the caller's list. ID identifies
// the item independently of its position. From and To index the rows declared
// in the paint that produced the move, in the order Row was called, so a caller
// declares every item, in order, on every paint. Apply the move once, then
// repaint the list.
type ReorderMove struct {
	ID       gfx.ID
	From, To int
}

func (move ReorderMove) Valid() bool {
	return !move.ID.IsZero() && move.From >= 0 && move.To >= 0 && move.From != move.To
}

type reorderHandle struct {
	id       gfx.ID
	disabled bool
	// bounds is where the row was painted in the last pass, translated by
	// whatever scroll the caller applied. RowBounds hands it back.
	bounds gfx.Rect
}

// ReorderListState owns pointer geometry, never the application's data. Pointer
// emits a move only on a valid drop; Key supports arrows/Home/End on a handle.
// Escape cancels a held drag. Rows retain their stable IDs after a move.
//
// Scrolling stays caller-owned: Bounds is wherever the caller placed the list,
// and Viewport clips it. A move focuses the row it lands on, which can be
// outside Viewport in a scrolled list; the canvas clips such a row's hit away,
// so add RevealScroll(move.To) to the caller's offset when applying the move.
type ReorderListState struct {
	rows              []reorderHandle
	bounds, viewport  gfx.Rect
	stride            float32
	active            gfx.ID
	from, to          int
	startY, lastY     float32
	pressed, dragging bool
}

func (state *ReorderListState) Dragging() bool { return state.dragging }
func (state *ReorderListState) Cancel() {
	state.active = gfx.ID{}
	state.pressed, state.dragging = false, false
}

// RowBounds reports where a declared row was painted in the last pass, and
// false when that ID was not declared. It is the geometry a caller scrolls by
// to keep the row a keyboard move focused in view.
func (state *ReorderListState) RowBounds(id gfx.ID) (gfx.Rect, bool) {
	for index := range state.rows {
		if state.rows[index].id == id {
			return state.rows[index].bounds, true
		}
	}
	return gfx.Rect{}, false
}

// RevealScroll is the change to add to the caller's scroll offset so the slot a
// keyboard or pointer move landed on comes back inside Viewport: pass the
// move's To. Rows are evenly spaced, so that slot keeps its geometry once the
// item moves into it. A row above the viewport yields a negative delta and a
// row below a positive one; it is zero when the slot is already visible, out of
// range, or the list was given no viewport.
func (state *ReorderListState) RevealScroll(slot int) float32 {
	if slot < 0 || slot >= len(state.rows) || state.viewport.Height <= 0 {
		return 0
	}
	bounds := state.rows[slot].bounds
	if bounds.Y < state.viewport.Y {
		return bounds.Y - state.viewport.Y
	}
	if bottom := bounds.Y + bounds.Height; bottom > state.viewport.Y+state.viewport.Height {
		return bottom - (state.viewport.Y + state.viewport.Height)
	}
	return 0
}

func (state *ReorderListState) target(y float32) int {
	if state.stride <= 0 || len(state.rows) == 0 {
		return 0
	}
	return min(max(int((y-state.bounds.Y)/state.stride), 0), len(state.rows)-1)
}

func (state *ReorderListState) Pointer(event gfx.PointerEvent) (ReorderMove, gfx.EventResult) {
	if event.Kind == gfx.PointerDown {
		state.Cancel()
		if event.Button != gfx.PointerButtonPrimary {
			return ReorderMove{}, gfx.EventResult{}
		}
		for i, row := range state.rows {
			if row.id == event.Target && !row.disabled {
				state.active, state.from, state.to = row.id, i, i
				state.startY, state.lastY, state.pressed, state.dragging = event.Y, event.Y, true, false
				return ReorderMove{}, gfx.EventResult{Handled: true, Repaint: true, Focus: row.id}
			}
		}
		return ReorderMove{}, gfx.EventResult{}
	}
	if !state.pressed {
		return ReorderMove{}, gfx.EventResult{}
	}
	result := gfx.EventResult{Handled: true, Repaint: true}
	switch event.Kind {
	case gfx.PointerMove:
		state.lastY = event.Y
		delta := event.Y - state.startY
		if delta >= 3 || delta <= -3 {
			state.dragging = true
		}
		state.to = state.target(event.Y)
	case gfx.PointerUp:
		var move ReorderMove
		inside := event.X >= state.viewport.X && event.X <= state.viewport.X+state.viewport.Width && event.Y >= state.viewport.Y && event.Y <= state.viewport.Y+state.viewport.Height
		if state.dragging && inside {
			move = ReorderMove{ID: state.active, From: state.from, To: state.target(event.Y)}
		}
		result.Focus = state.active
		state.Cancel()
		return move, result
	case gfx.PointerExit:
		// The runtime releases capture after delivering Exit, including an Exit
		// whose Target still names the previously captured handle.
		state.Cancel()
	}
	return ReorderMove{}, result
}

func (state *ReorderListState) Key(event gfx.KeyEvent) (ReorderMove, gfx.EventResult) {
	if event.Release || event.Modifiers != 0 {
		return ReorderMove{}, gfx.EventResult{}
	}
	if state.pressed && event.Key == gfx.KeyEscape {
		id := state.active
		state.Cancel()
		return ReorderMove{}, gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}
	}
	for i, row := range state.rows {
		if row.id != event.Target || row.disabled {
			continue
		}
		to := i
		switch event.Key {
		case gfx.KeyUp:
			to = max(i-1, 0)
		case gfx.KeyDown:
			to = min(i+1, len(state.rows)-1)
		case gfx.KeyHome:
			to = 0
		case gfx.KeyEnd:
			to = len(state.rows) - 1
		default:
			return ReorderMove{}, gfx.EventResult{}
		}
		return ReorderMove{ID: row.id, From: i, To: to}, gfx.EventResult{Handled: true, Repaint: true, Focus: row.id, FocusVisible: true}
	}
	return ReorderMove{}, gfx.EventResult{}
}

// ReorderRowLayout reserves an independent drag handle and a content slot for
// icons, text, switches or other controls. Only the handle starts a drag.
type ReorderRowLayout struct{ Bounds, Handle, Content gfx.Rect }

type ReorderList struct {
	canvas           *gfx.Canvas
	theme            gfx.Theme
	state            *ReorderListState
	bounds, viewport gfx.Rect
	rowHeight, gap   float32
	index            int
}

func BeginReorderList(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, state *ReorderListState) ReorderList {
	state.rows = state.rows[:0]
	return ReorderList{canvas: canvas, theme: theme, state: state, bounds: bounds, viewport: bounds, rowHeight: 44, gap: 4}
}

// Viewport bounds valid drops and clips the insertion indicator when the caller
// has translated Bounds to scroll its content. Scrolling remains caller-owned.
func (list ReorderList) Viewport(bounds gfx.Rect) ReorderList { list.viewport = bounds; return list }
func (list ReorderList) RowHeight(height float32) ReorderList {
	if height > 0 {
		list.rowHeight = height
	}
	return list
}

// Row declares one row in display order and paints it. Declare every item, in
// order, on every paint: ReorderMove.From and To index this declaration, so a
// partial or virtualized paint would move the wrong item.
func (list *ReorderList) Row(id gfx.ID, label string, disabled bool) ReorderRowLayout {
	disabled = disabled || id.IsZero()
	bounds := gfx.Rect{X: list.bounds.X, Y: list.bounds.Y + float32(list.index)*(list.rowHeight+list.gap), Width: list.bounds.Width, Height: list.rowHeight}
	list.index++
	list.state.rows = append(list.state.rows, reorderHandle{id: id, disabled: disabled, bounds: bounds})
	fill, edge := list.theme.Background, list.theme.Border
	if list.state.active == id && list.state.pressed {
		fill, edge = list.theme.Muted, list.theme.Primary
	}
	list.canvas.RoundedBorder(bounds, list.theme.Metrics.RadiusControl, 1, edge, fill)
	handle := gfx.Rect{X: bounds.X + 4, Y: bounds.Y + 4, Width: 28, Height: max(bounds.Height-8, 0)}
	if list.canvas.FocusVisible() == id {
		list.canvas.RoundedBorder(handle, list.theme.Metrics.RadiusControl, 1, list.theme.Ring, gfx.Color{})
	}
	if !id.IsZero() {
		// PosInSet is left out on purpose: the set size is only known after the
		// last Row, and half of that pair describes a position with no set.
		list.canvas.HitWithOptions(id, handle, label, gfx.HitOptions{Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand, Disabled: disabled})
	}
	color := list.theme.MutedForeground
	if disabled {
		color.Alpha *= .5
	}
	for y := float32(-4); y <= 4; y += 4 {
		for _, x := range [...]float32{-2, 2} {
			list.canvas.Rounded(gfx.Rect{X: handle.X + handle.Width/2 + x - 1, Y: handle.Y + handle.Height/2 + y - 1, Width: 2, Height: 2}, 1, color)
		}
	}
	return ReorderRowLayout{Bounds: bounds, Handle: handle, Content: gfx.Rect{X: bounds.X + 40, Y: bounds.Y, Width: max(bounds.Width-48, 0), Height: bounds.Height}}
}

func (list *ReorderList) End() {
	state := list.state
	state.bounds, state.viewport, state.stride = list.bounds, list.viewport, list.rowHeight+list.gap
	if !state.pressed {
		return
	}
	found := false
	for i, row := range state.rows {
		if row.id == state.active && !row.disabled {
			state.from, found = i, true
			break
		}
	}
	if !found {
		state.Cancel()
		return
	}
	state.to = state.target(state.lastY)
	if !state.dragging || state.to == state.from {
		return
	}
	y := list.bounds.Y + float32(state.to)*state.stride - 2
	if state.to > state.from {
		y += state.stride
	}
	list.canvas.Clip(list.viewport)
	list.canvas.Rounded(gfx.Rect{X: list.bounds.X + 2, Y: y, Width: max(list.bounds.Width-4, 0), Height: 3}, 1.5, list.theme.Primary)
	list.canvas.Unclip()
}
