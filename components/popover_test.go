package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestPopoverPositionsSidesAndAlignment(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	anchor := gfx.Rect{X: 300, Y: 200, Width: 80, Height: 32}
	tests := []struct {
		name string
		side PopoverSide
		want gfx.Rect
	}{
		{name: "bottom", side: PopoverBottom, want: gfx.Rect{X: 303, Y: 238, Width: 200, Height: 100}},
		{name: "top", side: PopoverTop, want: gfx.Rect{X: 303, Y: 94, Width: 200, Height: 100}},
		{name: "right", side: PopoverRight, want: gfx.Rect{X: 386, Y: 203, Width: 200, Height: 100}},
		{name: "left", side: PopoverLeft, want: gfx.Rect{X: 94, Y: 203, Width: 200, Height: 100}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			layout := BeginPopover(&canvas, gfx.LightTheme(), viewport, anchor, PopoverOptions{
				Layer: 5, Side: test.side, Align: PopoverAlignStart, SideOffset: 6, AlignOffset: 3,
				ContentSize: gfx.Size{Width: 200, Height: 100},
			}).Paint("Filters", "Narrow the visible results.")
			if layout.Content != test.want {
				t.Fatalf("content = %+v, want %+v", layout.Content, test.want)
			}
		})
	}
}

func TestPopoverKeepsCustomContentInNonmodalScope(t *testing.T) {
	var canvas gfx.Canvas
	popover := BeginPopover(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 640, Height: 480}, gfx.Rect{X: 200, Y: 100, Width: 80, Height: 32}, PopoverOptions{
		Layer: 6, Scope: gfx.Item(7), DismissID: gfx.Item(8),
	})
	layout := popover.Begin("Settings", "Change display preferences.")
	canvas.Fill(layout.Body, gfx.DarkTheme().Accent)
	canvas.HitWithOptions(gfx.Item(9), layout.Body, "Theme", gfx.HitOptions{Role: gfx.RoleButton})
	popover.End()
	if hit := nonBlockHits(canvas.Hits())[0]; hit.Scope != gfx.Item(7) || hit.Role != gfx.RoleButton {
		t.Fatalf("custom hit = %+v", hit)
	}
	if blocks := blocks(&canvas); len(blocks) != 1 || blocks[0].Bounds != layout.Content || blocks[0].Scope != gfx.Item(7) {
		t.Fatalf("blocks = %+v, want the popover surface %+v", blocks, layout.Content)
	}
	if !canvas.TrappedFocusScope().IsZero() || canvas.EscapeTarget() != gfx.Item(8) {
		t.Fatalf("nonmodal routing = scope %d escape %d", canvas.TrappedFocusScope(), canvas.EscapeTarget())
	}
	if canvas.Rectangles()[len(canvas.Rectangles())-1].Layer != 6 {
		t.Fatal("custom body did not inherit popover layer")
	}
}

func TestPopoverDisabledPaintsNothing(t *testing.T) {
	var canvas gfx.Canvas
	layout := BeginPopover(&canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, gfx.Rect{X: 20, Y: 20, Width: 40, Height: 20}, PopoverOptions{Layer: 2, DismissID: gfx.Item(3)}).Disabled().Paint("Hidden", "Disabled")
	if layout != (PopoverLayout{}) || len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 || !canvas.EscapeTarget().IsZero() {
		t.Fatalf("disabled popover painted: layout=%+v rectangles=%d texts=%d", layout, len(canvas.Rectangles()), len(canvas.Texts()))
	}
}

func TestPopoverCanEmphasizeItsTitleWithoutChangingTheDefault(t *testing.T) {
	for _, weight := range []float32{0, gfx.WeightBold} {
		var canvas gfx.Canvas
		BeginPopover(&canvas, gfx.LightTheme(), gfx.Rect{Width: 640, Height: 480}, gfx.Rect{X: 20, Y: 20, Width: 40, Height: 20}, PopoverOptions{TitleWeight: weight}).Paint("Heading", "Description")
		want := weight
		if want == 0 {
			want = gfx.WeightMedium
		}
		found := false
		for _, text := range canvas.Texts() {
			if text.Value == "Heading" {
				found = true
				if text.Style.Weight != want {
					t.Fatalf("title weight=%v, want %v", text.Style.Weight, want)
				}
			}
		}
		if !found {
			t.Fatal("heading was not painted")
		}
	}
}

func TestPopoverWithoutScopePreservesOuterFocusScope(t *testing.T) {
	var canvas gfx.Canvas
	canvas.PushFocusScope(gfx.Item(51))
	popover := BeginPopover(&canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 300}, gfx.Rect{X: 40, Y: 40, Width: 60, Height: 24}, PopoverOptions{
		Layer: 4, ContentSize: gfx.Size{Width: 180, Height: 100},
	})
	layout := popover.Begin("Actions", "Choose an action.")
	canvas.HitWithLabel(gfx.Item(8), layout.Body, "Popover action")
	popover.End()
	canvas.HitWithLabel(gfx.Item(9), gfx.Rect{Width: 10, Height: 10}, "Outer action")
	canvas.PopFocusScope()

	hits := canvas.Hits()
	if hits[len(hits)-2].Scope != gfx.Item(51) || hits[len(hits)-1].Scope != gfx.Item(51) {
		t.Fatalf("popover changed the outer focus scope: %+v", hits)
	}
}

func TestPopoverStateTransitionsAreAllocationFree(t *testing.T) {
	var state PopoverState
	if state.IsOpen() {
		t.Fatal("zero state is open")
	}
	state = state.Open()
	if !state.IsOpen() {
		t.Fatal("Open did not open")
	}
	if state.Toggle().IsOpen() || state.Close().IsOpen() || state.Dismiss().IsOpen() || state.Escape().IsOpen() || state.OutsidePress().IsOpen() {
		t.Fatal("close transition left popover open")
	}
	transition := func() {
		state = state.Open().Toggle().Open().Dismiss().Open().Escape().Open().OutsidePress().Open().Close()
	}
	if allocations := testing.AllocsPerRun(1000, transition); allocations != 0 {
		t.Fatalf("popover state allocations = %v", allocations)
	}
}

func TestPlaceOverlayFlipsToTheSideWithRoomAndStaysInsideTheViewport(t *testing.T) {
	viewport := gfx.Rect{Width: 400, Height: 300}
	size := gfx.Size{Width: 100, Height: 80}
	placement := OverlayPlacement{Side: OverlayBottom, SideOffset: 4, Margin: 8}

	below, side := PlaceOverlay(gfx.Rect{X: 50, Y: 40, Width: 60, Height: 30}, size, viewport, placement)
	if side != OverlayBottom || below.Y != 74 {
		t.Fatalf("with room below: side %v at y=%v", side, below.Y)
	}
	above, side := PlaceOverlay(gfx.Rect{X: 50, Y: 240, Width: 60, Height: 30}, size, viewport, placement)
	if side != OverlayTop || above.Y+above.Height != 236 {
		t.Fatalf("with no room below: side %v at %+v, want it ending 4px over the anchor", side, above)
	}
	edge, _ := PlaceOverlay(gfx.Rect{X: 380, Y: 40, Width: 10, Height: 30}, size, viewport, placement)
	if edge.X+edge.Width != 392 {
		t.Fatalf("near the right edge: %+v, want it pushed back inside the 8px margin", edge)
	}
	// A pointer position is an anchor without size: a menu opens down and to the right of it.
	pointer, side := PlaceOverlay(gfx.Rect{X: 390, Y: 290}, size, viewport, OverlayPlacement{Align: OverlayAlignStart, Margin: 4})
	if side != OverlayTop || pointer.Y+pointer.Height != 290 || pointer.X+pointer.Width != 396 {
		t.Fatalf("at the corner: side %v %+v", side, pointer)
	}
}

func TestPopoverFitsItsHeaderAndWrapsALongDescription(t *testing.T) {
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 800, Height: 600}
	anchor := gfx.Rect{X: 300, Y: 100, Width: 80, Height: 32}
	short := BeginPopover(&canvas, gfx.LightTheme(), viewport, anchor, PopoverOptions{Layer: 1}).Paint("Title", "Short.")
	long := BeginPopover(&canvas, gfx.LightTheme(), viewport, anchor, PopoverOptions{Layer: 1}).Paint("Title", "This description is long enough that it cannot stay on one line inside a panel two hundred and eighty-eight pixels wide.")
	if long.Content.Height <= short.Content.Height {
		t.Fatalf("a wrapped description made the popover %v tall, not taller than %v", long.Content.Height, short.Content.Height)
	}
	withBody := BeginPopover(&canvas, gfx.LightTheme(), viewport, anchor, PopoverOptions{Layer: 1, BodyHeight: 60}).Paint("Title", "Short.")
	if withBody.Body.Height != 60 {
		t.Fatalf("body = %+v, want the 60 that was asked for", withBody.Body)
	}
}

// A submenu goes beside its row, top aligned with the -3px offset, and flips left
// when the right edge has no room.
func TestSubmenuPlacementFollowsTheRow(t *testing.T) {
	trigger := gfx.Rect{X: 100, Y: 100, Width: 120, Height: 32}
	bounds := SubmenuPlacement(trigger, gfx.Size{Width: 140, Height: 200}, gfx.Rect{Width: 800, Height: 600})
	if bounds.X != trigger.X+trigger.Width || bounds.Y != trigger.Y-3 {
		t.Fatalf("right placement = %+v", bounds)
	}
	flipped := SubmenuPlacement(gfx.Rect{X: 700, Y: 100, Width: 80, Height: 32}, gfx.Size{Width: 140, Height: 200}, gfx.Rect{Width: 800, Height: 600})
	if flipped.X+flipped.Width != 700 || flipped.Y != 97 {
		t.Fatalf("flipped placement = %+v", flipped)
	}
}
