package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestMessageScrollerClampsAndMapsItems(t *testing.T) {
	scroller := BeginMessageScroller(nil, gfx.LightTheme(), gfx.Rect{X: 20, Y: 30, Width: 240, Height: 180}, 620, 900)
	geometry := scroller.Geometry()
	if geometry.Offset != 440 || geometry.MaxOffset != 440 || geometry.CanEnd || !geometry.CanStart {
		t.Fatalf("message geometry = %+v", geometry)
	}
	visible := scroller.ItemGeometry(7, 500, 100)
	hidden := scroller.ItemGeometry(2, 40, 80)
	if visible.Index != 7 || visible.Bounds.Y != 90 || !visible.Visible {
		t.Fatalf("visible item = %+v", visible)
	}
	if hidden.Visible {
		t.Fatalf("hidden item reported visible: %+v", hidden)
	}
	if geometry.Thumb.Y+geometry.Thumb.Height != geometry.Viewport.Y+geometry.Viewport.Height {
		t.Fatalf("thumb not clamped to end: %+v", geometry.Thumb)
	}
}

func TestMessageScrollerClipsAndPaintsSliderAndActiveButton(t *testing.T) {
	var canvas gfx.Canvas
	scroller := BeginMessageScroller(&canvas, gfx.LightTheme(), gfx.Rect{Width: 220, Height: 160}, 480, 120).
		Scrollbar(gfx.Item(21), "Message scroll position", "38%").
		Buttons(gfx.Item(22), "Scroll to start", gfx.Item(23), "Scroll to end")
	geometry := scroller.BeginContent()
	item := scroller.ItemGeometry(3, 140, 48)
	canvas.Fill(item.Bounds, gfx.LightTheme().Muted)
	scroller.EndContent()
	if !canvas.Rectangles()[0].HasClip || canvas.Rectangles()[0].ClipOf != geometry.Viewport {
		t.Fatalf("message item clip = %+v", canvas.Rectangles()[0])
	}
	if len(canvas.Hits()) != 3 {
		t.Fatalf("message hits = %+v", canvas.Hits())
	}
	if canvas.Hits()[0].Role != gfx.RoleSlider || canvas.Hits()[1].Role != gfx.RoleButton || canvas.Hits()[2].Role != gfx.RoleButton {
		t.Fatalf("message semantics = %+v", canvas.Hits())
	}
}

func TestMessageScrollerDisabledBoundsAndDeterministicStream(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() MessageScrollerGeometry {
		canvas.Reset()
		scroller := BeginMessageScroller(&canvas, theme, gfx.Rect{X: 5, Y: 6, Width: 200, Height: 120}, 360, 90).
			Scrollbar(gfx.Item(24), "Message scroll position", "38%").
			Buttons(gfx.Item(25), "Start", gfx.Item(26), "End").Disabled()
		scroller.BeginContent()
		return scroller.EndContent()
	}
	geometry := paint()
	rectangles := slices.Clone(canvas.Rectangles())
	if len(canvas.Hits()) != 1 || !canvas.Hits()[0].Disabled || geometry.StartButton.Width != 32 || geometry.EndButton.Width != 32 {
		t.Fatalf("disabled message scroller = %+v hits=%+v", geometry, canvas.Hits())
	}
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) {
		t.Fatal("identical message scrollers produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, func() { paint() }); allocations != 0 {
		t.Fatalf("message scroller allocations = %v", allocations)
	}
}

func TestMessageScrollerOffersJumpsOnlyBeyondTheEdgeThreshold(t *testing.T) {
	bounds := gfx.Rect{Width: 240, Height: 180}
	nearEnd := BeginMessageScroller(nil, gfx.LightTheme(), bounds, 620, 440-MessageScrollerEdgeThreshold+1).Geometry()
	if nearEnd.CanEnd {
		t.Fatal("the end button flickers on within the edge threshold of the end")
	}
	farEnd := BeginMessageScroller(nil, gfx.LightTheme(), bounds, 620, 440-MessageScrollerEdgeThreshold-1).Geometry()
	if !farEnd.CanEnd {
		t.Fatal("no end button beyond the edge threshold")
	}
	if BeginMessageScroller(nil, gfx.LightTheme(), bounds, 620, MessageScrollerEdgeThreshold-1).Geometry().CanStart {
		t.Fatal("the start button shows within the edge threshold of the start")
	}
}

func TestMessageScrollStateFollowsTheLiveEdgeUntilTheReaderLeavesIt(t *testing.T) {
	state := MessageScrollState{AutoScroll: true}
	state.ScrollToEnd()
	if got := state.Resolve(100, 300); got != 200 {
		t.Fatalf("opened at %v, want the end 200", got)
	}
	if got := state.Resolve(100, 340); got != 240 {
		t.Fatalf("streamed content moved the view to %v, want to follow it to 240", got)
	}
	// A slow trackpad drag is a few pixels per event; the first one up must release
	// the edge instead of being pinned back by the threshold.
	state.ScrollBy(-3)
	if state.Following() {
		t.Fatal("scrolling away from the end kept following")
	}
	if got := state.Resolve(100, 400); got != 237 {
		t.Fatalf("content growth moved a reader who left the edge: %v", got)
	}
	state.ScrollBy(1000)
	if !state.Following() || !state.AtEnd() {
		t.Fatalf("reaching the end did not take hold of the edge again: following=%v", state.Following())
	}
	if got := state.Resolve(100, 450); got != 350 {
		t.Fatalf("after returning, the view did not follow: %v", got)
	}
}

func TestMessageScrollStateWithoutAutoScrollOnlyMovesWhenTold(t *testing.T) {
	var state MessageScrollState
	state.ScrollToEnd()
	if got := state.Resolve(100, 300); got != 200 {
		t.Fatalf("ScrollToEnd resolved to %v", got)
	}
	if got := state.Resolve(100, 360); got != 200 {
		t.Fatalf("content growth moved the view to %v without AutoScroll", got)
	}
}

func TestMessageScrollStateKeepsTheReadersPlaceWhenHistoryIsPrepended(t *testing.T) {
	var state MessageScrollState
	state.Resolve(100, 300)
	state.ScrollTo(50)
	state.Prepended(120)
	if got := state.Resolve(100, 420); got != 170 {
		t.Fatalf("offset after prepending 120 above = %v, want 170", got)
	}
	following := MessageScrollState{AutoScroll: true}
	following.ScrollToEnd()
	following.Resolve(100, 300)
	following.Prepended(120)
	if got := following.Resolve(100, 420); got != 320 {
		t.Fatalf("a following transcript ended at %v, want the new end 320", got)
	}
}

func TestMessageScrollStateAnchorsATurnThenHandsOverToTheLiveEdge(t *testing.T) {
	state := MessageScrollState{AutoScroll: true, PreviousPeek: MessageScrollPreviousPeek}
	state.Resolve(100, 250)
	// The user's message was appended at 250 and is 20 tall: it goes near the top,
	// and the transcript grows room below it for the reply.
	state.AnchorTurn(250)
	if got := state.Resolve(100, 270); got != 186 {
		t.Fatalf("anchored offset = %v, want 186 (the turn 64 below the top)", got)
	}
	if state.Extent() < 286 {
		t.Fatalf("no room below the anchored turn: extent %v", state.Extent())
	}
	if got := state.Resolve(100, 280); got != 186 {
		t.Fatalf("a streaming reply moved the anchored turn to %v", got)
	}
	if got := state.Resolve(100, 300); got != 200 || !state.Following() {
		t.Fatalf("once the reply fills the viewport the edge should take over: offset %v following %v", got, state.Following())
	}

	held := MessageScrollState{PreviousPeek: MessageScrollPreviousPeek}
	held.AnchorTurn(250)
	if got := held.Resolve(100, 500); got != 186 {
		t.Fatalf("without AutoScroll the anchored turn moved to %v", got)
	}
}

func TestMessageScrollStateAllocatesNothing(t *testing.T) {
	state := MessageScrollState{AutoScroll: true}
	if allocations := testing.AllocsPerRun(100, func() {
		state.ScrollBy(-4)
		state.ScrollToEnd()
		state.Prepended(10)
		state.Resolve(100, 400)
	}); allocations != 0 {
		t.Fatalf("allocations = %v", allocations)
	}
}

func TestMessageScrollerMapsThumbBackToAnOffset(t *testing.T) {
	geometry := BeginMessageScroller(nil, gfx.LightTheme(), gfx.Rect{X: 20, Y: 30, Width: 240, Height: 180}, 620, 0).Geometry()
	if got := geometry.OffsetForThumb(geometry.Thumb.Y); got != 0 {
		t.Fatalf("thumb at the top maps to %v", got)
	}
	end := geometry.Viewport.Y + geometry.Viewport.Height - geometry.Thumb.Height
	if got := geometry.OffsetForThumb(end); got != geometry.MaxOffset {
		t.Fatalf("thumb at the bottom maps to %v, want %v", got, geometry.MaxOffset)
	}
	mid := geometry.OffsetForThumb(geometry.Track.Y + (geometry.Track.Height-geometry.Thumb.Height)/2)
	if mid <= 0 || mid >= geometry.MaxOffset {
		t.Fatalf("mid track maps to %v", mid)
	}
	if got := geometry.OffsetForThumb(geometry.Thumb.Y + geometry.Track.Height); got != geometry.MaxOffset {
		t.Fatalf("past the end maps to %v, want %v", got, geometry.MaxOffset)
	}
	// A full track has no travel: the offset is kept instead of jumping home.
	full := BeginMessageScroller(nil, gfx.LightTheme(), gfx.Rect{X: 0, Y: 0, Width: 240, Height: 180}, 180, 0).Geometry()
	if got := full.OffsetForThumb(120); got != full.Offset {
		t.Fatalf("full track maps to %v, want %v", got, full.Offset)
	}
}
