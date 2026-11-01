package windows

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Neighbouring cells share the pixel column their edge falls in, and each draws its own part of
// it. In a line whose cells alternate between light and dark text, which are drawn in separate
// passes, the second pass must not wipe out what the first drew there: a box-drawing line, whose
// glyphs run edge to edge, stays unbroken.
func TestAlternatingColorsLeaveNoGapBetweenCells(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	light, dark := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, ui.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
	const cells = 8
	face := ui.MonoFamily("Consolas")
	for _, scale := range []float32{1, 1.25, 1.5} {
		cellWidth, lineHeight, err := MeasureMono(16, face)
		if err != nil {
			t.Fatal(err)
		}
		runs := make([]ui.TextRun, cells)
		for i := range runs {
			runs[i] = ui.TextRun{Column: int32(i), Bytes: 3, Cells: 1, Color: light}
			if i%2 == 1 {
				runs[i].Color = dark
			}
		}
		label := platform.TextLabel{
			Value: strings.Repeat("─", cells), Width: cells * cellWidth, Height: lineHeight,
			FontSize: 16, CellWidth: cellWidth, Mono: true, MonoFace: face, Runs: runs,
		}
		width, height := int(cells*cellWidth*scale+1), int(lineHeight*scale+1)
		filled := int(cells*cellWidth*scale - 1)
		pixels, _, err := rasterizer.rasterize(label, scale, width, height, true, 0)
		if err != nil {
			t.Fatal(err)
		}
		// The row of the line is the one with the most coverage.
		best, row := 0, 0
		for y := 0; y < height; y++ {
			sum := 0
			for x := 0; x < filled; x++ {
				sum += int(pixels[(y*width+x)*4+3])
			}
			if sum > best {
				best, row = sum, y
			}
		}
		for x := 0; x < filled; x++ {
			if alpha := pixels[(row*width+x)*4+3]; alpha < 128 {
				t.Fatalf("at %vx the line has a gap at column %d of row %d: alpha %d", scale, x, row, alpha)
			}
		}
	}
}
