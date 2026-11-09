package components

import (
	"testing"
	"time"

	"github.com/vivaui/viva/gfx"
)

func TestHoverCardFitsItsHeaderAndKeepsTheDefaultOffsets(t *testing.T) {
	var canvas gfx.Canvas
	layout := BeginHoverCard(&canvas, gfx.LightTheme(), gfx.Rect{Width: 900, Height: 600}, gfx.Rect{X: 300, Y: 160, Width: 100, Height: 32}, HoverCardOptions{Layer: 12}).Paint("@viva", "Native desktop UI in Go.")
	if layout.Content != (gfx.Rect{X: 226, Y: 196, Width: 256, Height: 78}) {
		t.Fatalf("content = %+v", layout.Content)
	}
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Layer != 12 {
			t.Fatalf("rectangle layer = %d", rectangle.Layer)
		}
	}
}

func TestHoverCardSidesAndDisabledState(t *testing.T) {
	viewport := gfx.Rect{Width: 800, Height: 600}
	anchor := gfx.Rect{X: 300, Y: 240, Width: 80, Height: 30}
	var canvas gfx.Canvas
	layout := BeginHoverCard(&canvas, gfx.DarkTheme(), viewport, anchor, HoverCardOptions{
		Layer: 2, Side: HoverCardRight, Align: HoverCardAlignEnd, SideOffset: 8, AlignOffset: 2,
		ContentSize: gfx.Size{Width: 180, Height: 120},
	}).Paint("Preview", "Details")
	if layout.Content != (gfx.Rect{X: 388, Y: 152, Width: 180, Height: 120}) {
		t.Fatalf("right/end content = %+v", layout.Content)
	}
	canvas.Reset()
	layout = BeginHoverCard(&canvas, gfx.DarkTheme(), viewport, anchor, HoverCardOptions{Layer: 2}).Disabled().Paint("Preview", "Details")
	if layout != (HoverCardLayout{}) || len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("disabled hover card painted: %+v", layout)
	}
}

func TestHoverCardStateAndDeadlinesAreAllocationFree(t *testing.T) {
	var state HoverCardState
	if state.IsOpen() {
		t.Fatal("zero state is open")
	}
	state = state.Open()
	if !state.IsOpen() || state.Toggle().IsOpen() || state.Close().IsOpen() || state.Dismiss().IsOpen() || state.Escape().IsOpen() || state.OutsidePress().IsOpen() {
		t.Fatal("hover-card state transition failed")
	}
	now := time.Unix(200, 500)
	delays := HoverCardDelays{Open: 500 * time.Millisecond, Close: 300 * time.Millisecond}
	if got := delays.OpenDeadline(now); !got.Equal(now.Add(500 * time.Millisecond)) {
		t.Fatalf("open deadline = %v", got)
	}
	if got := delays.CloseDeadline(now); !got.Equal(now.Add(300 * time.Millisecond)) {
		t.Fatalf("close deadline = %v", got)
	}
	if got := (HoverCardDelays{Close: -time.Second}).CloseDeadline(now); !got.Equal(now) {
		t.Fatalf("negative-delay deadline = %v", got)
	}
	transition := func() {
		state = state.Open().Toggle().Open().Dismiss().Open().Escape().Open().OutsidePress().Open().Close()
		_ = delays.OpenDeadline(now)
		_ = delays.CloseDeadline(now)
	}
	if allocations := testing.AllocsPerRun(1000, transition); allocations != 0 {
		t.Fatalf("hover-card state/deadline allocations = %v", allocations)
	}
}

// Hover intent opens only after the pointer has rested, starts over when it
// moves, and reports how long to wait so the caller can schedule one frame.
func TestHoverIntentWaitsForRestAndStartsOver(t *testing.T) {
	now := time.Unix(0, 0)
	var intent HoverIntent
	if id, wait := intent.Rest(gfx.Item(1), now, 100*time.Millisecond); !id.IsZero() || wait != 100*time.Millisecond {
		t.Fatalf("first hover = (%v, %v)", id, wait)
	}
	if id, wait := intent.Rest(gfx.Item(1), now.Add(40*time.Millisecond), 100*time.Millisecond); !id.IsZero() || wait != 60*time.Millisecond {
		t.Fatalf("resting = (%v, %v)", id, wait)
	}
	if id, wait := intent.Rest(gfx.Item(2), now.Add(80*time.Millisecond), 100*time.Millisecond); !id.IsZero() || wait != 100*time.Millisecond {
		t.Fatalf("moving to another control = (%v, %v), want a fresh wait", id, wait)
	}
	if id, wait := intent.Rest(gfx.Item(2), now.Add(180*time.Millisecond), 100*time.Millisecond); id != gfx.Item(2) || wait != 0 {
		t.Fatalf("rested = (%v, %v), want item 2 now", id, wait)
	}
	if id, _ := intent.Rest(gfx.ID{}, now.Add(200*time.Millisecond), 100*time.Millisecond); !id.IsZero() || intent != (HoverIntent{}) {
		t.Fatal("leaving did not clear the intent")
	}
	if id, _ := intent.Rest(gfx.Item(3), now, 0); id != gfx.Item(3) {
		t.Fatal("a zero delay did not open at once")
	}
}

// A hover card is enterable and dismissible when it has an ID: its surface is a
// group hit, outside presses and Escape go to DismissID, and the card does not
// steal the page's presses on its own padding.
func TestHoverCardSurfaceIsEnterableAndDismissible(t *testing.T) {
	var canvas gfx.Canvas
	layout := BeginHoverCard(&canvas, gfx.LightTheme(), gfx.Rect{Width: 800, Height: 600}, gfx.Rect{X: 100, Y: 100, Width: 80, Height: 32}, HoverCardOptions{
		Layer: 5, ID: gfx.Item(8), DismissID: gfx.Item(9),
	}).Paint("Ada", "Designs the terminal.")
	if canvas.EscapeTarget() != gfx.Item(9) {
		t.Fatalf("Escape = %v", canvas.EscapeTarget())
	}
	hits := nonBlockHits(canvas.Hits())
	if len(hits) != 2 || hits[0].ID != gfx.Item(9) || !hits[0].Backdrop || hits[0].PassPrimary != true || hits[1].ID != gfx.Item(8) || hits[1].Role != gfx.RoleGroup || hits[1].Bounds != layout.Content {
		t.Fatalf("hits = %+v", hits)
	}
}
