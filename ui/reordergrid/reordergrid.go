// Package reordergrid is the opt-in page control for the arbitrary two-
// dimensional reorder surface of package components. It wraps
// components.BeginReorderGrid and components.ReorderGridState in a ui.Control:
//
//	grid := &reordergrid.Grid{
//		Draw: m.drawHosts, HandleHitSlop: 8,
//		Move: m.applyHostMove,
//	}
//	...
//	reordergrid.View(c, "Host order", grid).FillWidth().Height(220)
//
// The caller owns the items, their logical order and their rectangles, and
// paints each cell inside Draw; the wrapper begins the component, gives every
// declared item an identity inside the control so the page routes its events to
// the wrapper, and translates mouse and keyboard moves back to the caller's
// identities. The caller applies a Move only after a valid drop or key.
//
// Caller layouts map their cells with ItemBounds, or read the viewport handed
// to Draw. Pointer, key and wheel callbacks receive the caller's targets for
// observation; the wrapper has already updated the drag state before they run.
//
// It is a separate package so an application that does not import it pays
// nothing for it. Keep a Grid in the model beside its items: its handlers, draw
// function and item identities are made once, so a warm paint allocates nothing.
// Do not copy a Grid after it has been painted.
package reordergrid

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// Move is one completed move, with the caller's item identities. From and To
// are the caller's stable order values.
type Move = components.ReorderGridMove

// Layout is one paint's geometry and the way the callback declares its items.
// It is valid only during the Draw callback; keep the Viewport, not the Layout.
type Layout struct {
	// Viewport is the control's rectangle in page coordinates.
	Viewport gfx.Rect

	owner *Grid
	grid  *components.ReorderGrid
}

// Order declares one item in the caller's logical order without painting it.
// Virtualized layouts declare every item here, then call Item for the cells
// currently on screen.
func (l Layout) Order(id gfx.ID, lane, order int, disabled bool) {
	if l.grid == nil || l.owner == nil {
		return
	}
	l.grid.Order(l.owner.part(id), lane, order, disabled)
}

// Item declares one visible draggable item and paints the framework's handle.
// id is the caller's identity; the handle hit is given an identity inside the
// control so events reach the wrapper.
func (l Layout) Item(id gfx.ID, label string, lane, order int, bounds, handle gfx.Rect, disabled bool) {
	if l.grid == nil || l.owner == nil {
		return
	}
	l.grid.Item(l.owner.part(id), label, lane, order, bounds, handle, disabled)
}

// ItemBounds maps a cell of a fixed-column grid to page coordinates. It returns
// an empty rectangle for a non-positive column count, a negative index or a
// viewport too narrow to hold the columns.
func (l Layout) ItemBounds(columns, index int, gap, height float32) gfx.Rect {
	if columns <= 0 || index < 0 || height <= 0 || l.Viewport.Width <= 0 {
		return gfx.Rect{}
	}
	width := (l.Viewport.Width - gap*float32(columns-1)) / float32(columns)
	if width <= 0 {
		return gfx.Rect{}
	}
	column, row := index%columns, index/columns
	return gfx.Rect{
		X:     l.Viewport.X + float32(column)*(width+gap),
		Y:     l.Viewport.Y + float32(row)*(height+gap),
		Width: width, Height: height,
	}
}

// Dragging reports whether a handle is being dragged.
func (l Layout) Dragging() bool {
	if l.owner == nil {
		return false
	}
	return l.owner.State.Dragging()
}

// Active is the caller identity of the item held by the pointer, or zero.
func (l Layout) Active() gfx.ID {
	if l.owner == nil {
		return gfx.ID{}
	}
	return l.owner.caller(l.owner.State.Active())
}

// Target is the caller identity under the pointer during a drag, or zero.
func (l Layout) Target() gfx.ID {
	if l.owner == nil {
		return gfx.ID{}
	}
	return l.owner.caller(l.owner.State.Target())
}

// Grid is one reorder grid's state and page control. Keep it in the model; the
// caller paints the cells in Draw and applies a Move after a valid drop or key.
type Grid struct {
	// State is the caller-owned drag geometry. Use the zero value for a new grid.
	State components.ReorderGridState
	// HandleHitSlop expands each handle's interactive area without changing the
	// caller's layout. The expansion is clamped to the item's bounds.
	HandleHitSlop float32
	// Draw paints the caller's cells and declares them through layout. It runs
	// inside the control, after the component has cleared its per-paint geometry.
	Draw func(*ui.DrawContext, Layout)
	// Move receives every completed pointer or keyboard move with the caller's
	// identities. Apply it to the caller's order, then let the next paint run.
	Move func(Move)

	// Pointer receives every pointer event aimed at the control, with its target
	// translated to the caller's identity when it is one of them.
	Pointer func(gfx.PointerEvent) gfx.EventResult
	// Key receives the keys pressed while an item handle has the focus.
	Key func(gfx.KeyEvent) gfx.EventResult
	// Scroll receives the wheel over the control. Reordering itself does not
	// consume the wheel; a caller-owned scroll area may use it to auto-scroll.
	Scroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
	painted  components.ReorderGrid
	layout   Layout
	viewport gfx.Rect

	itemParts   map[gfx.ID]gfx.ID
	itemCallers []gfx.ID
}

// View declares the grid at c. label names the control for assistive
// technology. Give it a size; it is a ui.Control painted through Draw.
//
// Declare the view on its own, not inside a Memo.
func View(c ui.C, label string, grid *Grid) ui.Box {
	if grid == nil {
		panic("reordergrid: View needs its Grid")
	}
	grid.prepare()
	return ui.Control(c, label, &grid.handlers).Draw(grid.drawFn)
}

// View declares the grid at c without naming it separately, for a page that
// keeps the Grid in its model.
func (g *Grid) View(c ui.C, label string) ui.Box {
	return View(c, label, g)
}

// Viewport is the rectangle painted last, zero until View has painted once.
func (g *Grid) Viewport() gfx.Rect { return g.viewport }

// ItemBounds maps a cell of a fixed-column grid for the viewport painted last.
func (g *Grid) ItemBounds(columns, index int, gap, height float32) gfx.Rect {
	return (Layout{Viewport: g.viewport}).ItemBounds(columns, index, gap, height)
}

// Active is the caller identity of the item held by the pointer, or zero.
func (g *Grid) Active() gfx.ID { return g.caller(g.State.Active()) }

// Target is the caller identity under the pointer during a drag, or zero.
func (g *Grid) Target() gfx.ID { return g.caller(g.State.Target()) }

// Dragging reports whether a handle is being dragged.
func (g *Grid) Dragging() bool { return g.State.Dragging() }

// PointerPosition is the last captured pointer position while a handle is held,
// for a caller-owned scroll area that auto-scrolls between pointer events.
func (g *Grid) PointerPosition() (gfx.Point, bool) { return g.State.PointerPosition() }

// Cancel abandons the current drag without a move.
func (g *Grid) Cancel() { g.State.Cancel() }

func (g *Grid) prepare() {
	if g.prepared {
		return
	}
	g.prepared = true
	g.handlers.Pointer = g.pointer
	g.handlers.Key = g.key
	g.handlers.Scroll = g.scroll
	g.drawFn = g.draw
}

func (g *Grid) draw(d *ui.DrawContext) {
	g.viewport = d.Rect
	g.painted = components.BeginReorderGrid(d.Canvas, *d.Theme, d.Rect, &g.State)
	if g.HandleHitSlop > 0 {
		g.painted = g.painted.HandleHitSlop(g.HandleHitSlop)
	}
	g.layout = Layout{Viewport: d.Rect, owner: g, grid: &g.painted}
	if g.Draw != nil {
		g.Draw(d, g.layout)
	}
	g.painted.End()
}

// part gives the caller's item identity a stable identity inside the control,
// so the page routes the handle's events to this control's handlers. Identities
// are assigned once per item and kept across reorder and virtualized paints.
func (g *Grid) part(id gfx.ID) gfx.ID {
	if id.IsZero() {
		return gfx.ID{}
	}
	if part, ok := g.itemParts[id]; ok {
		return part
	}
	parts := g.handlers.Parts()
	if parts.IsZero() {
		// Painted outside a declared Control: keep the caller's identity so the
		// component still paints, though no event can be routed here.
		return id
	}
	if g.itemParts == nil {
		g.itemParts = make(map[gfx.ID]gfx.ID)
	}
	part := parts.At(len(g.itemCallers))
	g.itemParts[id] = part
	g.itemCallers = append(g.itemCallers, id)
	return part
}

// caller translates an identity inside the control back to the caller's.
func (g *Grid) caller(part gfx.ID) gfx.ID {
	if part.IsZero() {
		return part
	}
	if index, ok := part.In(g.handlers.Parts()); ok && index >= 0 && index < len(g.itemCallers) {
		return g.itemCallers[index]
	}
	return part
}

func (g *Grid) pointer(event gfx.PointerEvent) gfx.EventResult {
	move, result := g.State.Pointer(event)
	if move.Valid() {
		move.ID, move.Target = g.caller(move.ID), g.caller(move.Target)
		if g.Move != nil {
			g.Move(move)
		}
	}
	if g.Pointer != nil {
		event.Target = g.caller(event.Target)
		result = mergeResult(result, g.Pointer(event))
	}
	return result
}

func (g *Grid) key(event gfx.KeyEvent) gfx.EventResult {
	move, result := g.State.Key(event)
	if move.Valid() {
		move.ID, move.Target = g.caller(move.ID), g.caller(move.Target)
		if g.Move != nil {
			g.Move(move)
		}
	}
	if g.Key != nil {
		event.Target = g.caller(event.Target)
		result = mergeResult(result, g.Key(event))
	}
	return result
}

func (g *Grid) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if g.Scroll == nil {
		return gfx.EventResult{}
	}
	event.Target = g.caller(event.Target)
	return g.Scroll(event)
}

// mergeResult keeps the wrapper's handling and focus and adds the caller
// callback's handled and repaint requests. The result's focus stays on the
// identity inside the control so later keys reach the wrapper.
func mergeResult(wrapper, callback gfx.EventResult) gfx.EventResult {
	wrapper.Handled = wrapper.Handled || callback.Handled
	wrapper.Repaint = wrapper.Repaint || callback.Repaint
	return wrapper
}
