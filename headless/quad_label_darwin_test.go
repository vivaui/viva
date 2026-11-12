//go:build darwin && vivahead

package headless

import (
	"image"
	"os"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// TestScreenshotDrawsALabelIntoItsQuad runs a line through the whole path a
// window uses — canvas, snapshot, platform label and Metal renderer — and pins
// that a warped line's ink lands where its quad places it and takes the corner
// tints. A renderer that ignored either would leave the line in its box and
// white.
func TestScreenshotDrawsALabelIntoItsQuad(t *testing.T) {
	render := func(warped bool) *image.RGBA {
		t.Helper()
		config := app.Config{
			Title: "quad", Width: 200, Height: 120,
			ClearColor: app.Color{Alpha: 1},
			Paint: func(canvas *gfx.Canvas, size gfx.Size) {
				bounds := gfx.Rect{X: 10, Y: 60, Width: 180, Height: 24}
				white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
				line := strings.Repeat("M", 30)
				if !warped {
					canvas.Label(line, bounds, gfx.TextStyle{Size: 18, Color: white})
					return
				}
				// The right edge of the quad is raised by 40 logical pixels, so at
				// x=160..180 the line is at y≈27..49, and its right corner is green
				// instead of white.
				canvas.LabelQuad(line, bounds, gfx.Quad{
					Points: [4]gfx.Point{{X: 10, Y: 60}, {X: 190, Y: 20}, {X: 10, Y: 84}, {X: 190, Y: 44}},
					Tint:   [4]gfx.Color{white, {Green: 1, Alpha: 1}, white, {Green: 1, Alpha: 1}},
				}, gfx.TextStyle{Size: 18, Color: white})
			},
		}
		session, err := Open(config, nil, Options{Scale: 1})
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		picture, err := session.Image()
		if err != nil {
			t.Skipf("no renderer: %v", err)
		}
		if path := os.Getenv("VIVA_TEST_SHOT"); path != "" && warped {
			if err := session.Screenshot(path); err != nil {
				t.Fatal(err)
			}
		}
		return picture
	}
	// ink counts the painted pixels of a window and how many of them are white
	// enough to have kept a white tint.
	ink := func(picture *image.RGBA, area image.Rectangle) (count, whiteCount int) {
		for y := area.Min.Y; y < area.Max.Y; y++ {
			for x := area.Min.X; x < area.Max.X; x++ {
				pixel := picture.RGBAAt(x, y)
				if pixel.R > 60 {
					count++
					whiteCount++
				} else if pixel.G > 60 || pixel.B > 60 {
					count++
				}
			}
		}
		return count, whiteCount
	}
	raised := image.Rect(160, 28, 180, 48)
	box := image.Rect(160, 64, 180, 80)
	plain := render(false)
	if count, _ := ink(plain, raised); count != 0 {
		t.Fatalf("an axis-aligned label painted %d pixels above its box", count)
	}
	if count, _ := ink(plain, box); count < 20 {
		t.Fatalf("an axis-aligned label has %d ink pixels in its box, want its line", count)
	}
	warped := render(true)
	count, whiteCount := ink(warped, raised)
	if count < 20 {
		t.Fatalf("the warped label has %d ink pixels in its quad, want its line", count)
	}
	if whiteCount != 0 {
		t.Fatalf("%d of %d raised ink pixels are white, want the right corner's green tint", whiteCount, count)
	}
	if count, _ := ink(warped, box); count != 0 {
		t.Fatalf("the warped label still painted %d pixels in its rasterisation box", count)
	}
}
