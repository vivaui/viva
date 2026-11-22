package raster

import (
	"testing"

	"github.com/vivaui/viva/internal/scene"
)

// coverageBitmap is a 16x16 opaque white BGRA bitmap.
func coverageBitmap() []byte {
	pix := make([]byte, 16*16*4)
	for i := range pix {
		pix[i] = 255
	}
	return pix
}

func TestImageLabelRoundsItsCorners(t *testing.T) {
	draw := func(radius float32) *Canvas {
		canvas := NewCanvas(32, 32)
		var drawer Rasterizer
		drawer.Draw(canvas, Params{Scale: 1}, nil, []Label{{
			Pix: coverageBitmap(), Stride: 16 * 4, Width: 16, Height: 16, X: 8, Y: 8,
			Image: true, Radius: radius,
			Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		}})
		return canvas
	}
	square := draw(0)
	round := draw(8)
	alpha := func(c *Canvas, x, y int) byte { return c.Pix[y*c.Stride+x*4+3] }
	if alpha(square, 8, 8) != 255 || alpha(square, 23, 23) != 255 {
		t.Fatalf("a square image does not cover its corner: got %d", alpha(square, 23, 23))
	}
	if got := alpha(round, 23, 23); got != 0 {
		t.Errorf("the rounded corner kept %d alpha, want 0", got)
	}
	if got := alpha(round, 16, 16); got != 255 {
		t.Errorf("the rounded image's center kept %d alpha, want 255", got)
	}
}

func TestImageLabelFadesByItsOpacity(t *testing.T) {
	canvas := NewCanvas(32, 32)
	var drawer Rasterizer
	drawer.Draw(canvas, Params{Scale: 1}, nil, []Label{{
		Pix: coverageBitmap(), Stride: 16 * 4, Width: 16, Height: 16, X: 8, Y: 8,
		Image: true, Fade: 0.5,
		Color: scene.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}})
	got := canvas.Pix[16*canvas.Stride+16*4+3]
	if got < 120 || got > 136 {
		t.Errorf("half-faded image alpha = %d, want about 128", got)
	}
}
