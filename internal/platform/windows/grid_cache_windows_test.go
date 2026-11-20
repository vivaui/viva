package windows

import (
	"bytes"
	"fmt"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// gridCacheLabel is a row whose runs exercise every part of the layout key:
// plain cells, a decorated run, a ligature run and a wide character.
func gridCacheLabel() platform.TextLabel {
	value := "ab-cd-界-ef"
	label := platform.TextLabel{Value: value, Mono: true, CellWidth: 8, Width: float32(len(value)) * 8, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	runs := []gfx.TextRun{
		{Column: 0, Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Alpha: 1}},
		{Column: 1, Bytes: 1, Cells: 1, Color: gfx.Color{Green: 1, Alpha: 1}, Underline: true},
		{Column: 2, Bytes: 1, Cells: 1, Color: gfx.Color{Blue: 1, Alpha: 1}},
		{Column: 3, Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Alpha: 1}, Strike: true},
		{Column: 4, Bytes: 1, Cells: 1, Color: gfx.Color{Green: 1, Alpha: 1}},
		{Column: 5, Bytes: 3, Cells: 2, Color: gfx.Color{Blue: 1, Alpha: 1}},
		{Column: 7, Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Alpha: 1}},
		{Column: 8, Bytes: 2, Cells: 2, Color: gfx.Color{Green: 1, Alpha: 1}, Italic: true},
	}
	label.Runs = runs
	return label
}

// TestGridLayoutCacheKeepsRendering is the layout cache's correctness check:
// a rasterizer that has already shaped other runs must render a row exactly as
// one that has not, so a key that misses an input cannot reuse a wrong layout.
func TestGridLayoutCacheKeepsRendering(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	warm, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer warm.close()
	cold, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer cold.close()

	// Render the target after a differently-shaped line on the warm rasterizer,
	// and as the first line on the cold one.
	first := platform.TextLabel{Value: "wx-界", Mono: true, CellWidth: 8, Width: 24, Height: 20, FontSize: 14, Color: platform.Color{Alpha: 1}}
	first.Runs = []gfx.TextRun{
		{Column: 0, Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Alpha: 1}},
		{Column: 1, Bytes: 1, Cells: 1, Color: gfx.Color{Green: 1, Alpha: 1}, Underline: true},
		{Column: 2, Bytes: 3, Cells: 2, Color: gfx.Color{Blue: 1, Alpha: 1}, Ligatures: true},
	}
	target := gridCacheLabel()
	warmPixels, warmLCD, err := warm.rasterize(first, 1, 80, 20, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	_ = warmPixels
	warmPixels, warmLCD, err = warm.rasterize(target, 1, 88, 20, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	coldPixels, coldLCD, err := cold.rasterize(target, 1, 88, 20, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if warmLCD != coldLCD {
		t.Fatalf("cached rasterization lcd=%v, fresh %v", warmLCD, coldLCD)
	}
	if !bytes.Equal(warmPixels, coldPixels) {
		t.Fatal("a rasterizer that shaped other runs painted this row differently")
	}
}

// BenchmarkGridRasterizeColors is one 100-cell terminal line in one color and
// in seventy, the density a colorizer gives a log line. The layout cache makes
// the run-per-color cost a map lookup after the first row of its alphabet.
func BenchmarkGridRasterizeColors(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		b.Fatal(err)
	}
	defer rasterizer.close()
	for _, runs := range []int{1, 70} {
		b.Run(fmt.Sprintf("runs-%d", runs), func(b *testing.B) {
			label := cacheBenchLine(100, runs)
			b.ReportAllocs()
			for range b.N {
				if _, _, err := rasterizer.rasterize(label, 1, 800, 20, true, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// cacheBenchLine is a 100-cell row split into runs colors, each a cell or more.
func cacheBenchLine(cells, runs int) platform.TextLabel {
	value := make([]byte, cells)
	for index := range value {
		value[index] = byte('a' + index%26)
	}
	label := platform.TextLabel{Value: string(value), Mono: true, CellWidth: 8, Width: float32(cells) * 8, Height: 20, FontSize: 14, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	per := cells / runs
	colors := []gfx.Color{{Red: 1, Green: 1, Blue: 1, Alpha: 1}, {Green: 1, Alpha: 1}, {Red: 1, Alpha: 1}, {Red: 0.2, Green: 0.4, Blue: 1, Alpha: 1}, {Red: 1, Green: 1, Alpha: 1}}
	for index := 0; index < runs; index++ {
		from := index * per
		to := from + per
		if index == runs-1 {
			to = cells
		}
		label.Runs = append(label.Runs, gfx.TextRun{Column: int32(from), Bytes: int32(to - from), Cells: int32(to - from), Color: colors[index%len(colors)]})
	}
	return label
}

// TestGDIBatchKeepsPixels is the batching's correctness check: a row drawn in one GDI
// call per brightness pass must have the very pixels of the same row drawn piece by
// piece, or a colorizer's line would shimmer every time it entered the viewport.
func TestGDIBatchKeepsPixels(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	label := cacheBenchLine(100, 70)
	batched, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer batched.close()
	piecewise, err := newDirectRasterizer()
	if err != nil {
		t.Fatal(err)
	}
	defer piecewise.close()
	piecewise.gdi = &gdiGridRasterizer{noBatch: true}
	got, lcd, err := batched.rasterize(label, 1, 800, 20, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	want, wantLCD, err := piecewise.rasterize(label, 1, 800, 20, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if lcd != wantLCD {
		t.Fatalf("batched rasterization lcd=%v, piecewise %v", lcd, wantLCD)
	}
	if !bytes.Equal(got, want) {
		different, maxDelta := 0, 0
		for index := range got {
			if delta := int(got[index]) - int(want[index]); delta != 0 {
				different++
				if delta < 0 {
					delta = -delta
				}
				maxDelta = max(maxDelta, delta)
			}
		}
		t.Fatalf("batched row differs from piecewise in %d bytes (max delta %d)", different, maxDelta)
	}
}
