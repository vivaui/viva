package components

import "github.com/vivaui/viva/gfx"

// ReorderGridMove describes one completed move in an arbitrary two-dimensional
// layout. ID is the item being moved and Target is the slot it was dropped on.
// From and To are caller-owned stable order values, so virtualized layouts may
// declare only the visible items.
type ReorderGridMove struct {
	ID, Target gfx.ID
	From, To   int
}

func (move ReorderGridMove) Valid() bool {
	return !move.ID.IsZero() && !move.Target.IsZero() && move.From >= 0 && move.To >= 0 && move.ID != move.Target && move.From != move.To
}

type reorderGridItem struct {
	id       gfx.ID
	lane     int
	order    int
	disabled bool
	visible  bool
	bounds   gfx.Rect
}

// ReorderGridState owns drag geometry for arbitrary rows/cards. The caller owns
// item data and applies a ReorderGridMove only after a valid drop. lane prevents
// items from crossing independent groups while still allowing one state to serve
// a page that contains several groups.
//
// The active lane and source order are captured at pointer-down. That lets a
// virtualized caller auto-scroll until the source is no longer painted without
// losing the drag.
type ReorderGridState struct {
	items             []reorderGridItem
	indices           map[gfx.ID]int
	viewport          gfx.Rect
	active, target    gfx.ID
	lane, from, to    int
	start, last       gfx.Point
	pressed, dragging bool
}

func (state *ReorderGridState) Dragging() bool { return state.dragging }
func (state *ReorderGridState) Active() gfx.ID { return state.active }

// PointerPosition is the last captured pointer position while a handle is held.
// It lets a caller-owned scroll area keep auto-scrolling between pointer events.
func (state *ReorderGridState) PointerPosition() (gfx.Point, bool) {
	return state.last, state.pressed
}

func (state *ReorderGridState) Target() gfx.ID {
	if !state.dragging {
		return gfx.ID{}
	}
	return state.target
}

func (state *ReorderGridState) Cancel() {
	state.active, state.target = gfx.ID{}, gfx.ID{}
	state.from, state.to = -1, -1
	state.pressed, state.dragging = false, false
}

func reorderGridContains(rect gfx.Rect, x, y float32) bool {
	return x >= rect.X && x <= rect.X+rect.Width && y >= rect.Y && y <= rect.Y+rect.Height
}

func reorderGridCenter(rect gfx.Rect) gfx.Point {
	return gfx.Point{X: rect.X + rect.Width/2, Y: rect.Y + rect.Height/2}
}

// targetAt chooses the item under the pointer when possible and otherwise the
// nearest visible center. Disabled items and items in another lane own their
// geometry but are not valid targets, so drops over them are rejected.
func (state *ReorderGridState) targetAt(x, y float32) int {
	// A pointer directly over another lane is an invalid drop, not a request to
	// snap to whichever item in the active lane happens to be geographically
	// nearest.
	for index, item := range state.items {
		if !item.visible || !reorderGridContains(item.bounds, x, y) {
			continue
		}
		if item.disabled || item.lane != state.lane {
			return -1
		}
		return index
	}

	// In gaps between items, use the nearest item overall to decide which lane
	// owns that space. A gap closer to another lane is invalid rather than
	// reaching back to a distant item in the active lane.
	best, bestDistance := -1, float32(0)
	for index, item := range state.items {
		if !item.visible {
			continue
		}
		center := reorderGridCenter(item.bounds)
		dx, dy := center.X-x, center.Y-y
		distance := dx*dx + dy*dy
		if best < 0 || distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	if best < 0 || state.items[best].disabled || state.items[best].lane != state.lane {
		return -1
	}
	return best
}

func (state *ReorderGridState) Pointer(event gfx.PointerEvent) (ReorderGridMove, gfx.EventResult) {
	if event.Kind == gfx.PointerDown {
		state.Cancel()
		if event.Button != gfx.PointerButtonPrimary {
			return ReorderGridMove{}, gfx.EventResult{}
		}
		for _, item := range state.items {
			if item.id == event.Target && !item.disabled {
				state.active, state.target = item.id, item.id
				state.lane, state.from, state.to = item.lane, item.order, item.order
				state.start = gfx.Point{X: event.X, Y: event.Y}
				state.last, state.pressed = state.start, true
				return ReorderGridMove{}, gfx.EventResult{Handled: true, Repaint: true, Focus: item.id}
			}
		}
		return ReorderGridMove{}, gfx.EventResult{}
	}
	if !state.pressed {
		return ReorderGridMove{}, gfx.EventResult{}
	}

	result := gfx.EventResult{Handled: true, Repaint: true}
	switch event.Kind {
	case gfx.PointerMove:
		state.last = gfx.Point{X: event.X, Y: event.Y}
		dx, dy := event.X-state.start.X, event.Y-state.start.Y
		if dx*dx+dy*dy >= 9 {
			state.dragging = true
		}
		if state.dragging {
			if index := state.targetAt(event.X, event.Y); index >= 0 {
				item := state.items[index]
				state.target, state.to = item.id, item.order
			} else {
				state.target, state.to = gfx.ID{}, -1
			}
		}
	case gfx.PointerUp:
		var move ReorderGridMove
		inside := reorderGridContains(state.viewport, event.X, event.Y)
		if state.dragging && inside {
			if index := state.targetAt(event.X, event.Y); index >= 0 {
				item := state.items[index]
				state.target, state.to = item.id, item.order
				move = ReorderGridMove{ID: state.active, Target: item.id, From: state.from, To: item.order}
			}
		}
		result.Focus = state.active
		state.Cancel()
		return move, result
	case gfx.PointerExit:
		state.Cancel()
	}
	return ReorderGridMove{}, result
}

// Key reorders the focused handle within the items of its lane. Left/Up move to
// the previous declared item, Right/Down to the next, and Home/End to the first
// or last. The caller applies the returned move and repaints; focus follows the
// stable item ID to its new position.
func (state *ReorderGridState) Key(event gfx.KeyEvent) (ReorderGridMove, gfx.EventResult) {
	if event.Release || event.Modifiers != 0 {
		return ReorderGridMove{}, gfx.EventResult{}
	}
	if state.pressed && event.Key == gfx.KeyEscape {
		id := state.active
		state.Cancel()
		return ReorderGridMove{}, gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}
	}

	for _, item := range state.items {
		if item.id != event.Target || item.disabled {
			continue
		}
		target := item
		found := false
		switch event.Key {
		case gfx.KeyLeft, gfx.KeyUp:
			for _, candidate := range state.items {
				if candidate.disabled || candidate.lane != item.lane || candidate.order >= item.order {
					continue
				}
				if !found || candidate.order > target.order {
					target, found = candidate, true
				}
			}
		case gfx.KeyRight, gfx.KeyDown:
			for _, candidate := range state.items {
				if candidate.disabled || candidate.lane != item.lane || candidate.order <= item.order {
					continue
				}
				if !found || candidate.order < target.order {
					target, found = candidate, true
				}
			}
		case gfx.KeyHome:
			for _, candidate := range state.items {
				if candidate.disabled || candidate.lane != item.lane {
					continue
				}
				if !found || candidate.order < target.order {
					target, found = candidate, true
				}
			}
		case gfx.KeyEnd:
			for _, candidate := range state.items {
				if candidate.disabled || candidate.lane != item.lane {
					continue
				}
				if !found || candidate.order > target.order {
					target, found = candidate, true
				}
			}
		default:
			return ReorderGridMove{}, gfx.EventResult{}
		}
		if !found {
			target = item
		}
		move := ReorderGridMove{ID: item.id, Target: target.id, From: item.order, To: target.order}
		state.Cancel()
		return move, gfx.EventResult{Handled: true, Repaint: true, Focus: item.id, FocusVisible: true}
	}
	return ReorderGridMove{}, gfx.EventResult{}
}

// ReorderGrid records arbitrary item rectangles and paints only the framework's
// drag affordances. The caller paints the actual rows/cards.
type ReorderGrid struct {
	canvas  *gfx.Canvas
	theme   gfx.Theme
	state   *ReorderGridState
	hitSlop float32
}

func BeginReorderGrid(canvas *gfx.Canvas, theme gfx.Theme, viewport gfx.Rect, state *ReorderGridState) ReorderGrid {
	state.items = state.items[:0]
	if state.indices == nil {
		state.indices = make(map[gfx.ID]int)
	} else {
		clear(state.indices)
	}
	state.viewport = viewport
	return ReorderGrid{canvas: canvas, theme: theme, state: state}
}

// HandleHitSlop expands each drag handle's interactive area without changing
// the six-dot glyph or the caller's layout. The expanded hit is clamped to the
// item's bounds.
func (grid ReorderGrid) HandleHitSlop(slop float32) ReorderGrid {
	grid.hitSlop = max(slop, 0)
	return grid
}

// Order declares one item in the caller's logical order without painting it.
// Virtualized layouts can declare every item through Order, then call Item only
// for the rows/cards currently on screen. Pointer targeting uses visible items;
// keyboard moves use the complete logical order.
func (grid *ReorderGrid) Order(id gfx.ID, lane, order int, disabled bool) {
	if id.IsZero() {
		return
	}
	if index, found := grid.state.indices[id]; found {
		grid.state.items[index].lane = lane
		grid.state.items[index].order = order
		grid.state.items[index].disabled = disabled
		return
	}
	grid.state.indices[id] = len(grid.state.items)
	grid.state.items = append(grid.state.items, reorderGridItem{
		id: id, lane: lane, order: order, disabled: disabled,
	})
}

// Item declares one visible draggable item. lane separates independent reorder
// groups; order is the caller's stable logical position. A prior Order call is
// updated in place, so virtualized callers can keep a complete keyboard model
// while exposing pointer geometry only for what is painted.
func (grid *ReorderGrid) Item(id gfx.ID, label string, lane, order int, bounds, handle gfx.Rect, disabled bool) {
	disabled = disabled || id.IsZero()
	if id.IsZero() {
		return
	}
	if index, found := grid.state.indices[id]; found {
		item := &grid.state.items[index]
		item.lane, item.order, item.disabled = lane, order, disabled
		item.visible, item.bounds = true, bounds
	} else {
		grid.state.indices[id] = len(grid.state.items)
		grid.state.items = append(grid.state.items, reorderGridItem{
			id: id, lane: lane, order: order, disabled: disabled, visible: true, bounds: bounds,
		})
	}
	hit := handle
	if grid.hitSlop > 0 {
		hit.X = max(bounds.X, hit.X-grid.hitSlop)
		hit.Y = max(bounds.Y, hit.Y-grid.hitSlop)
		right := min(bounds.X+bounds.Width, handle.X+handle.Width+grid.hitSlop)
		bottom := min(bounds.Y+bounds.Height, handle.Y+handle.Height+grid.hitSlop)
		hit.Width = max(right-hit.X, 0)
		hit.Height = max(bottom-hit.Y, 0)
	}
	grid.canvas.HitWithOptions(id, hit, label, gfx.HitOptions{
		Role: gfx.RoleButton, Cursor: gfx.PointerCursorHand, Disabled: disabled,
	})
	if grid.canvas.FocusVisible() == id {
		grid.canvas.RoundedBorder(handle, grid.theme.Metrics.RadiusControl, 1, grid.theme.Ring, gfx.Color{})
	}
	color := grid.theme.MutedForeground
	if disabled {
		color.Alpha *= .5
	}
	if grid.state.active == id && grid.state.pressed {
		color = grid.theme.Primary
	}
	const dot = float32(2.5)
	for y := float32(-5); y <= 5; y += 5 {
		for _, x := range [...]float32{-2.5, 2.5} {
			grid.canvas.Rounded(gfx.Rect{
				X:     handle.X + handle.Width/2 + x - dot/2,
				Y:     handle.Y + handle.Height/2 + y - dot/2,
				Width: dot, Height: dot,
			}, dot/2, color)
		}
	}
}

func (grid *ReorderGrid) End() {
	state := grid.state
	if !state.pressed || !state.dragging {
		return
	}
	if index := state.targetAt(state.last.X, state.last.Y); index >= 0 {
		item := state.items[index]
		state.target, state.to = item.id, item.order
	} else {
		state.target, state.to = gfx.ID{}, -1
	}

	// Surface styling (including elevation and target indication) is caller-owned:
	// rows and cards know their exact fill/radius and can paint shadows before
	// their surface, preserving Canvas.Elevate's ordering contract.
}
