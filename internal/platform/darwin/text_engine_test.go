//go:build darwin && !ios

package darwin

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/textenginetest"
)

func TestTextEnginePassesTheContract(t *testing.T) {
	textenginetest.Run(t, SharedTextEngine())
}

func TestCoreTextHitRetainsTheMixedDirectionEdge(t *testing.T) {
	const prefix, first, second, suffix = "a\U00010437 ", "\u05d0\u05b7", "\u05d1", " cd"
	const value, size = prefix + first + second + suffix, 20
	engine := SharedTextEngine()
	var canvas gfx.Canvas
	canvas.SetTextEngine(engine)
	span := func(from, to int) gfx.TextSpan {
		t.Helper()
		spans := engine.Selection(nil, value, size, 0, from, to)
		if len(spans) != 1 || spans[0].Width <= 0 {
			t.Fatalf("range [%d,%d) spans %v", from, to, spans)
		}
		return spans[0]
	}
	start, middle, end := len(prefix), len(prefix+first), len(prefix+first+second)
	left := span(start-1, start)
	right := span(start, middle)
	for _, check := range []struct{ x, edge float32 }{
		{left.X + left.Width*0.9, left.X + left.Width},
		{right.X + right.Width*0.9, right.X + right.Width},
	} {
		hit := canvas.TextHitAt(value, size, 0, check.x)
		if hit.Offset != start || hit.X-check.edge > 0.02 || check.edge-hit.X > 0.02 {
			t.Errorf("clicked %g: hit %+v; want byte %d at %g (RTL ends at byte %d)", check.x, hit, start, check.edge, end)
		}
	}
}

func TestTextEngineRasterizesProportionalText(t *testing.T) {
	engine := SharedTextEngine()
	textenginetest.TransparentColorGlyphs(t, engine)
	bitmap, err := engine.RasterizeText("Privacy 中文", gfx.TextStyle{
		Size: 14, Weight: gfx.WeightSemibold, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
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
	style := gfx.TextStyle{Size: 14, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}

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
		style  gfx.TextStyle
		scale  float32
		width  int
		height int
	}{
		{"zero width", style, 2, 0, 20},
		{"zero height", style, 2, 64, 0},
		{"negative width", style, 2, -8, 20},
		{"zero scale", style, 0, 64, 20},
		{"negative scale", style, -1, 64, 20},
		{"zero font size", gfx.TextStyle{}, 2, 64, 20},
		{"oversized", style, 2, 16385, 20},
	} {
		bitmap, err := engine.RasterizeText("x", testcase.style, testcase.scale, testcase.width, testcase.height)
		if err == nil {
			t.Errorf("%s: no error, got %+v", testcase.name, bitmap)
		}
	}
}

func bitmapHasAlpha(bitmap gfx.Bitmap) bool {
	for index := 3; index < len(bitmap.Pix); index += 4 {
		if bitmap.Pix[index] != 0 {
			return true
		}
	}
	return false
}
