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

func bitmapHasAlpha(bitmap ui.Bitmap) bool {
	for index := 3; index < len(bitmap.Pix); index += 4 {
		if bitmap.Pix[index] != 0 {
			return true
		}
	}
	return false
}
