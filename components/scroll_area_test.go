package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestScrollAreaClampsOffsetsAndComputesBothThumbs(t *testing.T) {
	area := BeginScrollArea(nil, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}, gfx.Size{Width: 500, Height: 400}).Offset(999, -20)
	geometry := area.Geometry()
	if geometry.OffsetX != 300 || geometry.OffsetY != 0 || geometry.MaxOffsetX != 300 || geometry.MaxOffsetY != 300 {
		t.Fatalf("scroll offsets = %+v", geometry)
	}
	if geometry.VerticalTrack != (gfx.Rect{X: 200, Y: 20, Width: 10, Height: 90}) {
		t.Fatalf("vertical track = %+v", geometry.VerticalTrack)
	}
	if geometry.HorizontalTrack != (gfx.Rect{X: 10, Y: 110, Width: 190, Height: 10}) {
		t.Fatalf("horizontal track = %+v", geometry.HorizontalTrack)
	}
	if geometry.Content.X != -290 || geometry.Content.Y != 20 {
		t.Fatalf("translated content = %+v", geometry.Content)
	}
	if geometry.HorizontalThumb.X+geometry.HorizontalThumb.Width != geometry.HorizontalTrack.X+geometry.HorizontalTrack.Width-1 {
		t.Fatalf("horizontal thumb does not end at inset: %+v", geometry.HorizontalThumb)
	}
}

func TestScrollAreaClipsContentAndPaintsTypedSlidersOutsideClip(t *testing.T) {
	var canvas gfx.Canvas
	area := BeginScrollArea(&canvas, gfx.LightTheme(), gfx.Rect{X: 8, Y: 9, Width: 120, Height: 80}, gfx.Size{Width: 240, Height: 160}).
		Offset(30, 40).
		VerticalScrollbar(gfx.Item(11), "Vertical scroll", "50%").
		HorizontalScrollbar(gfx.Item(12), "Horizontal scroll", "25%")
	geometry := area.BeginContent()
	canvas.Fill(geometry.Content, gfx.LightTheme().Card)
	area.EndContent()
	if len(canvas.Clips()) != 0 {
		t.Fatalf("clip stack was not restored: %+v", canvas.Clips())
	}
	if !canvas.Rectangles()[0].HasClip || canvas.Rectangles()[0].ClipOf != geometry.Viewport {
		t.Fatalf("content clip = %+v", canvas.Rectangles()[0])
	}
	if len(canvas.Hits()) != 2 || canvas.Hits()[0].Role != gfx.RoleSeparator || canvas.Hits()[1].Role != gfx.RoleSeparator {
		t.Fatalf("scrollbar semantics = %+v", canvas.Hits())
	}
	if canvas.Hits()[0].Value != "50%" || canvas.Hits()[1].Value != "25%" {
		t.Fatalf("scrollbar values = %+v", canvas.Hits())
	}
}

func TestScrollAreaDisabledGeometryIsStableAndAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		area := BeginScrollArea(&canvas, theme, gfx.Rect{Width: 160, Height: 90}, gfx.Size{Width: 160, Height: 360}).
			Offset(0, 130).VerticalScrollbar(gfx.Item(13), "Messages", "48%").Disabled()
		area.BeginContent()
		area.EndContent()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	hitBounds := canvas.Hits()[0].Bounds
	paint()
	if !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || canvas.Hits()[0].Bounds != hitBounds {
		t.Fatalf("disabled scrollbar = %+v", canvas.Hits()[0])
	}
	if !slices.Equal(rectangles, canvas.Rectangles()) {
		t.Fatal("identical scroll areas produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("scroll area allocations = %v", allocations)
	}
}

func TestScrollAreaThumbOffsetInvertsThumbPlacement(t *testing.T) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 200}
	content := gfx.Size{Width: 300, Height: 1000}
	for _, offset := range []float32{0, 1, 137.5, 400, 799, 800} {
		geometry := BeginScrollArea(nil, gfx.LightTheme(), bounds, content).Offset(0, offset).Geometry()
		got := geometry.VerticalOffsetForThumb(geometry.VerticalThumb.Y)
		if got < offset-0.01 || got > offset+0.01 {
			t.Fatalf("offset %v: thumb at %v maps back to %v", offset, geometry.VerticalThumb.Y, got)
		}
	}

	geometry := BeginScrollArea(nil, gfx.LightTheme(), bounds, content).Geometry()
	if got := geometry.VerticalOffsetForThumb(-500); got != 0 {
		t.Fatalf("a drag above the track must clamp to the start, got %v", got)
	}
	if got := geometry.VerticalOffsetForThumb(5000); got != geometry.MaxOffsetY {
		t.Fatalf("a drag below the track must clamp to the end, got %v want %v", got, geometry.MaxOffsetY)
	}

	if got := geometry.HorizontalOffsetForThumb(geometry.HorizontalThumb.X); got != 0 {
		t.Fatalf("a horizontal drag at the start = %v", got)
	}
	if got := geometry.HorizontalOffsetForThumb(geometry.HorizontalTrack.X + geometry.HorizontalTrack.Width); got != geometry.MaxOffsetX {
		t.Fatalf("a horizontal drag at the end = %v, want %v", got, geometry.MaxOffsetX)
	}

	short := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 300, Height: 100}).Geometry()
	if got := short.VerticalOffsetForThumb(50); got != 0 {
		t.Fatalf("content that fits has nothing to scroll, got %v", got)
	}
}

// A named viewport is a focusable group, and the key helper moves the
// caller-owned offsets by a step, a page, or to an end.
func TestScrollAreaViewportAndKeys(t *testing.T) {
	var canvas gfx.Canvas
	area := BeginScrollArea(&canvas, gfx.LightTheme(), gfx.Rect{Width: 200, Height: 100}, gfx.Size{Width: 400, Height: 300}).Viewport(gfx.Item(7), "Scrollable content")
	area.BeginContent()
	area.EndContent()
	found := false
	for _, hit := range canvas.Hits() {
		if hit.ID == gfx.Item(7) && hit.Role == gfx.RoleGroup && hit.Focusable {
			found = true
		}
	}
	if !found {
		t.Fatalf("no focusable viewport: %+v", canvas.Hits())
	}
	for _, test := range []struct {
		name string
		x, y float32
		key  gfx.Key
		want float32
	}{
		{"down", 0, 0, gfx.KeyDown, 32},
		{"page down", 0, 0, gfx.KeyPageDown, 90},
		{"end", 0, 0, gfx.KeyEnd, 200},
		{"home", 0, 200, gfx.KeyHome, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			x, y, handled := area.ScrollForKey(test.x, test.y, test.key, 32)
			if !handled || x != 0 || y != test.want {
				t.Fatalf("= (%v, %v, %v), want y %v", x, y, handled, test.want)
			}
		})
	}
	if x, _, handled := area.ScrollForKey(0, 0, gfx.KeyRight, 32); !handled || x != 32 {
		t.Fatalf("Right = (%v, %v)", x, handled)
	}
	if _, _, handled := area.ScrollForKey(0, 0, gfx.KeyEnter, 32); handled {
		t.Fatal("Enter is not a scroll key")
	}
}

// Reserved bars take their strips out of the viewport, and only the strips the content needs:
// content as wide as the strip beside the vertical bar gets no horizontal bar.
func TestScrollAreaReserveScrollbarsKeepsContentClearOfTheBars(t *testing.T) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}
	tall := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 190, Height: 400}).ReserveScrollbars().Geometry()
	if tall.Viewport != (gfx.Rect{X: 10, Y: 20, Width: 190, Height: 100}) || tall.MaxOffsetX != 0 || tall.MaxOffsetY != 300 {
		t.Fatalf("tall content: viewport %+v max %v,%v", tall.Viewport, tall.MaxOffsetX, tall.MaxOffsetY)
	}
	if tall.HorizontalTrack != (gfx.Rect{}) || tall.VerticalTrack != (gfx.Rect{X: 200, Y: 20, Width: 10, Height: 100}) {
		t.Fatalf("tall content: tracks %+v %+v", tall.HorizontalTrack, tall.VerticalTrack)
	}
	both := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 500, Height: 400}).ReserveScrollbars().Geometry()
	if both.Viewport != (gfx.Rect{X: 10, Y: 20, Width: 190, Height: 90}) || both.MaxOffsetX != 310 || both.MaxOffsetY != 310 {
		t.Fatalf("wide and tall content: viewport %+v max %v,%v", both.Viewport, both.MaxOffsetX, both.MaxOffsetY)
	}
	if both.Corner == (gfx.Rect{}) || both.VerticalTrack.Height != both.Viewport.Height || both.HorizontalTrack.Width != both.Viewport.Width {
		t.Fatalf("wide and tall content: corner %+v tracks %+v %+v", both.Corner, both.VerticalTrack, both.HorizontalTrack)
	}
	// Adding the vertical bar narrows the viewport enough to need the horizontal one too.
	edge := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 195, Height: 400}).ReserveScrollbars().Geometry()
	if edge.MaxOffsetX == 0 || edge.HorizontalTrack == (gfx.Rect{}) {
		t.Fatalf("content wider than the viewport beside the bar has no horizontal bar: %+v", edge)
	}
	// Without it the bars overlay the content, as before.
	overlay := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 200, Height: 400}).Geometry()
	if overlay.Viewport != bounds || overlay.MaxOffsetX != 0 {
		t.Fatalf("default viewport %+v max %v", overlay.Viewport, overlay.MaxOffsetX)
	}
}

// A hover scroll area is the narrow overlay variant: the viewport keeps the full
// width, the bar geometry does not move between hidden and revealed, and while
// hidden the bar leaves nothing for the pointer to hit, so presses reach the
// content underneath.
func TestHoverScrollAreaRevealsANarrowOverlayBarOnlyWhileHovered(t *testing.T) {
	bounds := gfx.Rect{X: 8, Y: 9, Width: 200, Height: 120}
	content := gfx.Size{Width: 200, Height: 1000}
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	paint := func(hovered bool) ScrollAreaGeometry {
		canvas.Reset()
		area := BeginScrollArea(&canvas, theme, bounds, content).
			Offset(0, 260).
			VerticalScrollbar(gfx.Item(31), "Session list", "").
			HoverScrollbars(hovered)
		geometry := area.BeginContent()
		area.EndContent()
		return geometry
	}

	hidden := paint(false)
	if hidden.Viewport != bounds {
		t.Fatalf("hover bars must not reserve width: viewport %+v", hidden.Viewport)
	}
	if hidden.VerticalTrack.Width != 6 || hidden.VerticalThumb.Width != 4 {
		t.Fatalf("hover track/thumb width = %v/%v", hidden.VerticalTrack.Width, hidden.VerticalThumb.Width)
	}
	if len(canvas.Hits()) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("a hidden bar still painted: hits %+v rectangles %+v", canvas.Hits(), canvas.Rectangles())
	}

	revealed := paint(true)
	if revealed != hidden {
		t.Fatalf("revealing moved the geometry: %+v != %+v", revealed, hidden)
	}
	if len(canvas.Rectangles()) != 1 || canvas.Rectangles()[0].Bounds != revealed.VerticalThumb {
		t.Fatalf("revealed bar = %+v", canvas.Rectangles())
	}
	if len(canvas.Hits()) != 1 || canvas.Hits()[0].ID != gfx.Item(31) || canvas.Hits()[0].Role != gfx.RoleSeparator || canvas.Hits()[0].Bounds != revealed.VerticalTrack {
		t.Fatalf("revealed bar hit = %+v", canvas.Hits())
	}

	// Hiding again removes both the pill and its hit, so a click falls through.
	paint(false)
	if len(canvas.Hits()) != 0 || len(canvas.Rectangles()) != 0 {
		t.Fatalf("hiding left the bar behind: %+v %+v", canvas.Hits(), canvas.Rectangles())
	}
}

// ReserveScrollbars still works with hover bars: the viewport gives up the
// narrow strip instead of letting the thumb float over the content.
func TestHoverScrollAreaCanReserveItsNarrowStrip(t *testing.T) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 100}
	geometry := BeginScrollArea(nil, gfx.LightTheme(), bounds, gfx.Size{Width: 194, Height: 400}).
		ReserveScrollbars().
		HoverScrollbars(true).
		Geometry()
	if geometry.Viewport != (gfx.Rect{X: 10, Y: 20, Width: 194, Height: 100}) {
		t.Fatalf("reserved hover viewport = %+v", geometry.Viewport)
	}
	if geometry.VerticalTrack != (gfx.Rect{X: 204, Y: 20, Width: 6, Height: 100}) {
		t.Fatalf("reserved hover track = %+v", geometry.VerticalTrack)
	}
}

// Painting a revealed hover bar performs no allocation, like the rest of the
// scroll area.
func TestHoverScrollAreaPaintIsAllocationFree(t *testing.T) {
	theme := gfx.DarkTheme()
	bounds := gfx.Rect{Width: 160, Height: 90}
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		area := BeginScrollArea(&canvas, theme, bounds, gfx.Size{Width: 160, Height: 360}).
			Offset(0, 130).
			VerticalScrollbar(gfx.Item(13), "Sessions", "").
			HoverScrollbars(true)
		area.BeginContent()
		area.EndContent()
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("hover scroll area allocations = %v", allocations)
	}
}

func BenchmarkScrollAreaHoverBarsPaint(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	bounds := gfx.Rect{Width: 160, Height: 90}
	content := gfx.Size{Width: 160, Height: 3600}
	paint := func() {
		canvas.Reset()
		area := BeginScrollArea(&canvas, theme, bounds, content).
			Offset(0, 1300).
			VerticalScrollbar(gfx.Item(13), "Sessions", "").
			HoverScrollbars(true)
		area.BeginContent()
		area.EndContent()
	}
	paint()
	b.ReportAllocs()
	for b.Loop() {
		paint()
	}
}
