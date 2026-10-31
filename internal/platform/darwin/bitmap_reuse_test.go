//go:build darwin && vivahead

package darwin

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestRetainedBitmapSurvivesSceneAndAnimationChanges(t *testing.T) {
	offscreen, err := NewOffscreen(64, 48, 2, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	bitmap := TextLabel{
		X: 4, Y: 4, Width: 20, Height: 20,
		Bitmap: []byte{255, 0, 255, 255}, BitmapWidth: 1, BitmapHeight: 1, BitmapStride: 4,
		BitmapID: 71, BitmapVersion: 1, BitmapUpload: true,
	}
	caption := TextLabel{Value: "checking", X: 0, Y: 32, Width: 64, Height: 16, FontSize: 10}
	render := func(t *testing.T, label TextLabel, reverse bool, x, y int, wantBlue, wantGreen, wantRed byte) {
		t.Helper()
		labels := []TextLabel{label, caption}
		if reverse {
			labels[0], labels[1] = labels[1], labels[0]
		}
		if err := offscreen.Set(nil, labels, nil); err != nil {
			t.Fatal(err)
		}
		pixels, width, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		blue, green, red, _ := pixelAt(pixels, width, x*2, y*2)
		if blue != wantBlue || green != wantGreen || red != wantRed {
			t.Fatalf("bitmap pixel at (%d,%d) = BGR %d,%d,%d, want %d,%d,%d",
				x, y, blue, green, red, wantBlue, wantGreen, wantRed)
		}
	}
	render(t, bitmap, false, 10, 10, 255, 0, 255)
	bitmap.BitmapUpload = false
	caption.Value = "confirm"
	t.Run("confirmation resizes the door", func(t *testing.T) {
		bitmap.Width, bitmap.Height = 16, 16
		render(t, bitmap, false, 10, 10, 255, 0, 255)
	})
	t.Run("flight moves scales and changes layer and index", func(t *testing.T) {
		bitmap.X, bitmap.Y, bitmap.Width, bitmap.Height, bitmap.Layer = 32, 8, 12, 12, 2
		render(t, bitmap, true, 38, 14, 255, 0, 255)
		render(t, bitmap, true, 10, 10, 0, 0, 0)
	})
	t.Run("clip changes without replacing the texture", func(t *testing.T) {
		bitmap.Clipped, bitmap.Clip = true, ui.Rect{X: 32, Y: 8, Width: 6, Height: 12}
		render(t, bitmap, true, 34, 14, 255, 0, 255)
		render(t, bitmap, true, 42, 14, 0, 0, 0)
		bitmap.Clipped = false
		render(t, bitmap, true, 42, 14, 255, 0, 255)
	})
	t.Run("new pixel version replaces the retained texture", func(t *testing.T) {
		bitmap.Bitmap, bitmap.BitmapVersion, bitmap.BitmapUpload = []byte{0, 255, 0, 255}, 2, true
		render(t, bitmap, true, 38, 14, 0, 255, 0)
		bitmap.Width, bitmap.Height, bitmap.BitmapUpload = 10, 10, false
		render(t, bitmap, false, 38, 14, 0, 255, 0)
	})
}

func TestBitmapDownsampleAveragesSourcePixels(t *testing.T) {
	offscreen, err := NewOffscreen(8, 8, 1, Color{Alpha: 1})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	label := TextLabel{
		X: 2, Y: 2, Width: 1, Height: 1,
		Bitmap: []byte{0, 0, 255, 255, 0, 255, 0, 255}, BitmapWidth: 2, BitmapHeight: 1, BitmapStride: 8,
		BitmapID: 72, BitmapVersion: 1, BitmapUpload: true,
	}
	if err := offscreen.Set(nil, []TextLabel{label}, nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	blue, green, red, alpha := pixelAt(pixels, width, 2, 2)
	if alpha < 240 || red < 90 || green < 90 || blue > 80 {
		t.Fatalf("downsampled pixel = BGRA %d,%d,%d,%d, want an averaged red/green pixel", blue, green, red, alpha)
	}
}
