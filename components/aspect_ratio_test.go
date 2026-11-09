package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestAspectRatioFitsAndCenters(t *testing.T) {
	bounds := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 200}
	got := AspectRatio(bounds, 16.0/9.0)
	if got.Width != 200 || got.Height != 112.5 || got.X != 10 || got.Y != 63.75 {
		t.Fatalf("16:9 fit = %+v", got)
	}
	portrait := AspectRatio(bounds, 3.0/4.0)
	if portrait.Width != 150 || portrait.Height != 200 || portrait.X != 35 || portrait.Y != 20 {
		t.Fatalf("3:4 fit = %+v", portrait)
	}
}

func TestAspectRatioInvalidInputIsStableAndAllocationFree(t *testing.T) {
	bounds := gfx.Rect{X: 5, Y: 7, Width: 100, Height: 80}
	want := gfx.Rect{X: 55, Y: 47}
	if got := AspectRatio(bounds, 0); got != want {
		t.Fatalf("invalid ratio = %+v, want %+v", got, want)
	}
	if got := AspectRatio(bounds, float32(math.NaN())); got != want {
		t.Fatalf("unordered ratio = %+v, want %+v", got, want)
	}
	if got := AspectRatio(bounds, float32(math.Inf(1))); got != want {
		t.Fatalf("infinite ratio = %+v, want %+v", got, want)
	}
	if allocations := testing.AllocsPerRun(100, func() {
		_ = AspectRatio(bounds, 4.0/3.0)
	}); allocations != 0 {
		t.Fatalf("aspect-ratio allocations = %v, want 0", allocations)
	}
}

func TestAspectRatioDerivesHeightFromWidthWhenUnconstrained(t *testing.T) {
	got := AspectRatio(gfx.Rect{X: 8, Y: 12, Width: 160}, 16.0/9.0)
	want := gfx.Rect{X: 8, Y: 12, Width: 160, Height: 90}
	if got != want {
		t.Fatalf("unconstrained ratio = %+v, want %+v", got, want)
	}
}
