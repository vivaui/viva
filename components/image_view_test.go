package components

import (
	"github.com/vivaui/viva/gfx"
	"math"
	"testing"
)

func TestImageViewFitActualSizeAndPanAtDisplayScales(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		var canvas gfx.Canvas
		canvas.SetPixelScale(scale)
		bounds := gfx.Rect{X: 20, Y: 30, Width: 320, Height: 200}
		bitmap := gfx.Bitmap{Pix: make([]byte, 640*480*4), Width: 640, Height: 480, Stride: 640 * 4, ID: 1}
		fit := PaintImageView(&canvas, gfx.LightTheme(), bounds, ImageViewOptions{Bitmap: bitmap})
		if fit.Image.Width > bounds.Width || fit.Image.Height > bounds.Height || fit.Image.X < bounds.X || fit.Image.Y < bounds.Y {
			t.Fatalf("fit %g: %+v", scale, fit)
		}
		actual := PaintImageView(&canvas, gfx.LightTheme(), bounds, ImageViewOptions{Bitmap: bitmap, Zoom: 1, Pan: gfx.Point{X: 1e6, Y: -1e6}})
		if math.Abs(float64(actual.Image.Width*scale-640)) > .001 || math.Abs(float64(actual.Image.Height*scale-480)) > .001 {
			t.Fatalf("actual size %g: %+v", scale, actual)
		}
		for _, coordinate := range []float32{actual.Image.X, actual.Image.Y} {
			pixels := float64(coordinate * scale)
			if math.Abs(pixels-math.Round(pixels)) > .001 {
				t.Fatal("actual-size origin falls between device pixels", scale, coordinate)
			}
		}
		if actual.Image.Width > bounds.Width && actual.Image.X > bounds.X+.001 {
			t.Fatal("pan exposed empty horizontal space")
		}
		if actual.Image.Height > bounds.Height && actual.Image.Y+actual.Image.Height < bounds.Y+bounds.Height-.001 {
			t.Fatal("pan exposed empty vertical space")
		}
	}
}
