package linux

import (
	"bytes"
	_ "embed"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// testIconsFont is a fixed-pitch font of 600 units whose private-use glyphs have ink from 120 to
// 880, so that a glyph reaches 280 units past its cell, as a Nerd Font icon does.
//
//go:embed testdata/VivaTestIcons-Regular.ttf
var testIconsFont []byte

const testIcon = "\ue0b0"

func registerTestIcons(t *testing.T) gfx.MonoFace {
	t.Helper()
	engine := sharedTextEngine()
	if engine == nil {
		t.Skip("no text engine")
	}
	family, unregister, err := engine.RegisterFont(testIconsFont)
	if err != nil {
		t.Skipf("register the test font: %v", err)
	}
	t.Cleanup(unregister)
	return gfx.MonoFamily(family)
}

// gridLine rasterizes runs with cell pixels per cell and returns the coverage of each column (the
// most any row has), the color of the pixel with the most coverage in a column, and the spans,
// which are what the renderer paints when the line is LCD text (the machine's font settings
// decide; without them the runs are drawn in their own colors and there are no spans).
type gridResult struct {
	coverage []byte
	pixels   []byte
	width    int
	height   int
	lcd      bool
	spans    []colorSpan
}

func gridLine(t *testing.T, services *textServices, face gfx.MonoFace, size, cell float32, value string, runs []gfx.TextRun, cells int) gridResult {
	t.Helper()
	width, height := int(cell)*cells, int(size*1.5)
	label := platform.TextLabel{
		Value: value, Width: float32(width), Height: float32(height), FontSize: size, CellWidth: cell,
		Mono: true, MonoFace: face, Runs: runs,
	}
	pixels, lcd, err := services.rasterize(label, 1, width, height, true)
	if err != nil {
		t.Fatal(err)
	}
	result := gridResult{coverage: make([]byte, width), pixels: pixels, width: width, height: height, lcd: lcd, spans: append([]colorSpan(nil), services.spans...)}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			result.coverage[x] = max(result.coverage[x], pixels[(y*width+x)*4+3])
		}
	}
	return result
}

// white reports whether a column's strongest pixel is the color white: the run that owns the
// column drew it. The pixels are premultiplied BGRA, or in LCD coverage the owner's span says it.
func (result gridResult) white(column int) bool {
	if result.lcd {
		for _, span := range result.spans {
			if column >= span.left && column < span.right {
				return span.color[0] == 1 && span.color[1] == 1 && span.color[2] == 1
			}
		}
		return false
	}
	best := 0
	for y := 0; y < result.height; y++ {
		if result.pixels[(y*result.width+column)*4+3] > result.pixels[(best*result.width+column)*4+3] {
			best = y
		}
	}
	pixel := result.pixels[(best*result.width+column)*4:]
	return pixel[3] > 0 && pixel[0] == pixel[3] && pixel[1] == pixel[3] && pixel[2] == pixel[3]
}

// An icon is drawn with the advance of one cell and ink that goes past it. Where the cell after
// it is blank the ink shows, in the icon's color (t8y2/motty#503).
func TestIconShowsWholeOverTheBlankCellAfterIt(t *testing.T) {
	face := registerTestIcons(t)
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	const size, cell = 20, 10
	runs := []gfx.TextRun{
		{Bytes: 3, Cells: 1, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}},
		{Column: 2, Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Green: 0.5, Alpha: 1}},
	}
	line := gridLine(t, services, face, size, cell, testIcon+"x", runs, 6)
	// The glyph is 0.76 of the size wide, 15.2 px, centered in a cell of 10 on its 12 px advance:
	// its ink is in column 12, two pixels into the blank cell.
	const probe = cell + 2
	if line.coverage[probe] == 0 {
		t.Errorf("no ink at column %d, in the blank cell after the icon: the glyph is cut at its cell", probe)
	}
	if !line.white(probe) {
		t.Errorf("column %d is not painted in the icon's own color (LCD %v)", probe, line.lcd)
	}
	if line.white(cell * 2) {
		t.Errorf("the text after the blank cell is painted in the icon's color (LCD %v)", line.lcd)
	}
}

// A glyph's ink never lands in the columns of a character beside it.
func TestIconInkStaysOutOfTheNeighborsColumns(t *testing.T) {
	face := registerTestIcons(t)
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	const size, cell = 20, 10
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	beside := []gfx.TextRun{{Bytes: 3, Cells: 1, Color: white}, {Column: 1, Bytes: 2, Cells: 2, Color: white}}
	alone := []gfx.TextRun{{Column: 1, Bytes: 2, Cells: 2, Color: white}}
	with := gridLine(t, services, face, size, cell, testIcon+"ab", beside, 5)
	without := gridLine(t, services, face, size, cell, "ab", alone, 5)
	if !bytes.Equal(with.coverage[cell:], without.coverage[cell:]) {
		t.Errorf("the columns of the text beside the icon differ from the text drawn alone:\nwith the icon %v\nalone         %v", with.coverage[cell:], without.coverage[cell:])
	}
}
