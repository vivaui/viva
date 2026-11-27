package gfx_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/framebuild"
	"github.com/vivaui/viva/internal/raster"
	"github.com/vivaui/viva/internal/render"
)

func TestIconClipPreservesThinStrokeCoverage(t *testing.T) {
	icon := gfx.Icon{
		ViewBox: 32, Stroke: 1, Cap: gfx.CapButt,
		Commands: []gfx.PathCommand{gfx.MoveTo(12.25, 4), gfx.LineTo(12.25, 28)},
	}
	bounds := gfx.Rect{Width: 32, Height: 32}
	for _, test := range []struct {
		name string
		clip *gfx.IconClip
		from int
		to   int
	}{
		{name: "unclipped", from: 5, to: 27},
		{name: "contained", clip: &gfx.IconClip{Bounds: bounds}, from: 5, to: 27},
		{name: "cut", clip: &gfx.IconClip{Bounds: gfx.Rect{Y: 6, Width: 32, Height: 20}}, from: 6, to: 26},
		{name: "cut-rounded", clip: &gfx.IconClip{Bounds: gfx.Rect{Y: 6, Width: 24, Height: 20}, Radius: 10}, from: 6, to: 26},
		{name: "cut-across-width", clip: &gfx.IconClip{Bounds: gfx.Rect{X: 12, Y: 6, Width: 2, Height: 20}}, from: 6, to: 26},
		{name: "cut-away-both-edges", clip: &gfx.IconClip{Bounds: gfx.Rect{X: 12.375, Y: 6, Width: 0.25, Height: 20}}, from: 6, to: 26},
	} {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			canvas.DrawIconTransformed(icon, bounds, gfx.Color{Red: 1, Alpha: 1}, gfx.IconTransform{Clip: test.clip})
			instances := make([]render.Rectangle, 0, len(canvas.Triangles()))
			for _, triangle := range canvas.Triangles() {
				instances = append(instances, framebuild.SceneTriangle(triangle))
			}
			pixels := raster.NewCanvas(32, 32)
			var renderer raster.Rasterizer
			renderer.Draw(pixels, raster.Params{Scale: 1}, instances, nil)
			for y := test.from; y < test.to; y++ {
				// The center at x=12.5 is a quarter pixel from the right
				// stroke edge, giving 75% coverage regardless of the cut.
				if got := pixels.Pix[y*pixels.Stride+12*4+3]; got < 190 || got > 192 {
					t.Fatalf("alpha at (12, %d) = %d, want 191", y, got)
				}
			}
			if test.from == 6 {
				for _, y := range []int{5, 26} {
					for x := 11; x <= 12; x++ {
						if got := pixels.Pix[y*pixels.Stride+x*4+3]; got != 0 {
							t.Fatalf("alpha outside clip at (%d, %d) = %d, want 0", x, y, got)
						}
					}
				}
			}
		})
	}
}
