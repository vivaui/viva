package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func reorderGridFixture() (ReorderGridState, *gfx.Canvas) {
	var state ReorderGridState
	canvas := &gfx.Canvas{}
	grid := BeginReorderGrid(canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 140}, &state)
	for index := 0; index < 4; index++ {
		x := float32(index%2) * 110
		y := float32(index/2) * 60
		bounds := gfx.Rect{X: x, Y: y, Width: 100, Height: 50}
		handle := gfx.Rect{X: x + 4, Y: y + 10, Width: 20, Height: 30}
		grid.Item(gfx.Item(index+1), "Move", 0, index, bounds, handle, false)
	}
	grid.End()
	return state, canvas
}

func TestReorderGridCommitsOnlyOnDrop(t *testing.T) {
	state, _ := reorderGridFixture()
	move, result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	if move.Valid() || !result.Handled {
		t.Fatal("press should only pick up the handle")
	}
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})
	if move.Valid() || !state.Dragging() || state.Target() != gfx.Item(4) {
		t.Fatalf("move=%+v dragging=%v target=%v", move, state.Dragging(), state.Target())
	}
	move, result = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 160, Y: 85})
	if !move.Valid() || move.ID != gfx.Item(1) || move.Target != gfx.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("drop=%+v result=%+v", move, result)
	}
}

func TestReorderGridRejectsContentAndOutsideDrops(t *testing.T) {
	state, _ := reorderGridFixture()
	if _, result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(99), X: 50, Y: 25}); result.Handled {
		t.Fatal("content started a drag")
	}
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 260, Y: 180})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 260, Y: 180})
	if move.Valid() {
		t.Fatalf("outside drop committed: %+v", move)
	}
}

func TestReorderGridRejectsDisabledTargets(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 80}, &state)
	grid.Item(gfx.Item(1), "A1", 0, 0, gfx.Rect{X: 0, Width: 90, Height: 60}, gfx.Rect{X: 4, Y: 10, Width: 20, Height: 30}, false)
	grid.Item(gfx.Item(2), "B disabled", 1, 1, gfx.Rect{X: 100, Width: 90, Height: 60}, gfx.Rect{X: 104, Y: 10, Width: 20, Height: 30}, true)
	grid.Item(gfx.Item(3), "A2", 0, 2, gfx.Rect{X: 200, Width: 90, Height: 60}, gfx.Rect{X: 204, Y: 10, Width: 20, Height: 30}, false)
	grid.End()

	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 180, Y: 25})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 180, Y: 25})
	if move.Valid() {
		t.Fatalf("drop over disabled target committed: %+v", move)
	}
}

func TestReorderGridKeepsMovesInsideALane(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 80}, &state)
	for index, lane := range []int{0, 1, 0} {
		x := float32(index) * 100
		bounds := gfx.Rect{X: x, Width: 90, Height: 60}
		grid.Item(gfx.Item(index+1), "Move", lane, index, bounds, gfx.Rect{X: x + 4, Y: 10, Width: 20, Height: 30}, false)
	}
	grid.End()

	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 145, Y: 25})
	if target := state.Target(); !target.IsZero() {
		t.Fatalf("another lane produced target %v", target)
	}
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 145, Y: 25})
	if move.Valid() {
		t.Fatalf("drop directly over another lane committed: %+v", move)
	}

	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 195, Y: 25})
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 195, Y: 25})
	if move.Valid() {
		t.Fatalf("gap owned by another lane committed: %+v", move)
	}

	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 245, Y: 25})
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 245, Y: 25})
	if !move.Valid() || move.Target != gfx.Item(3) {
		t.Fatalf("same-lane drop=%+v", move)
	}
}

func TestReorderGridKeepsStableOrdersWhenTheSourceIsVirtualizedAway(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 220, Height: 80}, &state)
	grid.Item(gfx.Item(1), "One", 7, 10, gfx.Rect{Width: 100, Height: 60}, gfx.Rect{X: 4, Y: 10, Width: 20, Height: 30}, false)
	grid.Item(gfx.Item(2), "Two", 7, 11, gfx.Rect{X: 110, Width: 100, Height: 60}, gfx.Rect{X: 114, Y: 10, Width: 20, Height: 30}, false)
	grid.End()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 25})

	// A virtualized repaint no longer declares the source, but the active lane and
	// caller order remain known to the drag state.
	canvas.Reset()
	grid = BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 220, Height: 80}, &state)
	grid.Item(gfx.Item(3), "Three", 7, 15, gfx.Rect{X: 110, Width: 100, Height: 60}, gfx.Rect{X: 114, Y: 10, Width: 20, Height: 30}, false)
	grid.End()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 25})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 160, Y: 25})
	if !move.Valid() || move.ID != gfx.Item(1) || move.Target != gfx.Item(3) || move.From != 10 || move.To != 15 {
		t.Fatalf("virtualized move=%+v", move)
	}
}

func TestReorderGridKeyboardMovesWithinLane(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, &state)
	for index, lane := range []int{0, 1, 0, 0} {
		x := float32(index) * 100
		bounds := gfx.Rect{X: x, Width: 90, Height: 60}
		grid.Item(gfx.Item(index+1), "Move", lane, index, bounds, gfx.Rect{X: x + 4, Y: 10, Width: 20, Height: 30}, false)
	}
	grid.End()

	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(3), Key: gfx.KeyLeft})
	if !move.Valid() || move.Target != gfx.Item(1) || move.From != 2 || move.To != 0 || !result.Handled || !result.FocusVisible {
		t.Fatalf("left move=%+v result=%+v", move, result)
	}
	move, result = state.Key(gfx.KeyEvent{Target: gfx.Item(1), Key: gfx.KeyEnd})
	if !move.Valid() || move.Target != gfx.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("end move=%+v result=%+v", move, result)
	}
	move, result = state.Key(gfx.KeyEvent{Target: gfx.Item(1), Key: gfx.KeyUp})
	if move.Valid() || !result.Handled {
		t.Fatalf("boundary move=%+v result=%+v", move, result)
	}
}

func TestReorderGridKeyboardCancelsHeldPointerTransaction(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})

	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(1), Key: gfx.KeyEnd})
	if !move.Valid() || !result.Handled {
		t.Fatalf("keyboard move=%+v result=%+v", move, result)
	}
	if _, held := state.PointerPosition(); held || state.Dragging() {
		t.Fatal("keyboard reorder left the pointer transaction active")
	}
}

func TestReorderGridKeyboardUsesLogicalOrderBeyondVisibleItems(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 120, Height: 80}, &state)
	for index := 0; index < 4; index++ {
		grid.Order(gfx.Item(index+1), 0, index, false)
	}
	// Only the first row is visible, but keyboard End still knows the complete
	// lane order and can move the focused item to the last slot.
	grid.Item(gfx.Item(1), "One", 0, 0,
		gfx.Rect{Width: 100, Height: 60},
		gfx.Rect{X: 4, Y: 10, Width: 20, Height: 30}, false)
	grid.End()

	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(1), Key: gfx.KeyEnd})
	if !move.Valid() || move.Target != gfx.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("virtual keyboard move=%+v result=%+v", move, result)
	}
}

func TestReorderGridEscapeCancelsHeldPointerDrag(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})
	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(1), Key: gfx.KeyEscape})
	if move.Valid() || !result.Handled || !result.FocusVisible || state.Dragging() {
		t.Fatalf("escape move=%+v result=%+v dragging=%v", move, result, state.Dragging())
	}
}

func TestReorderGridEndClearsTargetWhenReflowMakesItInvalid(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})
	if state.Target().IsZero() {
		t.Fatal("expected an initial target")
	}

	var canvas gfx.Canvas
	grid := BeginReorderGrid(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 140}, &state)
	// The source still exists logically, but virtualization/reflow has moved every
	// pointer target out of the painted viewport.
	grid.Order(gfx.Item(1), 0, 0, false)
	grid.End()
	if !state.Target().IsZero() {
		t.Fatalf("stale target remains after reflow: %v", state.Target())
	}
}

func TestReorderGridCancelsOnExit(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerExit, Captured: true})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, X: 160, Y: 85})
	if state.Dragging() || move.Valid() {
		t.Fatal("an interrupted drag committed")
	}
}

func TestReorderGridPaintAllocatesNothingAfterWarmup(t *testing.T) {
	var state ReorderGridState
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		grid := BeginReorderGrid(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 600, Height: 480}, &state).HandleHitSlop(10)
		for i := 0; i < 12; i++ {
			x := float32(i%3) * 190
			y := float32(i/3) * 90
			bounds := gfx.Rect{X: x, Y: y, Width: 180, Height: 80}
			handle := gfx.Rect{X: x + 145, Y: y + 20, Width: 28, Height: 40}
			grid.Order(gfx.Item(i+1), i/6, i, false)
			grid.Item(gfx.Item(i+1), "Move", i/6, i, bounds, handle, false)
		}
		grid.End()
	}
	paint()
	if got := testing.AllocsPerRun(100, paint); got != 0 {
		t.Fatalf("%v allocations per paint", got)
	}
}

func BenchmarkReorderGridPaint(b *testing.B) {
	var state ReorderGridState
	var canvas gfx.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		grid := BeginReorderGrid(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 600, Height: 480}, &state).HandleHitSlop(10)
		for i := 0; i < 12; i++ {
			x := float32(i%3) * 190
			y := float32(i/3) * 90
			bounds := gfx.Rect{X: x, Y: y, Width: 180, Height: 80}
			handle := gfx.Rect{X: x + 145, Y: y + 20, Width: 28, Height: 40}
			grid.Order(gfx.Item(i+1), i/6, i, false)
			grid.Item(gfx.Item(i+1), "Move", i/6, i, bounds, handle, false)
		}
		grid.End()
	}
}

func BenchmarkReorderGridDrag(b *testing.B) {
	state, _ := reorderGridFixture()
	b.ReportAllocs()
	for b.Loop() {
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 14, Y: 25})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 160, Y: 85})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 160, Y: 85})
	}
}
