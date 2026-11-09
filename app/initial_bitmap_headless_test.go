//go:build darwin && vivahead

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/darwin"
)

func TestPreparedWindowUploadsInitialBitmapToMetal(test *testing.T) {
	bitmap := gfx.Bitmap{Pix: []byte{0, 0, 255, 255}, Width: 1, Height: 1, Stride: 4, ID: 9}
	caption := "estimated"
	config := Config{Width: 80, Height: 40, Paint: func(canvas *gfx.Canvas, _ gfx.Size) {
		canvas.DrawBitmap(bitmap, gfx.Rect{Width: 20, Height: 20})
		canvas.Label(caption, gfx.Rect{X: 20, Width: 60, Height: 20}, gfx.TextStyle{Size: 12})
	}}
	var canvas gfx.Canvas
	config.Paint(&canvas, gfx.Size{Width: 80, Height: 40})
	caption = "ready"
	native, cleanup, err := prepareNativeWindow(config, &canvas)
	if err != nil {
		test.Fatal(err)
	}
	defer cleanup()
	offscreen, err := darwin.NewOffscreen(80, 40, 1, platform.Color{Alpha: 1})
	if err != nil {
		test.Fatal(err)
	}
	defer offscreen.Close()
	if err := offscreen.Set(native.Rectangles, native.TextLabels, nil); err != nil {
		test.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		test.Fatal(err)
	}
	offset := (5*width + 5) * 4
	if pixels[offset] != 0 || pixels[offset+1] != 0 || pixels[offset+2] != 255 || pixels[offset+3] != 255 {
		test.Fatalf("initial bitmap pixels on Metal = %v, want opaque red", pixels[offset:offset+4])
	}
}
