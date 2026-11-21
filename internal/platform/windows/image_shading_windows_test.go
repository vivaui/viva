package windows

import (
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func imageShadedLabel(radius, fade float32) []platform.TextLabel {
	pixels := make([]byte, 16*16*4)
	for index := range pixels {
		pixels[index] = 255
	}
	return []platform.TextLabel{{
		X: 2, Y: 2, Width: 16, Height: 16,
		Bitmap: pixels, BitmapWidth: 16, BitmapHeight: 16, BitmapStride: 16 * 4, BitmapID: 1, BitmapUpload: true,
		ImageRadius: radius, ImageFade: fade,
	}}
}

// A rounded image leaves its corners to what is behind it, and a faded one is
// dimmer than an opaque one: the D3D11 text shader cuts and fades the picture.
func TestOffscreenRoundsAndFadesAnImage(t *testing.T) {
	square := renderScene(t, newOffscreen(t, 20, 20, 1), nil, imageShadedLabel(0, 0))
	round := renderScene(t, newOffscreen(t, 20, 20, 1), nil, imageShadedLabel(8, 0))
	faded := renderScene(t, newOffscreen(t, 20, 20, 1), nil, imageShadedLabel(0, 0.5))

	if !isPainted(square, 3, 3) {
		t.Error("a square image does not paint its corner")
	}
	if isPainted(round, 3, 3) {
		t.Error("the rounded corner still paints")
	}
	if !isPainted(round, 10, 10) {
		t.Error("the rounded image's center is not painted")
	}
	red := square.RGBAAt(10, 10).R
	fadedRed := faded.RGBAAt(10, 10).R
	if fadedRed >= red || fadedRed == 0 {
		t.Errorf("faded image red = %d, want about half of %d", fadedRed, red)
	}
}
