//go:build !android

package linux

import (
	"image/color"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/render"
	"github.com/vivaui/viva/internal/scene"
)

func TestCPUOffscreenMagnifiesBitmapInsideClip(t *testing.T) {
	offscreen, err := NewCPUOffscreen(20, 10, 1.5, platform.Color{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(offscreen.Close)
	label := bitmapLabel()
	label.Bitmap = []byte{0, 0, 128, 128, 0, 255, 0, 255}
	label.Clipped = true
	label.Clip = gfx.Rect{X: 3, Y: 3, Width: 14, Height: 4}
	if err := offscreen.Set(nil, []platform.TextLabel{label}); err != nil {
		t.Fatal(err)
	}
	result, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	for _, check := range []struct {
		x, y int
		want color.RGBA
	}{
		{4, 4, color.RGBA{R: 128, A: 128}},
		{14, 7, color.RGBA{R: 69, G: 117, A: 186}},
		{25, 10, color.RGBA{G: 255, A: 255}},
		{3, 4, color.RGBA{}},
		{26, 10, color.RGBA{}},
		{25, 11, color.RGBA{}},
	} {
		if got := result.RGBAAt(check.x, check.y); got != check.want {
			t.Errorf("pixel (%d, %d) = %+v, want %+v", check.x, check.y, got, check.want)
		}
	}
}

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
	clipped.Clipped, clipped.Clip = true, gfx.Rect{X: 0, Y: 30, Width: 5, Height: 20} // shows none of the text
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
