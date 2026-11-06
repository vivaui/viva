//go:build darwin

package darwin

import (
	"testing"

	"github.com/vivaui/viva/internal/textenginetest"
	"github.com/vivaui/viva/ui"
)

func TestTextEnginePassesTheContract(t *testing.T) {
	textenginetest.Run(t, SharedTextEngine())
}

func TestTextEngineRasterizesProportionalText(t *testing.T) {
	engine := SharedTextEngine()
	bitmap, err := engine.RasterizeText("Privacy 中文", ui.TextStyle{
		Size: 14, Weight: ui.WeightSemibold, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}, 2, 240, 56)
	if err != nil {
		t.Fatal(err)
	}
	if bitmap.Width != 240 || bitmap.Height != 56 || bitmap.Stride != 960 || !bitmapHasAlpha(bitmap) {
		t.Fatalf("rasterized bitmap = %dx%d stride %d, ink=%v", bitmap.Width, bitmap.Height, bitmap.Stride, bitmapHasAlpha(bitmap))
	}
}

// An empty line is a transparent bitmap of the size asked for, and invalid
// geometry is an error: the same answers the other backends give, so a caller
// does not have to know which platform it is drawing on.
func TestTextEngineRasterizeContractMatchesTheOtherBackends(t *testing.T) {
	engine := SharedTextEngine()
	style := ui.TextStyle{Size: 14, Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}

	empty, err := engine.RasterizeText("", style, 2, 64, 20)
	if err != nil {
		t.Fatalf("empty line: %v", err)
	}
	if empty.Width != 64 || empty.Height != 20 || empty.Stride != 256 || len(empty.Pix) != 64*20*4 {
		t.Fatalf("empty line bitmap = %dx%d stride %d len %d", empty.Width, empty.Height, empty.Stride, len(empty.Pix))
	}
	if bitmapHasAlpha(empty) {
		t.Fatal("an empty line drew ink")
	}

	for _, testcase := range []struct {
		name   string
		style  ui.TextStyle
		scale  float32
		width  int
		height int
	}{
		{"zero width", style, 2, 0, 20},
		{"zero height", style, 2, 64, 0},
		{"negative width", style, 2, -8, 20},
		{"zero scale", style, 0, 64, 20},
		{"negative scale", style, -1, 64, 20},
		{"zero font size", ui.TextStyle{}, 2, 64, 20},
		{"oversized", style, 2, 16385, 20},
	} {
		bitmap, err := engine.RasterizeText("x", testcase.style, testcase.scale, testcase.width, testcase.height)
		if err == nil {
			t.Errorf("%s: no error, got %+v", testcase.name, bitmap)
		}
	}
}

func bitmapHasAlpha(bitmap ui.Bitmap) bool {
	for index := 3; index < len(bitmap.Pix); index += 4 {
		if bitmap.Pix[index] != 0 {
			return true
		}
	}
	return false
}
