package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestScrollAreaClampsOffsetsAndComputesBothThumbs(t *testing.T) {
	area := BeginScrollArea(nil, ui.LightTheme(), ui.Rect{X: 10, Y: 20, Width: 200, Height: 100}, ui.Size{Width: 500, Height: 400}).Offset(999, -20)
	geometry := area.Geometry()
	if geometry.OffsetX != 300 || geometry.OffsetY != 0 || geometry.MaxOffsetX != 300 || geometry.MaxOffsetY != 300 {
		t.Fatalf("scroll offsets = %+v", geometry)
	}
	if geometry.VerticalTrack != (ui.Rect{X: 200, Y: 20, Width: 10, Height: 90}) {
		t.Fatalf("vertical track = %+v", geometry.VerticalTrack)
	}
	if geometry.HorizontalTrack != (ui.Rect{X: 10, Y: 110, Width: 190, Height: 10}) {
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
	var canvas ui.Canvas
	area := BeginScrollArea(&canvas, ui.LightTheme(), ui.Rect{X: 8, Y: 9, Width: 120, Height: 80}, ui.Size{Width: 240, Height: 160}).
		Offset(30, 40).
		VerticalScrollbar(ui.Item(11), "Vertical scroll", "50%").
		HorizontalScrollbar(ui.Item(12), "Horizontal scroll", "25%")
	geometry := area.BeginContent()
	canvas.Fill(geometry.Content, ui.LightTheme().Card)
	area.EndContent()
	if len(canvas.Clips()) != 0 {
		t.Fatalf("clip stack was not restored: %+v", canvas.Clips())
	}
	if !canvas.Rectangles()[0].HasClip || canvas.Rectangles()[0].ClipOf != geometry.Viewport {
		t.Fatalf("content clip = %+v", canvas.Rectangles()[0])
	}
	if len(canvas.Hits()) != 2 || canvas.Hits()[0].Role != ui.RoleSeparator || canvas.Hits()[1].Role != ui.RoleSeparator {
		t.Fatalf("scrollbar semantics = %+v", canvas.Hits())
	}
	if canvas.Hits()[0].Value != "50%" || canvas.Hits()[1].Value != "25%" {
		t.Fatalf("scrollbar values = %+v", canvas.Hits())
	}
}

func TestScrollAreaDisabledGeometryIsStableAndAllocationFree(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	paint := func() {
		canvas.Reset()
		area := BeginScrollArea(&canvas, theme, ui.Rect{Width: 160, Height: 90}, ui.Size{Width: 160, Height: 360}).
			Offset(0, 130).VerticalScrollbar(ui.Item(13), "Messages", "48%").Disabled()
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
	bounds := ui.Rect{X: 10, Y: 20, Width: 300, Height: 200}
	content := ui.Size{Width: 300, Height: 1000}
	for _, offset := range []float32{0, 1, 137.5, 400, 799, 800} {
		geometry := BeginScrollArea(nil, ui.LightTheme(), bounds, content).Offset(0, offset).Geometry()
		got := geometry.VerticalOffsetForThumb(geometry.VerticalThumb.Y)
		if got < offset-0.01 || got > offset+0.01 {
			t.Fatalf("offset %v: thumb at %v maps back to %v", offset, geometry.VerticalThumb.Y, got)
		}
	}

	geometry := BeginScrollArea(nil, ui.LightTheme(), bounds, content).Geometry()
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

	short := BeginScrollArea(nil, ui.LightTheme(), bounds, ui.Size{Width: 300, Height: 100}).Geometry()
	if got := short.VerticalOffsetForThumb(50); got != 0 {
		t.Fatalf("content that fits has nothing to scroll, got %v", got)
	}
}

// A named viewport is a focusable group, and the key helper moves the
// caller-owned offsets by a step, a page, or to an end.
func TestScrollAreaViewportAndKeys(t *testing.T) {
	var canvas ui.Canvas
	area := BeginScrollArea(&canvas, ui.LightTheme(), ui.Rect{Width: 200, Height: 100}, ui.Size{Width: 400, Height: 300}).Viewport(ui.Item(7), "Scrollable content")
	area.BeginContent()
	area.EndContent()
	found := false
	for _, hit := range canvas.Hits() {
		if hit.ID == ui.Item(7) && hit.Role == ui.RoleGroup && hit.Focusable {
			found = true
		}
	}
	if !found {
		t.Fatalf("no focusable viewport: %+v", canvas.Hits())
	}
	for _, test := range []struct {
		name string
		x, y float32
		key  ui.Key
		want float32
	}{
		{"down", 0, 0, ui.KeyDown, 32},
		{"page down", 0, 0, ui.KeyPageDown, 90},
		{"end", 0, 0, ui.KeyEnd, 200},
		{"home", 0, 200, ui.KeyHome, 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			x, y, handled := area.ScrollForKey(test.x, test.y, test.key, 32)
			if !handled || x != 0 || y != test.want {
				t.Fatalf("= (%v, %v, %v), want y %v", x, y, handled, test.want)
			}
		})
	}
	if x, _, handled := area.ScrollForKey(0, 0, ui.KeyRight, 32); !handled || x != 32 {
		t.Fatalf("Right = (%v, %v)", x, handled)
	}
	if _, _, handled := area.ScrollForKey(0, 0, ui.KeyEnter, 32); handled {
		t.Fatal("Enter is not a scroll key")
	}
}

// Reserved bars take their strips out of the viewport, and only the strips the content needs:
// content as wide as the strip beside the vertical bar gets no horizontal bar.
func TestScrollAreaReserveScrollbarsKeepsContentClearOfTheBars(t *testing.T) {
	bounds := ui.Rect{X: 10, Y: 20, Width: 200, Height: 100}
	tall := BeginScrollArea(nil, ui.LightTheme(), bounds, ui.Size{Width: 190, Height: 400}).ReserveScrollbars().Geometry()
	if tall.Viewport != (ui.Rect{X: 10, Y: 20, Width: 190, Height: 100}) || tall.MaxOffsetX != 0 || tall.MaxOffsetY != 300 {
		t.Fatalf("tall content: viewport %+v max %v,%v", tall.Viewport, tall.MaxOffsetX, tall.MaxOffsetY)
	}
	if tall.HorizontalTrack != (ui.Rect{}) || tall.VerticalTrack != (ui.Rect{X: 200, Y: 20, Width: 10, Height: 100}) {
		t.Fatalf("tall content: tracks %+v %+v", tall.HorizontalTrack, tall.VerticalTrack)
	}
	both := BeginScrollArea(nil, ui.LightTheme(), bounds, ui.Size{Width: 500, Height: 400}).ReserveScrollbars().Geometry()
	if both.Viewport != (ui.Rect{X: 10, Y: 20, Width: 190, Height: 90}) || both.MaxOffsetX != 310 || both.MaxOffsetY != 310 {
		t.Fatalf("wide and tall content: viewport %+v max %v,%v", both.Viewport, both.MaxOffsetX, both.MaxOffsetY)
	}
	if both.Corner == (ui.Rect{}) || both.VerticalTrack.Height != both.Viewport.Height || both.HorizontalTrack.Width != both.Viewport.Width {
		t.Fatalf("wide and tall content: corner %+v tracks %+v %+v", both.Corner, both.VerticalTrack, both.HorizontalTrack)
	}
	// Adding the vertical bar narrows the viewport enough to need the horizontal one too.
	edge := BeginScrollArea(nil, ui.LightTheme(), bounds, ui.Size{Width: 195, Height: 400}).ReserveScrollbars().Geometry()
	if edge.MaxOffsetX == 0 || edge.HorizontalTrack == (ui.Rect{}) {
		t.Fatalf("content wider than the viewport beside the bar has no horizontal bar: %+v", edge)
	}
	// Without it the bars overlay the content, as before.
	overlay := BeginScrollArea(nil, ui.LightTheme(), bounds, ui.Size{Width: 200, Height: 400}).Geometry()
	if overlay.Viewport != bounds || overlay.MaxOffsetX != 0 {
		t.Fatalf("default viewport %+v max %v", overlay.Viewport, overlay.MaxOffsetX)
	}
}
