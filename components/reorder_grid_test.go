package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func reorderGridFixture() (ReorderGridState, *ui.Canvas) {
	var state ReorderGridState
	canvas := &ui.Canvas{}
	grid := BeginReorderGrid(canvas, ui.LightTheme(), ui.Rect{Width: 240, Height: 140}, &state)
	for index := 0; index < 4; index++ {
		x := float32(index%2) * 110
		y := float32(index/2) * 60
		bounds := ui.Rect{X: x, Y: y, Width: 100, Height: 50}
		handle := ui.Rect{X: x + 4, Y: y + 10, Width: 20, Height: 30}
		grid.Item(ui.Item(index+1), "Move", 0, index, bounds, handle, false)
	}
	grid.End()
	return state, canvas
}

func TestReorderGridCommitsOnlyOnDrop(t *testing.T) {
	state, _ := reorderGridFixture()
	move, result := state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	if move.Valid() || !result.Handled {
		t.Fatal("press should only pick up the handle")
	}
	move, _ = state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 85})
	if move.Valid() || !state.Dragging() || state.Target() != ui.Item(4) {
		t.Fatalf("move=%+v dragging=%v target=%v", move, state.Dragging(), state.Target())
	}
	move, result = state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 160, Y: 85})
	if !move.Valid() || move.ID != ui.Item(1) || move.Target != ui.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("drop=%+v result=%+v", move, result)
	}
}

func TestReorderGridRejectsContentAndOutsideDrops(t *testing.T) {
	state, _ := reorderGridFixture()
	if _, result := state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(99), X: 50, Y: 25}); result.Handled {
		t.Fatal("content started a drag")
	}
	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 260, Y: 180})
	move, _ := state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 260, Y: 180})
	if move.Valid() {
		t.Fatalf("outside drop committed: %+v", move)
	}
}

func TestReorderGridKeepsMovesInsideALane(t *testing.T) {
	var state ReorderGridState
	var canvas ui.Canvas
	grid := BeginReorderGrid(&canvas, ui.LightTheme(), ui.Rect{Width: 300, Height: 80}, &state)
	for index, lane := range []int{0, 1, 0} {
		x := float32(index) * 100
		bounds := ui.Rect{X: x, Width: 90, Height: 60}
		grid.Item(ui.Item(index+1), "Move", lane, index, bounds, ui.Rect{X: x + 4, Y: 10, Width: 20, Height: 30}, false)
	}
	grid.End()

	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 145, Y: 25})
	if target := state.Target(); !target.IsZero() {
		t.Fatalf("another lane produced target %v", target)
	}
	move, _ := state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 145, Y: 25})
	if move.Valid() {
		t.Fatalf("drop directly over another lane committed: %+v", move)
	}

	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 195, Y: 25})
	move, _ = state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 195, Y: 25})
	if move.Valid() {
		t.Fatalf("gap owned by another lane committed: %+v", move)
	}

	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 245, Y: 25})
	move, _ = state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 245, Y: 25})
	if !move.Valid() || move.Target != ui.Item(3) {
		t.Fatalf("same-lane drop=%+v", move)
	}
}

func TestReorderGridKeepsStableOrdersWhenTheSourceIsVirtualizedAway(t *testing.T) {
	var state ReorderGridState
	var canvas ui.Canvas
	grid := BeginReorderGrid(&canvas, ui.LightTheme(), ui.Rect{Width: 220, Height: 80}, &state)
	grid.Item(ui.Item(1), "One", 7, 10, ui.Rect{Width: 100, Height: 60}, ui.Rect{X: 4, Y: 10, Width: 20, Height: 30}, false)
	grid.Item(ui.Item(2), "Two", 7, 11, ui.Rect{X: 110, Width: 100, Height: 60}, ui.Rect{X: 114, Y: 10, Width: 20, Height: 30}, false)
	grid.End()
	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 25})

	// A virtualized repaint no longer declares the source, but the active lane and
	// caller order remain known to the drag state.
	canvas.Reset()
	grid = BeginReorderGrid(&canvas, ui.LightTheme(), ui.Rect{Width: 220, Height: 80}, &state)
	grid.Item(ui.Item(3), "Three", 7, 15, ui.Rect{X: 110, Width: 100, Height: 60}, ui.Rect{X: 114, Y: 10, Width: 20, Height: 30}, false)
	grid.End()
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 25})
	move, _ := state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 160, Y: 25})
	if !move.Valid() || move.ID != ui.Item(1) || move.Target != ui.Item(3) || move.From != 10 || move.To != 15 {
		t.Fatalf("virtualized move=%+v", move)
	}
}

func TestReorderGridKeyboardMovesWithinLane(t *testing.T) {
	var state ReorderGridState
	var canvas ui.Canvas
	grid := BeginReorderGrid(&canvas, ui.LightTheme(), ui.Rect{Width: 400, Height: 80}, &state)
	for index, lane := range []int{0, 1, 0, 0} {
		x := float32(index) * 100
		bounds := ui.Rect{X: x, Width: 90, Height: 60}
		grid.Item(ui.Item(index+1), "Move", lane, index, bounds, ui.Rect{X: x + 4, Y: 10, Width: 20, Height: 30}, false)
	}
	grid.End()

	move, result := state.Key(ui.KeyEvent{Target: ui.Item(3), Key: ui.KeyLeft})
	if !move.Valid() || move.Target != ui.Item(1) || move.From != 2 || move.To != 0 || !result.Handled || !result.FocusVisible {
		t.Fatalf("left move=%+v result=%+v", move, result)
	}
	move, result = state.Key(ui.KeyEvent{Target: ui.Item(1), Key: ui.KeyEnd})
	if !move.Valid() || move.Target != ui.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("end move=%+v result=%+v", move, result)
	}
	move, result = state.Key(ui.KeyEvent{Target: ui.Item(1), Key: ui.KeyUp})
	if move.Valid() || !result.Handled {
		t.Fatalf("boundary move=%+v result=%+v", move, result)
	}
}

func TestReorderGridKeyboardUsesLogicalOrderBeyondVisibleItems(t *testing.T) {
	var state ReorderGridState
	var canvas ui.Canvas
	grid := BeginReorderGrid(&canvas, ui.LightTheme(), ui.Rect{Width: 120, Height: 80}, &state)
	for index := 0; index < 4; index++ {
		grid.Order(ui.Item(index+1), 0, index, false)
	}
	// Only the first row is visible, but keyboard End still knows the complete
	// lane order and can move the focused item to the last slot.
	grid.Item(ui.Item(1), "One", 0, 0,
		ui.Rect{Width: 100, Height: 60},
		ui.Rect{X: 4, Y: 10, Width: 20, Height: 30}, false)
	grid.End()

	move, result := state.Key(ui.KeyEvent{Target: ui.Item(1), Key: ui.KeyEnd})
	if !move.Valid() || move.Target != ui.Item(4) || move.From != 0 || move.To != 3 || !result.Handled {
		t.Fatalf("virtual keyboard move=%+v result=%+v", move, result)
	}
}

func TestReorderGridEscapeCancelsHeldPointerDrag(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 85})
	move, result := state.Key(ui.KeyEvent{Target: ui.Item(1), Key: ui.KeyEscape})
	if move.Valid() || !result.Handled || !result.FocusVisible || state.Dragging() {
		t.Fatalf("escape move=%+v result=%+v dragging=%v", move, result, state.Dragging())
	}
}

func TestReorderGridCancelsOnExit(t *testing.T) {
	state, _ := reorderGridFixture()
	state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 85})
	state.Pointer(ui.PointerEvent{Kind: ui.PointerExit, Captured: true})
	move, _ := state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, X: 160, Y: 85})
	if state.Dragging() || move.Valid() {
		t.Fatal("an interrupted drag committed")
	}
}

func TestReorderGridPaintAllocatesNothingAfterWarmup(t *testing.T) {
	var state ReorderGridState
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		grid := BeginReorderGrid(&canvas, ui.DarkTheme(), ui.Rect{Width: 600, Height: 480}, &state).HandleHitSlop(10)
		for i := 0; i < 12; i++ {
			x := float32(i%3) * 190
			y := float32(i/3) * 90
			bounds := ui.Rect{X: x, Y: y, Width: 180, Height: 80}
			handle := ui.Rect{X: x + 145, Y: y + 20, Width: 28, Height: 40}
			grid.Order(ui.Item(i+1), i/6, i, false)
			grid.Item(ui.Item(i+1), "Move", i/6, i, bounds, handle, false)
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
	var canvas ui.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		grid := BeginReorderGrid(&canvas, ui.DarkTheme(), ui.Rect{Width: 600, Height: 480}, &state).HandleHitSlop(10)
		for i := 0; i < 12; i++ {
			x := float32(i%3) * 190
			y := float32(i/3) * 90
			bounds := ui.Rect{X: x, Y: y, Width: 180, Height: 80}
			handle := ui.Rect{X: x + 145, Y: y + 20, Width: 28, Height: 40}
			grid.Order(ui.Item(i+1), i/6, i, false)
			grid.Item(ui.Item(i+1), "Move", i/6, i, bounds, handle, false)
		}
		grid.End()
	}
}

func BenchmarkReorderGridDrag(b *testing.B) {
	state, _ := reorderGridFixture()
	b.ReportAllocs()
	for b.Loop() {
		state.Pointer(ui.PointerEvent{Kind: ui.PointerDown, Button: ui.PointerButtonPrimary, Target: ui.Item(1), X: 14, Y: 25})
		state.Pointer(ui.PointerEvent{Kind: ui.PointerMove, Captured: true, X: 160, Y: 85})
		state.Pointer(ui.PointerEvent{Kind: ui.PointerUp, Captured: true, X: 160, Y: 85})
	}
}
