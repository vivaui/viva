package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestDirectionResolvesLogicalGeometry(t *testing.T) {
	container := gfx.Rect{X: 10, Y: 20, Width: 100, Height: 40}
	logical := LogicalRect{InlineStart: 8, BlockStart: 4, InlineSize: 30, BlockSize: 12}
	if got := LTR.Rect(container, logical); got != (gfx.Rect{X: 18, Y: 24, Width: 30, Height: 12}) {
		t.Fatalf("ltr rect = %+v", got)
	}
	if got := RTL.Rect(container, logical); got != (gfx.Rect{X: 72, Y: 24, Width: 30, Height: 12}) {
		t.Fatalf("rtl rect = %+v", got)
	}
	if got := Direction(0).Rect(container, logical); got != LTR.Rect(container, logical) {
		t.Fatalf("default direction = %+v", got)
	}
}

func TestDirectionResolvesInsetsAndEdges(t *testing.T) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 100, Height: 40}
	insets := LogicalInsets{Start: 8, Top: 3, End: 12, Bottom: 5}
	if got := LTR.Inset(bounds, insets); got != (gfx.Rect{X: 18, Y: 23, Width: 80, Height: 32}) {
		t.Fatalf("ltr inset = %+v", got)
	}
	if got := RTL.Inset(bounds, insets); got != (gfx.Rect{X: 22, Y: 23, Width: 80, Height: 32}) {
		t.Fatalf("rtl inset = %+v", got)
	}
	if got := RTL.Start(bounds, 16); got != (gfx.Rect{X: 94, Y: 20, Width: 16, Height: 40}) {
		t.Fatalf("rtl start = %+v", got)
	}
	if got := RTL.End(bounds, 16); got != (gfx.Rect{X: 10, Y: 20, Width: 16, Height: 40}) {
		t.Fatalf("rtl end = %+v", got)
	}
}

func TestDirectionMapsMovementAndVisualOrder(t *testing.T) {
	if LTR.InlineDelta(7) != 7 || RTL.InlineDelta(7) != -7 {
		t.Fatalf("inline deltas = %v %v", LTR.InlineDelta(7), RTL.InlineDelta(7))
	}
	if LTR.VisualIndex(1, 4) != 1 || RTL.VisualIndex(1, 4) != 2 {
		t.Fatalf("visual indices = %d %d", LTR.VisualIndex(1, 4), RTL.VisualIndex(1, 4))
	}
	if RTL.VisualIndex(4, 4) != -1 || RTL.VisualIndex(-1, 4) != -1 {
		t.Fatal("out-of-range visual index was accepted")
	}
}
