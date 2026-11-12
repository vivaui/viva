package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func reorderFixture() (ReorderListState, *gfx.Canvas) {
	var state ReorderListState
	canvas := &gfx.Canvas{}
	list := BeginReorderList(canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 300, Height: 140}, &state)
	for i := 0; i < 3; i++ {
		list.Row(gfx.Item(i+1), "Move item", false)
	}
	list.End()
	return state, canvas
}

func TestReorderListCommitsOnlyAtDropAndCancelsOutside(t *testing.T) {
	state, _ := reorderFixture()
	move, result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 24, Y: 40})
	if move.Valid() || !result.Handled {
		t.Fatal("press should only pick up the handle")
	}
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 138})
	if move.Valid() || !state.Dragging() {
		t.Fatal("drag committed before pointer release")
	}
	move, result = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 24, Y: 138})
	if !move.Valid() || move.From != 0 || move.To != 2 || move.ID != gfx.Item(1) || !result.Handled || state.Dragging() {
		t.Fatalf("drop: %+v %+v", move, result)
	}
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(2), X: 24, Y: 90})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 500, Y: 200})
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 500, Y: 200})
	if move.Valid() || state.Dragging() {
		t.Fatal("drop outside the list reordered data")
	}
}

func TestReorderListKeyboardAndEscapeKeepStableIdentity(t *testing.T) {
	state, _ := reorderFixture()
	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(2), Key: gfx.KeyDown})
	if !move.Valid() || move.From != 1 || move.To != 2 || result.Focus != gfx.Item(2) || !result.FocusVisible {
		t.Fatal("keyboard reorder lost stable focus")
	}
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(2), X: 24, Y: 90})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 130})
	move, result = state.Key(gfx.KeyEvent{Key: gfx.KeyEscape})
	if move.Valid() || !result.Handled || state.Dragging() {
		t.Fatal("Escape failed to cancel the move")
	}
	move, _ = state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 24, Y: 130})
	if move.Valid() {
		t.Fatal("cancelled drag committed on release")
	}
}

func TestReorderListCancelsWhenTheRuntimeDropsCapture(t *testing.T) {
	for _, captured := range []bool{false, true} {
		state, _ := reorderFixture()
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 24, Y: 40})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 138})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerExit, Captured: captured})
		move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, X: 24, Y: 138})
		if state.Dragging() || move.Valid() {
			t.Fatalf("capture=%v: an interrupted drag committed on release", captured)
		}
	}
	state, _ := reorderFixture()
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 24, Y: 40})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 138})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonSecondary, Target: gfx.Item(2), X: 24, Y: 90})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Button: gfx.PointerButtonSecondary, X: 24, Y: 138})
	if state.Dragging() || move.Valid() {
		t.Fatal("a replacement press retained the old drag")
	}
}

func TestReorderListUsesScrolledCoordinatesAndDoesNotStartFromContent(t *testing.T) {
	var state ReorderListState
	var canvas gfx.Canvas
	viewport := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 90}
	list := BeginReorderList(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: -28, Width: 300, Height: 140}, &state).Viewport(viewport)
	for i := 0; i < 3; i++ {
		list.Row(gfx.Item(i+1), "Move", false)
	}
	list.End()
	_, result := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(99), X: 90, Y: 40})
	if result.Handled {
		t.Fatal("content control started a reorder")
	}
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(2), X: 24, Y: 40})
	state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 85})
	move, _ := state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 24, Y: 85})
	if !move.Valid() || move.From != 1 || move.To != 2 {
		t.Fatalf("scrolled move: %+v", move)
	}
}

func TestReorderListPaintAllocatesNothingAfterWarmup(t *testing.T) {
	var state ReorderListState
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		list := BeginReorderList(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 300, Height: 200}, &state)
		for i := 0; i < 4; i++ {
			list.Row(gfx.Item(i+1), "Move", false)
		}
		list.End()
	}
	paint()
	if got := testing.AllocsPerRun(100, paint); got != 0 {
		t.Fatalf("%v allocations per paint", got)
	}
}

func BenchmarkReorderListPaint(b *testing.B) {
	var state ReorderListState
	var canvas gfx.Canvas
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		list := BeginReorderList(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 400, Height: 480}, &state)
		for i := 0; i < 10; i++ {
			list.Row(gfx.Item(i+1), "Move", false)
		}
		list.End()
	}
}

func BenchmarkReorderListDrag(b *testing.B) {
	state, _ := reorderFixture()
	b.ReportAllocs()
	for b.Loop() {
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, Target: gfx.Item(1), X: 24, Y: 40})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerMove, Captured: true, X: 24, Y: 138})
		state.Pointer(gfx.PointerEvent{Kind: gfx.PointerUp, Captured: true, X: 24, Y: 138})
	}
}

func TestReorderListReportsRowBoundsAndTheScrollThatRevealsThem(t *testing.T) {
	var state ReorderListState
	var canvas gfx.Canvas
	viewport := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 90}
	// The caller has scrolled the content up by 100: the first rows sit above the
	// viewport and the last ones below it.
	list := BeginReorderList(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: -80, Width: 300, Height: 480}, &state).Viewport(viewport)
	for i := 0; i < 10; i++ {
		list.Row(gfx.Item(i+1), "Move", false)
	}
	list.End()

	if _, ok := state.RowBounds(gfx.Item(99)); ok {
		t.Fatal("RowBounds reported a row that was not declared")
	}
	first, ok := state.RowBounds(gfx.Item(1))
	if !ok || first.Y != -80 || first.Height != 44 {
		t.Fatalf("first row bounds = %+v, %v", first, ok)
	}
	if delta := state.RevealScroll(0); delta != first.Y-viewport.Y {
		t.Fatalf("row above the viewport reveals by %v, want %v", delta, first.Y-viewport.Y)
	}
	if delta := state.RevealScroll(3); delta != 0 {
		t.Fatalf("a row inside the viewport reveals by %v, want 0", delta)
	}
	last, _ := state.RowBounds(gfx.Item(10))
	if delta, want := state.RevealScroll(9), last.Y+last.Height-(viewport.Y+viewport.Height); delta != want {
		t.Fatalf("row below the viewport reveals by %v, want %v", delta, want)
	}
}

func TestReorderListKeyboardMoveNamesTheRowItsScrollMustReveal(t *testing.T) {
	var state ReorderListState
	var canvas gfx.Canvas
	viewport := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 90}
	list := BeginReorderList(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: -80, Width: 300, Height: 480}, &state).Viewport(viewport)
	for i := 0; i < 10; i++ {
		list.Row(gfx.Item(i+1), "Move", false)
	}
	list.End()

	move, result := state.Key(gfx.KeyEvent{Target: gfx.Item(2), Key: gfx.KeyEnd})
	if !move.Valid() || move.From != 1 || move.To != 9 || result.Focus != gfx.Item(2) || !result.FocusVisible {
		t.Fatalf("keyboard move = %+v, result = %+v", move, result)
	}
	delta := state.RevealScroll(move.To)
	if delta <= 0 {
		t.Fatalf("reveal delta = %v, want the slot the move landed on scrolled into view", delta)
	}
	slot := state.rows[move.To].bounds
	revealed := gfx.Rect{Y: slot.Y - delta, Height: slot.Height}
	if revealed.Y < viewport.Y || revealed.Y+revealed.Height > viewport.Y+viewport.Height {
		t.Fatalf("row %+v is still outside the viewport %+v after revealing", revealed, viewport)
	}
}
