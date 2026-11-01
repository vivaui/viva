package windows

import (
	"math"
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// Cells of different brightness are drawn in separate passes. The second pass must not wipe out
// what the first drew at the edge between two cells: a box-drawing line, whose glyphs run edge to
// edge, stays unbroken.
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
		// A cell is a whole number of device pixels wide (ui.Canvas.MonoCellSize).
		cellWidth = float32(math.Round(float64(cellWidth*scale))) / scale
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

// The color of a column is the color of the one run that owns it. Runs that shared an edge column
// drew its coverage twice, and a glyph that fills its cell, such as a powerline arrow, has full
// coverage there, so the second run's color covered the first as a line the height of the cell
// (t8y2/motty#279). Even a cell width that is not whole, which MonoCellSize no longer gives, must
// leave every column to one run.
func TestRunsOwnDisjointColumns(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	const cells = 12
	face := ui.MonoFamily("Consolas")
	colors := [3]ui.Color{{Red: 1, Alpha: 1}, {Red: 1, Green: 1, Blue: 1, Alpha: 1}, {Green: 1, Alpha: 1}}
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		for _, cellWidth := range []float32{8, 8.4, 9.6, 10.25} {
			runs := make([]ui.TextRun, cells)
			for i := range runs {
				runs[i] = ui.TextRun{Column: int32(i), Bytes: 3, Cells: 1, Color: colors[i%len(colors)]}
			}
			label := platform.TextLabel{
				Value: strings.Repeat("─", cells), Width: cells * cellWidth, Height: 20,
				FontSize: 16, CellWidth: cellWidth, Mono: true, MonoFace: face, Runs: runs,
			}
			width, height := int(cells*cellWidth*scale+1), int(20*scale)
			if _, _, err := rasterizer.rasterize(label, scale, width, height, true, 0); err != nil {
				t.Fatal(err)
			}
			spans := rasterizer.spans
			if len(spans) != cells {
				t.Fatalf("%vx cell %v: %d spans for %d runs of three colors", scale, cellWidth, len(spans), cells)
			}
			for i := 1; i < len(spans); i++ {
				if spans[i].left < spans[i-1].right {
					t.Fatalf("%vx cell %v: span %d starts at column %d, inside the span before it, which ends at %d", scale, cellWidth, i, spans[i].left, spans[i-1].right)
				}
			}
		}
	}
}
