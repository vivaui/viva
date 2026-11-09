//go:build vivahead && darwin

package headless

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// A label has the same size in logical pixels at every display scale. At a
// fractional one its bitmap used to be drawn at scale/ceil(scale) of its size.
func TestLabelKeepsItsLogicalSizeAtFractionalScales(t *testing.T) {
	width := func(scale float64) float64 {
		session, err := Open(app.Config{Title: "label", Width: 300, Height: 60, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Label("Hello web-01 你好", gfx.Rect{X: 10, Y: 10, Width: 280, Height: 40},
				gfx.TextStyle{Size: 13, Weight: gfx.WeightRegular, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}})
		}}, nil, Options{Scale: scale})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		path := filepath.Join(t.TempDir(), "label.png")
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture := readPNG(t, path)
		left, right := picture.Bounds().Max.X, 0
		for y := 0; y < picture.Bounds().Max.Y; y++ {
			for x := 0; x < picture.Bounds().Max.X; x++ {
				if r, _, _, _ := picture.At(x, y).RGBA(); r > 0x8000 {
					left, right = min(left, x), max(right, x)
				}
			}
		}
		if right < left {
			t.Fatalf("scale %g drew no text", scale)
		}
		return float64(right-left+1) / scale
	}
	want := width(1)
	for _, scale := range []float64{1.25, 1.5, 1.75, 2} {
		if got := width(scale); got < want-3 || got > want+3 {
			t.Errorf("text is %.1f logical pixels wide at scale %g, %.1f at scale 1", got, scale, want)
		}
	}
}

func readPNG(t *testing.T, path string) image.Image {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	picture, err := png.Decode(file)
	if err != nil {
		t.Fatal(err)
	}
	return picture
}

// A picture drawn pixel for pixel reaches the screen without being resampled:
// stripes one pixel wide stay black and white at 125% and 150%, where drawing
// the same picture scaled to the same size smears them into greys.
func TestSurfaceDrawPixelsIsNotResampledAtFractionalScales(t *testing.T) {
	var surface gfx.Surface
	const width, height = 600, 40
	surface.Resize(width, height)
	stripes := make([]byte, width*height*4)
	for y := range height {
		for x := range width {
			value := byte(0)
			if x%2 == 0 {
				value = 255
			}
			offset := (y*width + x) * 4
			stripes[offset], stripes[offset+1], stripes[offset+2] = value, value, value
		}
	}
	surface.Update(0, 0, width, height, stripes, width*4)

	// greys counts the pixels of the picture's row that are neither black nor
	// white, and whites those that are white.
	greys := func(scale float64, pixels bool) (greys, whites int) {
		session, err := Open(app.Config{Title: "pixels", Width: 500, Height: 60, Paint: func(canvas *gfx.Canvas, size gfx.Size) {
			canvas.Fill(gfx.Rect{Width: size.Width, Height: size.Height}, gfx.Color{Alpha: 1})
			if pixels {
				surface.DrawPixels(canvas, gfx.Point{X: 10, Y: 10})
				return
			}
			surface.Draw(canvas, gfx.Rect{X: 10, Y: 10, Width: width / float32(scale), Height: height / float32(scale)})
		}}, nil, Options{Scale: scale})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		path := filepath.Join(t.TempDir(), "pixels.png")
		if err := session.Screenshot(path); err != nil {
			t.Fatal(err)
		}
		picture := readPNG(t, path)
		row := int(25 * scale)
		for x := int(12 * scale); x < int(float64(10)*scale)+width-4; x++ {
			r, _, _, _ := picture.At(x, row).RGBA()
			switch {
			case r > 0xf000:
				whites++
			case r > 0x0fff:
				greys++
			}
		}
		return greys, whites
	}
	for _, scale := range []float64{1.25, 1.5} {
		grey, white := greys(scale, true)
		if grey != 0 || white < 100 {
			t.Errorf("scale %g: DrawPixels drew %d grey and %d white pixels from stripes of black and white", scale, grey, white)
		}
		grey, white = greys(scale, false)
		t.Logf("scale %g: Draw scaled to the same size drew %d grey and %d white pixels", scale, grey, white)
	}
}
