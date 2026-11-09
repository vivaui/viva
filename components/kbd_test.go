package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestKbdFitsContentAndUsesNeutralTokens(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	bounds := Kbd(&canvas, theme, gfx.Rect{X: 4, Y: 6}, "⌘K")
	if bounds.Width < bounds.Height || bounds.Height != theme.Metrics.FontSizeS+9 {
		t.Fatalf("fitted keycap bounds = %+v", bounds)
	}
	// A hairline border in theme.Border, then the muted fill inside it.
	rectangles := canvas.Rectangles()
	if len(rectangles) != 2 || rectangles[0].Color != theme.Border || rectangles[0].Radius != kbdRadius(theme) ||
		rectangles[1].Color != theme.Muted || rectangles[1].Bounds != (gfx.Rect{X: bounds.X + 1, Y: bounds.Y + 1, Width: bounds.Width - 2, Height: bounds.Height - 2}) {
		t.Fatalf("keycap rectangles = %+v", rectangles)
	}
	if len(canvas.Texts()) != 1 || canvas.Texts()[0].Style.Weight != gfx.WeightMedium || canvas.Texts()[0].Style.Color != theme.MutedForeground {
		t.Fatalf("keycap text = %+v", canvas.Texts())
	}
}

func TestKbdGroupUsesFourPixelGap(t *testing.T) {
	var canvas gfx.Canvas
	group := BeginKbdGroup(&canvas, gfx.LightTheme(), gfx.Rect{X: 3, Y: 5})
	first := group.Add("⌘")
	second := group.Add("K")
	if second.X != first.X+first.Width+4 || second.Y != first.Y {
		t.Fatalf("keycap group bounds = %+v then %+v", first, second)
	}
}

func TestKbdRepeatedPaintIsStableAndAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Kbd(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 30, Height: 22}, "⌘")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical keycaps produced different command streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("kbd paint allocations = %v, want 0 after warm-up", allocations)
	}
}
