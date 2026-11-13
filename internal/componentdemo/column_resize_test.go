package componentdemo

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// columnResizeDividerHit is the hit the last paint registered for divider index.
func columnResizeDividerHit(t *testing.T, canvas *gfx.Canvas, index int) gfx.Hit {
	t.Helper()
	id := columnResizeDemoDivider.At(index)
	for _, hit := range canvas.Hits() {
		if hit.ID == id {
			return hit
		}
	}
	t.Fatalf("divider %d has no hit", index)
	return gfx.Hit{}
}

// columnResizeLineX is where a divider's one-pixel line sits: its hit strip is
// centered on it.
func columnResizeLineX(hit gfx.Hit) float32 {
	return hit.Bounds.X + (hit.Bounds.Width-1)/2
}

func TestColumnResizeDemoDragFollowsPointer(t *testing.T) {
	state := newTestState(t, "column-resize", "file-list")
	size := gfx.Size{Width: 760, Height: 380}
	var canvas gfx.Canvas
	paintAt(&canvas, state, size)
	stage := state.bag().placed
	divider := columnResizeDividerHit(t, &canvas, 0)
	if divider.Focusable || divider.SkipTab {
		t.Fatalf("divider is a tab stop: focusable=%t skipTab=%t", divider.Focusable, divider.SkipTab)
	}
	widths := state.columnResizeWidths()
	if want := stage.X + widths[0] + columnResizeGap/2; columnResizeLineX(divider) != want {
		t.Fatalf("divider at %g, want %g", columnResizeLineX(divider), want)
	}
	// The pointer grabs divider 0 and moves left, so the divider must follow it
	// rather than jump by the difference between the stored and painted widths.
	target := stage.X + 300
	state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary,
		X: columnResizeLineX(divider), Y: divider.Bounds.Y, Target: divider.ID,
	})
	state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerMove, X: target, Y: divider.Bounds.Y, Target: divider.ID, Captured: true,
	})
	paintAt(&canvas, state, size)
	moved := columnResizeDividerHit(t, &canvas, 0)
	if delta := columnResizeLineX(moved) - target; delta < -1 || delta > 1 {
		t.Fatalf("divider moved to %g, want the pointer at %g", columnResizeLineX(moved), target)
	}
	widths = state.columnResizeWidths()
	if widths[0] < columnResizeMinimums[0] || widths[1] < columnResizeMinimums[1] || widths[2] < columnResizeMinimums[2] {
		t.Fatalf("widths %v below the minimums %v", widths, columnResizeMinimums)
	}
	if filled := widths[0] + widths[1] + widths[2] + 2*columnResizeGap; filled < stage.Width-0.5 || filled > stage.Width+0.5 {
		t.Fatalf("columns fill %g of the %g stage", filled, stage.Width)
	}
	if want := stage.Width - 2*columnResizeGap - widths[1] - widths[2]; widths[0] < want-0.5 || widths[0] > want+0.5 {
		t.Fatalf("name column = %g, want the stage's leftover %g", widths[0], want)
	}
}

func TestColumnResizeDemoFitUsesTheContentWidth(t *testing.T) {
	state := newTestState(t, "column-resize", "file-list")
	size := gfx.Size{Width: 760, Height: 380}
	var canvas gfx.Canvas
	paintAt(&canvas, state, size)
	divider := columnResizeDividerHit(t, &canvas, 0)
	state.Pointer(gfx.PointerEvent{
		Kind: gfx.PointerDown, Button: gfx.PointerButtonPrimary, ClickCount: 2,
		X: columnResizeLineX(divider), Y: divider.Bounds.Y, Target: divider.ID,
	})
	paintAt(&canvas, state, size)
	if widths := state.columnResizeWidths(); widths[0] < columnResizeFits[0]-0.5 || widths[0] > columnResizeFits[0]+0.5 {
		t.Fatalf("fit name column = %g, want %g", widths[0], columnResizeFits[0])
	}
}

func TestColumnResizeDemoStaysInsideItsStage(t *testing.T) {
	for _, size := range []gfx.Size{{Width: 320, Height: 340}, {Width: 760, Height: 380}} {
		state := newTestState(t, "column-resize", "file-list")
		var canvas gfx.Canvas
		paintAt(&canvas, state, size)
		stage := state.bag().placed
		widths := state.columnResizeWidths()
		if widths[0] < columnResizeMinimums[0] || widths[1] < columnResizeMinimums[1] || widths[2] < columnResizeMinimums[2] {
			t.Fatalf("%v: widths %v below the minimums %v", size, widths, columnResizeMinimums)
		}
		if filled := widths[0] + widths[1] + widths[2] + 2*columnResizeGap; filled > stage.Width+0.5 {
			t.Fatalf("%v: columns fill %g of the %g stage", size, filled, stage.Width)
		}
	}
}
