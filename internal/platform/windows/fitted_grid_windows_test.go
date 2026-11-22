package windows

import (
	"bytes"
	"math"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestFittedDirectWriteCaretsKeepBothDirections(t *testing.T) {
	const arabic = "\u0633\u0644\u0627\u0645"
	const prefix = "a\u0301\U00010437 "
	const span = 108
	var canvas gfx.Canvas
	canvas.SetTextEngine(TextEngine())
	style := gfx.TextStyle{Size: 20, Mono: true}
	carets, ok := canvas.FittedBidiMonoCarets(nil, prefix+arabic+" z", style, span)
	if !ok || len(carets) != 10 {
		t.Fatalf("mixed layout did not return ten grapheme boundaries: %+v, %t", carets, ok)
	}
	start, end := carets[3], carets[7]
	if start.Offset != len(prefix) || end.Offset != len(prefix+arabic) || start.Leading <= start.Trailing || end.Leading <= end.Trailing {
		t.Errorf("mixed layout lost a visual edge: start %+v, end %+v", start, end)
	}
	carets, ok = canvas.FittedBidiMonoCarets(carets[:0], arabic, style, span)
	if !ok || len(carets) != 5 {
		t.Fatalf("RTL layout lost Arabic letter boundaries: %+v, %t", carets, ok)
	}
	if math.Abs(float64(carets[0].Leading-span)) > 0.5 || math.Abs(float64(carets[4].Trailing)) > 0.5 {
		t.Errorf("RTL carets did not fill the fitted span: %+v", carets)
	}
	for i := 0; i < 4; i++ {
		if carets[i].Leading <= carets[i+1].Trailing {
			t.Errorf("Arabic grapheme %d did not progress to the left: %+v", i, carets)
		}
	}
	const padded = arabic + "  "
	carets, ok = canvas.FittedBidiMonoCarets(carets[:0], padded, style, span)
	if !ok || len(carets) != 7 {
		t.Fatalf("RTL layout lost trailing space boundaries: %+v, %t", carets, ok)
	}
	if math.Abs(float64(carets[0].Leading-span)) > 0.5 || math.Abs(float64(carets[6].Trailing)) > 0.5 || carets[4].Leading <= carets[5].Trailing {
		t.Errorf("trailing RTL spaces did not occupy the left of the fitted span: %+v", carets)
	}
}

func TestFittedDirectWriteItalicKeepsTheUprightLayout(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer rasterizer.close()
	const value = "ab \u0633\u0644\u0627\u0645 z"
	label := platform.TextLabel{Value: value, Width: 144, Height: 40, FontSize: 22, Mono: true, CellWidth: 12,
		Runs: []gfx.TextRun{{Bytes: int32(len(value)), Cells: 9, Color: gfx.RGB(255, 255, 255), Ligatures: true}}}
	render := func(italic bool) []byte {
		t.Helper()
		label.Runs[0].Italic = italic
		pix, _, err := rasterizer.rasterize(label, 1, 144, 40, false, 0)
		if err != nil {
			t.Fatal(err)
		}
		for i := 3; i < len(pix); i += 4 {
			if pix[i] != 0 {
				return bytes.Clone(pix)
			}
		}
		t.Fatalf("italic=%t produced an empty label", italic)
		return nil
	}
	if bytes.Equal(render(false), render(true)) {
		t.Fatal("fitted italic run drew the same pixels as upright text")
	}
	if len(rasterizer.shaped) != 1 {
		t.Errorf("italic pixels used a separate shaped layout: %d layouts", len(rasterizer.shaped))
	}
}
