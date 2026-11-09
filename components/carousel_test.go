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
