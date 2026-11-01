package linux

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
	"github.com/vivaui/viva/ui"
)

func TestCPUOffscreenDrawsShapesAndTextWithoutVulkan(t *testing.T) {
	clear := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	off, err := NewCPUOffscreen(120, 60, 1.5, clear)
	if err != nil {
		t.Skipf("no text services: %v", err) // Pango and Cairo are system libraries
	}
	defer off.Close()
	bar := render.Rectangle{Kind: scene.KindRectangle, Bounds: scene.Rect{X: 0, Y: 40, Width: 120, Height: 20}, Color: scene.Color{Blue: 1, Alpha: 1}}
	label := platform.TextLabel{Value: "Hello", X: 10, Y: 8, Width: 70, Height: 20, FontSize: 14, Color: platform.Color{Alpha: 1}}
	clipped := label
	clipped.Y = 30
	clipped.Value = "Clipped"
	clipped.Clipped, clipped.Clip = true, ui.Rect{X: 0, Y: 30, Width: 5, Height: 20} // shows none of the text
	if err := off.Set([]render.Rectangle{bar}, []platform.TextLabel{label, clipped}); err != nil {
		t.Fatal(err)
	}
	picture, err := off.Image()
	if err != nil {
		t.Fatal(err)
	}
	if picture.Bounds().Dx() != 180 || picture.Bounds().Dy() != 90 {
		t.Fatalf("image is %v, want 180x90 device pixels", picture.Bounds())
	}
	dark := func(x0, y0, x1, y1 int) int {
		n := 0
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if c := picture.RGBAAt(x, y); c.R < 128 && c.G < 128 && c.B < 128 {
					n++
				}
			}
		}
		return n
	}
	if n := dark(15, 12, 125, 42); n < 20 {
		t.Fatalf("only %d dark pixels where the label is drawn", n)
	}
	if n := dark(0, 45, 180, 55); n != 0 {
		t.Fatalf("%d dark pixels in the clipped label's row; the clip leaves no glyph", n)
	}
	if c := picture.RGBAAt(100, 80); c.B != 255 || c.R != 0 || c.A != 255 {
		t.Fatalf("pixel in the blue bar = %+v", c)
	}
}
