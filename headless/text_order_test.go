//go:build darwin || windows || linux

package headless

import (
	"image"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
)

// paintThenCount renders one frame whose paint function runs paint, and counts
// the near-black pixels of the 360x60 area the test draws in.
func paintThenCount(t *testing.T, paint func(*gfx.Canvas), area gfx.Rect) int {
	t.Helper()
	config := app.Config{
		Title: "text order", Width: 400, Height: 100,
		ClearColor: app.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Paint:      func(canvas *gfx.Canvas, _ gfx.Size) { paint(canvas) },
	}
	session, err := Open(config, nil, Options{Scale: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	if err := session.Rest(); err != nil {
		t.Fatal(err)
	}
	pixels, err := session.Image()
	if err != nil {
		t.Fatal(err)
	}
	return countInk(pixels, area)
}

// countInk is how many pixels of area are near black: the glyphs, and not the
// area's white or red ground.
func countInk(pixels *image.RGBA, area gfx.Rect) int {
	ink := 0
	left, top := int(area.X), int(area.Y)
	right, bottom := int(area.X+area.Width), int(area.Y+area.Height)
	for y := top; y < bottom; y++ {
		for x := left; x < right; x++ {
			r, g, b, _ := pixels.At(x, y).RGBA()
			if r < 0x4000 && g < 0x4000 && b < 0x4000 {
				ink++
			}
		}
	}
	return ink
}

// The canvas paints in the order the application calls it: a rectangle declared
// after a label covers it, whatever kind of primitive each is.
func TestLaterOpaqueRectCoversEarlierText(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	area := gfx.Rect{X: 20, Y: 20, Width: 360, Height: 60}
	black := gfx.Color{Alpha: 1}
	red := gfx.Color{Red: 1, Alpha: 1}
	label := func(canvas *gfx.Canvas) {
		canvas.Label("MMMMMMMMMM", area, gfx.TextStyle{Size: 40, Color: black})
	}
	textOnly := paintThenCount(t, label, area)
	if textOnly == 0 {
		t.Fatal("the label painted nothing, so the order test cannot see it")
	}
	covered := paintThenCount(t, func(canvas *gfx.Canvas) {
		label(canvas)
		canvas.Fill(area, red)
	}, area)
	if covered != 0 {
		t.Fatalf("%d glyph pixels remain after a later opaque rectangle", covered)
	}
	// A label declared after the rectangle is above it, as before. Antialiasing
	// against the red ground may move a few fringe pixels across the threshold,
	// so compare the bulk of the ink, not an exact count.
	above := paintThenCount(t, func(canvas *gfx.Canvas) {
		canvas.Fill(area, red)
		label(canvas)
	}, area)
	if above*2 < textOnly {
		t.Fatalf("a label over the rectangle has %d glyph pixels, far below the %d of a label alone", above, textOnly)
	}
}

// Text is not the only label a renderer keeps apart from shapes: an image is
// placed in the paint order the same way.
func TestLaterOpaqueRectCoversEarlierImage(t *testing.T) {
	if !rendererAvailable {
		t.Skip("native screenshots need the platform headless renderer")
	}
	pixels := make([]byte, 16*16*4)
	for index := 3; index < len(pixels); index += 4 {
		pixels[index] = 255
	}
	picture := gfx.Bitmap{Pix: pixels, Width: 16, Height: 16, Stride: 16 * 4, ID: 1}
	area := gfx.Rect{X: 20, Y: 20, Width: 16, Height: 16}
	draw := func(canvas *gfx.Canvas) {
		canvas.DrawBitmapRegion(picture, gfx.Rect{Width: 16, Height: 16}, area)
	}
	pictureOnly := paintThenCount(t, draw, area)
	if pictureOnly == 0 {
		t.Fatal("the image painted nothing, so the order test cannot see it")
	}
	covered := paintThenCount(t, func(canvas *gfx.Canvas) {
		draw(canvas)
		canvas.Fill(area, gfx.Color{Red: 1, Alpha: 1})
	}, area)
	if covered != 0 {
		t.Fatalf("%d picture pixels remain after a later opaque rectangle", covered)
	}
}
