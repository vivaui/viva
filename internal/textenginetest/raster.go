package textenginetest

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// TransparentColorGlyphs checks that a color font obeys the same zero opacity
// as the ordinary glyph next to it.
func TransparentColorGlyphs(t *testing.T, engine gfx.TextRasterizer) {
	t.Helper()
	for _, alpha := range []float32{1, 0} {
		style := gfx.TextStyle{Size: 24, Color: gfx.Color{Alpha: alpha}}
		bitmap, err := engine.RasterizeText("A😷🥺", style, 1, 160, 48)
		if err != nil {
			t.Fatal(err)
		}
		ink := 0
		for i := 3; i < len(bitmap.Pix); i += 4 {
			if bitmap.Pix[i] != 0 {
				ink++
			}
		}
		if (alpha == 0) != (ink == 0) {
			t.Errorf("foreground alpha %g produced %d painted pixels", alpha, ink)
		}
	}
}
