package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestBadgeOfficialVariants(t *testing.T) {
	theme := gfx.DarkTheme()
	destructiveBackground := withAlpha(theme.Danger, 0.20)
	tests := [...]struct {
		variant    BadgeVariant
		background gfx.Color
		foreground gfx.Color
		rectangles int
	}{
		{BadgeDefault, theme.Primary, theme.PrimaryForeground, 1},
		{BadgeSecondary, theme.Secondary, theme.SecondaryForeground, 1},
		{BadgeDestructive, destructiveBackground, theme.Danger, 1},
		{BadgeGhost, gfx.Color{}, theme.Foreground, 0},
		{BadgeLink, gfx.Color{}, theme.Primary, 0},
	}
	for _, test := range tests {
		var canvas gfx.Canvas
		canvas.SetMeasurer(func(value string, size, weight float32) float32 { return 36 })
		bounds := PaintBadge(&canvas, theme, gfx.Rect{X: 7, Y: 9, Width: 800, Height: 200}, "Status", test.variant)
		if bounds != (gfx.Rect{X: 7, Y: 9, Width: 54, Height: 20}) {
			t.Fatalf("variant %d bounds = %+v, want natural text width + px-2 padding + the 1px border", test.variant, bounds)
		}
		if len(canvas.Rectangles()) != test.rectangles {
			t.Fatalf("variant %d painted %d rectangles, want %d", test.variant, len(canvas.Rectangles()), test.rectangles)
		}
		if test.rectangles != 0 && canvas.Rectangles()[0].Color != test.background {
			t.Fatalf("variant %d background = %+v, want %+v", test.variant, canvas.Rectangles()[0].Color, test.background)
		}
		if text := canvas.Texts()[0]; text.Style.Color != test.foreground || text.Style.Weight != gfx.WeightMedium {
			t.Fatalf("variant %d text = %+v", test.variant, text.Style)
		}
	}
}

func TestBadgeOutlinePaintsOfficialBorderWithoutStatusDot(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetMeasurer(func(value string, size, weight float32) float32 { return 42 })
	bounds := PaintBadge(&canvas, theme, gfx.Rect{X: 8, Y: 8, Width: 96, Height: 20}, "Outline", BadgeOutline)
	if bounds.Width != 60 {
		t.Fatalf("outline width = %v, want measured label width + 18", bounds.Width)
	}
	rectangles := canvas.Rectangles()
	if len(rectangles) != 2 {
		t.Fatalf("outline painted %d rectangles, want border + inset fill", len(rectangles))
	}
	if rectangles[0].Color != theme.Border || rectangles[0].Radius != 10 || rectangles[0].Bounds.Width != 60 {
		t.Fatalf("outline border = %+v", rectangles[0])
	}
	if len(canvas.Texts()) != 1 || canvas.Texts()[0].Style.Color != theme.Foreground {
		t.Fatalf("outline text = %+v", canvas.Texts())
	}
}

func TestBadgeRepeatedPaintReusesCanvasStorage(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	PaintBadge(&canvas, theme, gfx.Rect{}, "Ready", BadgeDefault)
	allocations := testing.AllocsPerRun(100, func() {
		canvas.Reset()
		PaintBadge(&canvas, theme, gfx.Rect{}, "Ready", BadgeDefault)
	})
	if allocations != 0 {
		t.Fatalf("repeated badge paint allocated %v times", allocations)
	}
}
