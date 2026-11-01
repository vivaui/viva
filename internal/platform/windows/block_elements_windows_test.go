package windows

import (
	"runtime"
	"strings"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// A row of upper half blocks fills its cells edge to edge and leaves the lower half empty,
// in light text, dark text and a line that alternates the two, with and without ClearType,
// at the pitches display scales give: no seam where two cells meet, none where a dark cell
// follows a light one.
func TestBlockElementsFillTheirCellsWithoutSeams(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	light, dark := ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, ui.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
	const cells = 8
	for name, colorOf := range map[string]func(int) ui.Color{
		"light": func(int) ui.Color { return light },
		"dark":  func(int) ui.Color { return dark },
		"mixed": func(i int) ui.Color {
			if i%2 == 1 {
				return dark
			}
			return light
		},
	} {
		for _, scale := range []float32{1, 1.25, 1.5} {
			face := ui.MonoFamily("Consolas")
			cellWidth, lineHeight, err := MeasureMono(16, face)
			if err != nil {
				t.Fatal(err)
			}
			runs := make([]ui.TextRun, cells)
			for i := range runs {
				runs[i] = ui.TextRun{Column: int32(i), Bytes: 3, Cells: 1, Color: colorOf(i)}
			}
			label := platform.TextLabel{
				Value: strings.Repeat("▀", cells), Width: cells * cellWidth, Height: lineHeight,
				FontSize: 16, CellWidth: cellWidth, Mono: true, MonoFace: face, Runs: runs,
			}
			width, height := int(cells*cellWidth*scale+1), int(lineHeight*scale+1)
			filled := int(cells*cellWidth*scale + 0.5)
			for _, lcd := range []bool{true, false} {
				pixels, _, err := rasterizer.rasterize(label, scale, width, height, lcd, 0)
				if err != nil {
					t.Fatal(err)
				}
				mid := int(float32(height)*0.5 + 0.5)
				for row := 0; row < height; row++ {
					for x := 0; x < filled; x++ {
						alpha := pixels[(row*width+x)*4+3]
						if row < mid && alpha != 255 || row >= mid && alpha != 0 {
							t.Fatalf("%s at %vx, ClearType %v: pixel (%d,%d) has alpha %d, the upper half is rows below %d", name, scale, lcd, x, row, alpha, mid)
						}
					}
				}
			}
		}
	}
}
