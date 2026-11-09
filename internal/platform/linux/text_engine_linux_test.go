package linux

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textenginetest"
)

var _ gfx.StyledTextEngine = (*textEngine)(nil)

func TestItalicPangoLineHasSeparateCacheEntry(t *testing.T) {
	engine := newTestEngine(t)
	engine.mu.Lock()
	regular, regularErr := engine.lineStyleLocked("Italic 本文", 16, 0, false)
	italic, italicErr := engine.lineStyleLocked("Italic 本文", 16, 0, true)
	engine.mu.Unlock()
	if regularErr != nil || italicErr != nil || regular == italic {
		t.Fatalf("Pango regular/italic lines = %p/%p, errs=%v/%v", regular, italic, regularErr, italicErr)
	}
}

func newTestEngine(t *testing.T) *textEngine {
	t.Helper()
	engine, err := newTextEngine()
	if err != nil {
		t.Skipf("no Pango: %v", err)
	}
	t.Cleanup(engine.close)
	return engine
}

func TestEnginePassesTheTextEngineContract(t *testing.T) {
	textenginetest.Run(t, newTestEngine(t))
}

func TestTextEngineRasterizesProportionalText(t *testing.T) {
	engine := newTestEngine(t)
	bitmap, err := engine.RasterizeText("Privacy 中文", gfx.TextStyle{
		Size: 14, Weight: gfx.WeightSemibold, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}, 2, 240, 56)
	if err != nil {
		t.Fatal(err)
	}
	ink := false
	for index := 3; index < len(bitmap.Pix); index += 4 {
		ink = ink || bitmap.Pix[index] != 0
	}
	if bitmap.Width != 240 || bitmap.Height != 56 || bitmap.Stride != 960 || !ink {
		t.Fatalf("rasterized bitmap = %dx%d stride %d, ink=%v", bitmap.Width, bitmap.Height, bitmap.Stride, ink)
	}
}

// The Linux editor must measure what the label draws: a line's ink ends where its width
// says, with the same layout shaping both.
func TestAnEngineWidthIsWhereTheDrawnGlyphsEnd(t *testing.T) {
	engine := sharedTextEngine()
	if engine == nil {
		t.Skip("no Pango")
	}
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	for _, scale := range []float32{1, 2} {
		for _, value := range []string{"Hello world", "2213123中文", "192.168.1.1 主机", "office affine ffi"} {
			label := platform.TextLabel{Value: value, Width: 300, Height: 28, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
			bounds := rasterAlphaBoundsAt(t, services, label, scale)
			width := engine.Width(value, 14, 0) * scale
			// The ink lies inside the advance box to within a glyph's side bearing.
			if float64(bounds.maxX+1) > float64(width)+3 || float64(bounds.maxX+1) < float64(width)-6*float64(scale) {
				t.Errorf("%q at %gx: ink ends at %d, the engine's width is %v device pixels", value, scale, bounds.maxX+1, width)
			}
		}
	}
}

// A digit run is drawn the same whether or not another piece follows it (the squeeze that
// laid out 2213123 narrower before 中文 is gone): the columns the digits cover are
// identical in both bitmaps.
func TestADigitRunIsNotSqueezedByWhatFollowsIt(t *testing.T) {
	engine := sharedTextEngine()
	if engine == nil {
		t.Skip("no Pango")
	}
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	for _, scale := range []float32{1, 1.5, 2} {
		alone := platform.TextLabel{Value: "2213123", Width: 200, Height: 28, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
		followed := alone
		followed.Value = "2213123中文"
		width, height := int(math.Ceil(float64(alone.Width*scale))), int(math.Ceil(float64(alone.Height*scale)))
		aloneBits, _, err := services.rasterize(alone, scale, width, height, false)
		if err != nil {
			t.Fatal(err)
		}
		followedBits, _, err := services.rasterize(followed, scale, width, height, false)
		if err != nil {
			t.Fatal(err)
		}
		// Stay clear of the last digit's antialiasing against the first character.
		columns := int(engine.Width("2213123", 14, 0)*scale) - 2
		for row := range height {
			left := aloneBits[row*width*4 : (row*width+columns)*4]
			right := followedBits[row*width*4 : (row*width+columns)*4]
			if !bytes.Equal(left, right) {
				t.Fatalf("at %gx the digits differ when 中文 follows them (row %d)", scale, row)
			}
		}
	}
}

func BenchmarkEngine(b *testing.B) {
	engine, err := newTextEngine()
	if err != nil {
		b.Skipf("no Pango: %v", err)
	}
	defer engine.close()
	const line = "192.168.1.1 主机 Settings and preferences"
	distinct := make([]string, 4096) // more than the cache holds, so each is shaped
	for index := range distinct {
		distinct[index] = fmt.Sprintf("%s %d", line, index)
	}
	engine.Width(line, 14, 0)
	engine.Caret(line, 14, 0, 12)
	b.Run("WidthHit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			engine.Width(line, 14, 0)
		}
	})
	b.Run("CaretHit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			engine.Caret(line, 14, 0, 12)
		}
	})
	b.Run("OffsetAtHit", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			engine.OffsetAt(line, 14, 0, 60)
		}
	})
	b.Run("WidthMiss", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; b.Loop(); index++ {
			engine.Width(distinct[index%len(distinct)], 14, 0)
		}
	})
	b.Run("CaretMiss", func(b *testing.B) {
		b.ReportAllocs()
		for index := 0; b.Loop(); index++ {
			engine.Caret(distinct[index%len(distinct)], 14, 0, 12)
		}
	})
}
