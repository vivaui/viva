//go:build darwin && vivahead

package darwin

import (
	"bytes"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

var _ gfx.StyledTextEngine = (*TextEngine)(nil)

func TestItalicAndRegularLinesHaveSeparateCoreTextCacheEntries(t *testing.T) {
	engine := SharedTextEngine()
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	regular := engine.lineStyle("墨栈 Italic", 20, gfx.WeightRegular, false)
	italic := engine.lineStyle("墨栈 Italic", 20, gfx.WeightRegular, true)
	if regular == nil || italic == nil || regular == italic {
		t.Fatal("italic and regular shared a shaped CoreText line")
	}
}

func TestItalicProportionalLabelDrawsDifferentGlyphPixels(t *testing.T) {
	color := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	render := func(italic bool) []byte {
		offscreen, err := NewOffscreen(280, 50, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		err = offscreen.Set(nil, []TextLabel{{
			Value: "Italic 本文", X: 0, Y: 0, Width: 280, Height: 50,
			FontSize: 24, Weight: gfx.WeightRegular, Italic: italic, Color: color,
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		pixels, _, _, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return pixels
	}
	if bytes.Equal(render(false), render(true)) {
		t.Fatal("proportional italic label has regular glyph pixels")
	}
}

// leanOf is how far the ink of a picture leans: the horizontal distance between the center
// of mass of its top half and of its bottom half, in pixels. An upright line has none and a
// line slanted to the right has the top half further right.
func leanOf(pixels []byte, width, height int) float64 {
	var top, bottom, topMass, bottomMass float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			at := (y*width + x) * 4
			weight := float64(pixels[at]) + float64(pixels[at+1]) + float64(pixels[at+2]) // white ink on black
			if y < height/2 {
				top += weight * float64(x)
				topMass += weight
			} else {
				bottom += weight * float64(x)
				bottomMass += weight
			}
		}
	}
	if topMass == 0 || bottomMass == 0 {
		return 0
	}
	return top/topMass - bottom/bottomMass
}

// A CJK font has no italic face, so Core Text leaves its glyphs upright; the label is
// drawn with a slant for them, as it is for the fallback of a Latin font.
func TestItalicLabelSlantsGlyphsOfFontsWithoutAnItalicFace(t *testing.T) {
	color := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	lean := func(text string, italic bool) float64 {
		offscreen, err := NewOffscreen(280, 50, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		err = offscreen.Set(nil, []TextLabel{{
			Value: text, X: 0, Y: 0, Width: 280, Height: 50,
			FontSize: 24, Weight: gfx.WeightRegular, Italic: italic, Color: color,
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		pixels, width, height, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return leanOf(pixels, width, height)
	}
	for _, text := range []string{"本文强调", "emphasis", "强调 emphasis"} {
		upright, slanted := lean(text, false), lean(text, true)
		if slanted < upright+0.5 {
			t.Errorf("%q: italic leans %.2f px, upright %.2f; want the italic to lean right", text, slanted, upright)
		}
	}
}

// A fixed-pitch grid run carries its own slant, as a code editor's comment does: the
// same cells drawn with a slanted run must lean further right than the upright line.
// The run flag word is what the native renderer reads.
func TestItalicGridRunSlantsItsCells(t *testing.T) {
	if got := textRunFlags(gfx.TextRun{Italic: true}); got&runItalic == 0 {
		t.Fatalf("flags = %d, want the italic bit", got)
	}
	if got := textRunFlags(gfx.TextRun{Underline: true}); got&runItalic != 0 {
		t.Fatalf("an upright run packed the italic bit: %d", got)
	}
	color := platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	const (
		value = "comment text"
		cell  = float32(12)
	)
	render := func(italic bool) ([]byte, int, int) {
		offscreen, err := NewOffscreen(240, 40, 2, Color{Alpha: 1})
		if err != nil {
			t.Skipf("no Metal device: %v", err)
		}
		defer offscreen.Close()
		runs := []gfx.TextRun{{
			Bytes: int32(len(value)), Cells: int32(len(value)),
			Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, Italic: italic,
		}}
		err = offscreen.Set(nil, []TextLabel{{
			Value: value, X: 0, Y: 4, Width: 240, Height: 30, FontSize: 20,
			Color: color, Mono: true, CellWidth: cell, Runs: runs,
		}}, nil)
		if err != nil {
			t.Fatal(err)
		}
		pixels, width, height, err := offscreen.Render()
		if err != nil {
			t.Fatal(err)
		}
		return pixels, width, height
	}
	upright, width, height := render(false)
	slanted, _, _ := render(true)
	if bytes.Equal(upright, slanted) {
		t.Fatal("an italic grid run drew the upright glyphs")
	}
	if up, slant := leanOf(upright, width, height), leanOf(slanted, width, height); slant < up+0.5 {
		t.Errorf("italic grid run leans %.2f px, upright %.2f; want the italic to lean right", slant, up)
	}
}
