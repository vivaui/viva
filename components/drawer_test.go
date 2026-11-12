package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestDrawerSideGeometryAndHandle(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	tests := []struct {
		name string
		side DrawerSide
		want gfx.Rect
	}{
		{name: "bottom", side: DrawerBottom, want: gfx.Rect{X: 0, Y: 320, Width: 800, Height: 280}},
		{name: "top", side: DrawerTop, want: gfx.Rect{X: 0, Y: 0, Width: 800, Height: 280}},
		{name: "right", side: DrawerRight, want: gfx.Rect{X: 520, Y: 0, Width: 280, Height: 600}},
		{name: "left", side: DrawerLeft, want: gfx.Rect{X: 0, Y: 0, Width: 280, Height: 600}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			layout := BeginDrawer(&canvas, gfx.LightTheme(), viewport, DrawerOptions{
				Layer: 20, Scope: gfx.Item(21), DismissID: gfx.Item(22), CloseID: gfx.Item(23), Side: test.side, ContentExtent: 280,
			}).WithHandle().Paint("Details", "Review the selected item.")
			if layout.Content != test.want {
				t.Fatalf("content = %+v, want %+v", layout.Content, test.want)
			}
			if layout.Handle.Width == 0 || layout.Handle.Height == 0 {
				t.Fatal("handle slot is empty")
			}
		})
	}
}

func TestDrawerUsesModalProtocolAndDisabledSemantics(t *testing.T) {
	var canvas gfx.Canvas
	BeginDrawer(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 640, Height: 480}, DrawerOptions{
		Layer: 24, Scope: gfx.Item(25), DismissID: gfx.Item(26), CloseID: gfx.Item(27),
	}).Disabled().Paint("Unavailable", "The drawer is temporarily locked.")
	if canvas.TrappedFocusScope() != gfx.Item(25) || !canvas.EscapeTarget().IsZero() {
		t.Fatalf("modal routing = %d / %d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	if len(targets(&canvas)) != 2 || len(blocks(&canvas)) != 1 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	for _, hit := range targets(&canvas) {
		if hit.Scope != gfx.Item(25) || hit.Role != gfx.RoleButton || !hit.Disabled {
			t.Fatalf("drawer hit = %+v", hit)
		}
	}
	for _, text := range canvas.Texts() {
		if text.Layer != 24 {
			t.Fatalf("text layer = %d", text.Layer)
		}
	}
}

// A drawer swipe dismisses on a half-open drag or a throw and otherwise snaps to
// the nearest caller fraction.
func TestDrawerSwipeDismissesOrSnaps(t *testing.T) {
	var swipe DrawerSwipe
	swipe.Begin(0)
	if got := swipe.Move(120, 300); got != 120 {
		t.Fatalf("Move = %v", got)
	}
	if offset, dismissed := swipe.End(300, 200, []float32{0.25, 0.6}); dismissed || offset != 75 {
		t.Fatalf("snap = (%v, %v), want 75 open", offset, dismissed)
	}
	swipe.Begin(0)
	swipe.Move(200, 300)
	if offset, dismissed := swipe.End(300, 0, nil); !dismissed || offset != 300 {
		t.Fatalf("half drag = (%v, %v), want dismissed", offset, dismissed)
	}
	swipe.Begin(0)
	swipe.Move(20, 300)
	if _, dismissed := swipe.End(300, 900, nil); !dismissed {
		t.Fatal("a fast throw did not dismiss")
	}
}
