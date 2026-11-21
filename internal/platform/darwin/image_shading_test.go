//go:build darwin && vivahead

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func imageLabel(radius, fade float32) []platform.TextLabel {
	pixels := make([]byte, 16*16*4)
	for index := range pixels {
		pixels[index] = 255 // opaque white BGRA
	}
	return []platform.TextLabel{{
		X: 2, Y: 2, Width: 16, Height: 16,
		Bitmap: pixels, BitmapWidth: 16, BitmapHeight: 16, BitmapStride: 16 * 4, BitmapID: 1, BitmapUpload: true,
		ImageRadius: radius, ImageFade: fade,
	}}
}

func TestOffscreenRoundsAndFadesAnImage(t *testing.T) {
	offscreen, err := NewOffscreen(20, 20, 1, Color{Alpha: 0})
	if err != nil {
		t.Skipf("no Metal device: %v", err)
	}
	defer offscreen.Close()
	if err := offscreen.Set(nil, imageLabel(8, 0), nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err := offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, alpha := pixelAt(pixels, width, 3, 3); alpha != 0 {
		t.Errorf("rounded corner alpha = %d, want 0", alpha)
	}
	if _, _, _, alpha := pixelAt(pixels, width, 10, 10); alpha < 250 {
		t.Errorf("rounded image center alpha = %d, want opaque", alpha)
	}

	if err := offscreen.Set(nil, imageLabel(0, 0.5), nil); err != nil {
		t.Fatal(err)
	}
	pixels, width, _, err = offscreen.Render()
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, alpha := pixelAt(pixels, width, 10, 10)
	if alpha < 120 || alpha > 136 {
		t.Errorf("half-faded image alpha = %d, want about 128", alpha)
	}
}
