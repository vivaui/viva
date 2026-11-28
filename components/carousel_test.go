package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestCarouselHorizontalGeometryClampsIndexOffsetAndItems(t *testing.T) {
	carousel := BeginCarousel(nil, gfx.LightTheme(), gfx.Rect{X: 100, Y: 40, Width: 300, Height: 180}, 3, 99).Content(940, 999)
	geometry := carousel.Geometry()
	if geometry.Index != 2 || geometry.Offset != 640 || geometry.MaxOffset != 640 || !geometry.CanPrevious || geometry.CanNext {
		t.Fatalf("horizontal carousel = %+v", geometry)
	}
	item := carousel.ItemGeometry(2, 640, 300)
	if item.Bounds != geometry.Viewport || !item.Visible || !item.Selected {
		t.Fatalf("selected item = %+v viewport=%+v", item, geometry.Viewport)
	}
	if geometry.PreviousButton.X+geometry.PreviousButton.Width+carouselControlOffset != geometry.Viewport.X {
		t.Fatalf("previous button placement = %+v", geometry.PreviousButton)
	}
}

// An RTL carousel mirrors the layout: the first slide sits at the viewport's
// right edge, the offset reveals slides on its left, the previous control
// stands on the right of the viewport and the next one on the left, and the
// arrows point back the other way. The selected slide still fills the viewport.
func TestCarouselRTLGeometryMirrorsTheReadingDirection(t *testing.T) {
	var canvas gfx.Canvas
	carousel := BeginCarousel(&canvas, gfx.LightTheme(), gfx.Rect{X: 100, Y: 40, Width: 300, Height: 180}, 3, 1).
		Content(900, 300).Direction(RTL).Controls(gfx.Item(41), "Previous slide", gfx.Item(42), "Next slide")
	geometry := carousel.BeginContent()
	// Offset 300 shows the second slide: its content window [300, 600) lands on
	// the viewport [100, 400), and the content's right edge — the first slide's
	// inline start — stands 300 past the viewport's right edge, at x=700.
	if geometry.Content.X != 100+300+300-900 || geometry.Content.Width != 900 {
		t.Fatalf("RTL content = %+v, want X %v", geometry.Content, 100+300+300-900)
	}
	first := carousel.ItemGeometry(0, 0, 300)
	if first.Bounds.X != 400 || first.Bounds.X+first.Bounds.Width != 700 || first.Visible {
		t.Fatalf("RTL first slide = %+v, want it outside the right edge at [400, 700)", first.Bounds)
	}
	second := carousel.ItemGeometry(1, 300, 300)
	if second.Bounds != geometry.Viewport || !second.Visible || !second.Selected {
		t.Fatalf("RTL selected slide = %+v viewport=%+v", second.Bounds, geometry.Viewport)
	}
	third := carousel.ItemGeometry(2, 600, 300)
	if third.Bounds.X != -200 || third.Visible {
		t.Fatalf("RTL last slide = %+v, want it outside the left edge at [-200, 100)", third.Bounds)
	}
	// The controls swap sides: previous on the right of the viewport, next on
	// the left, and each arrow points back toward its own slide.
	if geometry.PreviousButton.X != geometry.Viewport.X+geometry.Viewport.Width+carouselControlOffset {
		t.Fatalf("RTL previous button = %+v, want it right of the viewport", geometry.PreviousButton)
	}
	if geometry.NextButton.X+geometry.NextButton.Width+carouselControlOffset != geometry.Viewport.X {
		t.Fatalf("RTL next button = %+v, want it left of the viewport", geometry.NextButton)
	}
	carousel.EndContent()
	hits := canvas.Hits()
	if len(hits) != 2 || hits[0].Bounds != geometry.PreviousButton || hits[1].Bounds != geometry.NextButton {
		t.Fatalf("RTL control hits = %+v geometry=%+v", hits, geometry)
	}
	// Vertical keeps its own axis: the direction does not move anything.
	vertical := BeginCarousel(nil, gfx.LightTheme(), gfx.Rect{X: 50, Y: 60, Width: 180, Height: 240}, 3, 1).
		Content(900, 300).Vertical().Direction(RTL)
	verticalGeometry := vertical.Geometry()
	if verticalGeometry.Content.Y != 60-300 || verticalGeometry.PreviousButton.X != 50+(180-verticalGeometry.PreviousButton.Width)/2 {
		t.Fatalf("RTL vertical carousel = %+v", verticalGeometry)
	}
}

func TestCarouselVerticalGeometryAndClipping(t *testing.T) {
	var canvas gfx.Canvas
	carousel := BeginCarousel(&canvas, gfx.LightTheme(), gfx.Rect{X: 50, Y: 60, Width: 180, Height: 240}, 4, 1).
		Content(1000, 250).Vertical().Controls(gfx.Item(31), "Previous slide", gfx.Item(32), "Next slide")
	geometry := carousel.BeginContent()
	item := carousel.ItemGeometry(1, 250, 240)
	canvas.Fill(item.Bounds, gfx.LightTheme().Card)
	carousel.EndContent()
	if geometry.Content.Y != -190 || item.Bounds.Y != 60 || item.Bounds.Height != 240 {
		t.Fatalf("vertical carousel geometry = %+v item=%+v", geometry, item)
	}
	if !canvas.Rectangles()[0].HasClip || canvas.Rectangles()[0].ClipOf != geometry.Viewport {
		t.Fatalf("carousel clip = %+v", canvas.Rectangles()[0])
	}
	if geometry.PreviousButton.Y+geometry.PreviousButton.Height+carouselControlOffset != geometry.Viewport.Y || geometry.NextButton.Y != geometry.Viewport.Y+geometry.Viewport.Height+carouselControlOffset {
		t.Fatalf("vertical button placement = prev=%+v next=%+v", geometry.PreviousButton, geometry.NextButton)
	}
}

func TestCarouselButtonsExposeDisabledBoundsAndSemantics(t *testing.T) {
	var canvas gfx.Canvas
	carousel := BeginCarousel(&canvas, gfx.LightTheme(), gfx.Rect{Width: 240, Height: 140}, 3, 0).
		Content(720, 0).Controls(gfx.Item(33), "Previous slide", gfx.Item(34), "Next slide")
	carousel.BeginContent()
	geometry := carousel.EndContent()
	if len(canvas.Hits()) != 2 {
		t.Fatalf("carousel hits = %+v", canvas.Hits())
	}
	previous, next := canvas.Hits()[0], canvas.Hits()[1]
	if previous.Bounds != geometry.PreviousButton || previous.Role != gfx.RoleButton || !previous.Disabled || previous.Focusable {
		t.Fatalf("previous semantics = %+v", previous)
	}
	if next.Bounds != geometry.NextButton || next.Role != gfx.RoleButton || next.Disabled || !next.Focusable {
		t.Fatalf("next semantics = %+v", next)
	}
}

func TestCarouselDeterministicStreamAndNoRepeatedPaintAllocations(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		carousel := BeginCarousel(&canvas, theme, gfx.Rect{X: 60, Y: 40, Width: 280, Height: 160}, 5, 2).
			Content(1440, 580).Controls(gfx.Item(35), "Previous slide", gfx.Item(36), "Next slide")
		carousel.BeginContent()
		item := carousel.ItemGeometry(2, 580, 280)
		canvas.Rounded(item.Bounds, theme.Metrics.RadiusSurface, theme.Card)
		carousel.EndContent()
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	hits := slices.Clone(canvas.Hits())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) || !slices.Equal(hits, canvas.Hits()) {
		t.Fatal("identical carousels produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("carousel paint allocations = %v", allocations)
	}
}

// The carousel region moves one slide with the arrows and to the ends with
// Home/End, without wrapping.
func TestCarouselStepForKey(t *testing.T) {
	if next, ok := CarouselStepForKey(gfx.KeyRight, 0, 3, LTR); !ok || next != 1 {
		t.Fatalf("Right = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyLeft, 0, 3, LTR); !ok || next != 0 {
		t.Fatalf("Left at start = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyEnd, 0, 3, LTR); !ok || next != 2 {
		t.Fatalf("End = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyHome, 2, 3, LTR); !ok || next != 0 {
		t.Fatalf("Home = %v, %v", next, ok)
	}
	if _, ok := CarouselStepForKey(gfx.KeyEnter, 0, 3, LTR); ok {
		t.Fatal("Enter is not a carousel movement")
	}
}

// An RTL carousel reads right to left: Left shows the next slide and Right the
// previous one, while the vertical keys and the ends keep their meaning.
func TestCarouselStepForKeyFollowsTheReadingDirection(t *testing.T) {
	if next, ok := CarouselStepForKey(gfx.KeyLeft, 1, 3, RTL); !ok || next != 2 {
		t.Fatalf("RTL Left = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyRight, 1, 3, RTL); !ok || next != 0 {
		t.Fatalf("RTL Right = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyLeft, 2, 3, RTL); !ok || next != 2 {
		t.Fatalf("RTL Left at the last slide = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyRight, 0, 3, RTL); !ok || next != 0 {
		t.Fatalf("RTL Right at the first slide = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyUp, 1, 3, RTL); !ok || next != 0 {
		t.Fatalf("RTL Up = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyDown, 1, 3, RTL); !ok || next != 2 {
		t.Fatalf("RTL Down = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyHome, 2, 3, RTL); !ok || next != 0 {
		t.Fatalf("RTL Home = %v, %v", next, ok)
	}
	if next, ok := CarouselStepForKey(gfx.KeyEnd, 0, 3, RTL); !ok || next != 2 {
		t.Fatalf("RTL End = %v, %v", next, ok)
	}
}
