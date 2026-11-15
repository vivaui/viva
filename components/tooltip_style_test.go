package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A theme that sets the tooltip surface paints that surface: the padding and text size
// change the box, so an application's own tip geometry comes from its theme alone.
func TestTooltipStyleSizesTheSurface(t *testing.T) {
	theme := gfx.LightTheme()
	theme.Tooltip = gfx.TooltipStyle{PadX: 6, PadY: 10, FontSize: 13, Radius: 4}
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 400, Height: 300}
	anchor := gfx.Rect{X: 100, Y: 100, Width: 40, Height: 20}
	content := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{}).Paint("hint")
	if want := float32((13 + 5) + 2*10); content.Height != want {
		t.Fatalf("height = %v, want %v", content.Height, want)
	}
	if content.Width < 2*6 {
		t.Fatalf("width = %v, want at least the horizontal padding", content.Width)
	}
}

// A theme that sets nothing keeps Viva's own surface: the small text and the padded box.
func TestTooltipStyleDefaultsToTheSharedSurface(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	viewport := gfx.Rect{Width: 400, Height: 300}
	anchor := gfx.Rect{X: 100, Y: 100, Width: 40, Height: 20}
	content := BeginTooltip(&canvas, theme, viewport, anchor, TooltipOptions{}).Paint("hint")
	if want := (theme.Metrics.FontSizeS + 5) + 2*6; content.Height != want {
		t.Fatalf("height = %v, want %v", content.Height, want)
	}
}
