package windows

import (
	"bytes"
	_ "embed"
	"runtime"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

// testIconFamily is testdata/VivaTestIcons-Regular.ttf: a fixed-pitch font of 600 units whose
// private-use glyphs have ink from 120 to 880, so that a glyph reaches 280 units (0.28 of its
// size) past its cell, as a Nerd Font icon does.
const (
	testIcon       = "\ue0b0"
	testIconInkMin = 120.0 / 1000
	testIconInkMax = 880.0 / 1000
	testIconCell   = 600.0 / 1000
)

//go:embed testdata/VivaTestIcons-Regular.ttf
var testIconsFont []byte

func registerTestIcons(t *testing.T) gfx.MonoFace {
	t.Helper()
	data := testIconsFont
	engine, err := theTextEngine()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	family, unregister, err := engine.RegisterFont(data)
	if err != nil {
		t.Fatalf("register the test font: %v", err)
	}
	t.Cleanup(unregister)
	return gfx.MonoFamily(family)
}

func newTestRasterizer(t *testing.T) *directRasterizer {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	t.Cleanup(rasterizer.close)
	return rasterizer
}

// The reach measured from the picture of a glyph is the font's outline, give or take the pixel
// the ClearType filter adds on each side.
func TestMeasuredInkIsTheFontsOutline(t *testing.T) {
	face := registerTestIcons(t)
	rasterizer := newTestRasterizer(t)
	const size = 40
	span := float32(testIconCell * size)
	reach, err := rasterizer.measureInk(inkKey{family: gfx.MonoFamilyName(face), size: size, span: span, text: testIcon})
	if err != nil {
		t.Fatal(err)
	}
	// The outline goes from 0.12 to 0.88 of the size, 0.28 past the cell that ends at 0.6. The font
	// is a test font: its table of side bearings does not agree with its outline, so the glyph is
	// drawn anywhere between the outline's place and one that starts at the cell's edge, and the
	// ink goes at least as far past the cell as its width does. The filter adds a pixel each way.
	if reach.Left > 2 {
		t.Errorf("left reach = %v px, but the ink starts inside the cell", reach.Left)
	}
	least := (testIconInkMax - testIconInkMin - testIconCell) * size
	most := (testIconInkMax-testIconCell)*size + 2
	if got := float64(reach.Right); got < least || got > most {
		t.Errorf("right reach = %v px, want between %.1f and %.1f", got, least, most)
	}
}

// gridLine rasterizes runs on a grid of cell pixels per cell and returns the coverage of each
// column (the most any row has) and the spans, which are what the renderer paints.
func gridLine(t *testing.T, rasterizer *directRasterizer, face gfx.MonoFace, size, cell float32, value string, runs []gfx.TextRun, cells int) ([]byte, []colorSpan) {
	t.Helper()
	width, height := int(cell)*cells, int(size*1.5)
	label := platform.TextLabel{
		Value: value, Width: float32(width), Height: float32(height), FontSize: size, CellWidth: cell,
		Mono: true, MonoFace: face, Runs: runs,
	}
	pixels, lcd, err := rasterizer.rasterize(label, 1, width, height, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !lcd {
		t.Fatal("the line was not drawn as ClearType coverage")
	}
	column := make([]byte, width)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			column[x] = max(column[x], pixels[(y*width+x)*4+3])
		}
	}
	return column, append([]colorSpan(nil), rasterizer.spans...)
}

func spanColorAt(spans []colorSpan, column int) (d2dColor, bool) {
	for _, span := range spans {
		if column >= span.left && column < span.right {
			return span.color, true
		}
	}
	return d2dColor{}, false
}

// An icon is drawn with the advance of one cell and ink that goes past it. Where the cell
// after it is blank the ink shows, in the icon's color (t8y2/motty#503).
func TestIconShowsWholeOverTheBlankCellAfterIt(t *testing.T) {
	face := registerTestIcons(t)
	rasterizer := newTestRasterizer(t)
	const size, cell = 20, 10
	light, other := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}, gfx.Color{Red: 1, Green: 0.5, Alpha: 1}
	runs := []gfx.TextRun{{Bytes: 3, Cells: 1, Color: light}, {Column: 2, Bytes: 1, Cells: 1, Color: other}}
	column, spans := gridLine(t, rasterizer, face, size, cell, testIcon+"x", runs, 6)
	// The glyph is 0.76 of the size wide, 15.2 px, centered in a cell of 10 on its 12 px advance:
	// its ink is in column 12, two pixels into the blank cell, whichever side bearing it is
	// drawn with.
	const probe = cell + 2
	if column[probe] == 0 {
		t.Errorf("no ink at column %d, in the blank cell after the icon: the glyph is cut at its cell", probe)
	}
	if color, ok := spanColorAt(spans, probe); !ok || color.R != 1 || color.G != 1 || color.B != 1 {
		t.Errorf("column %d is painted %+v (owned %v), want the icon's own color", probe, color, ok)
	}
	if color, ok := spanColorAt(spans, cell*2); !ok || color.G != 0.5 {
		t.Errorf("the text after the blank cell is painted %+v (owned %v), want its own color", color, ok)
	}
}

// A glyph's ink never lands in the columns of a character beside it, whatever the colors:
// they would be read as that run's coverage and painted in its color, a line next to an arrow.
func TestIconInkStaysOutOfTheNeighborsColumns(t *testing.T) {
	face := registerTestIcons(t)
	rasterizer := newTestRasterizer(t)
	const size, cell = 20, 10
	white := gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	// Both light, so that they are drawn in one pass, the case the shared pass reads as one.
	beside := []gfx.TextRun{{Bytes: 3, Cells: 1, Color: white}, {Column: 1, Bytes: 2, Cells: 2, Color: white, Ligatures: true}}
	alone := []gfx.TextRun{{Column: 1, Bytes: 2, Cells: 2, Color: white, Ligatures: true}}
	with, _ := gridLine(t, rasterizer, face, size, cell, testIcon+"ab", beside, 5)
	without, _ := gridLine(t, rasterizer, face, size, cell, "ab", alone, 5)
	if !bytes.Equal(with[cell:], without[cell:]) {
		t.Errorf("the columns of the text beside the icon differ from the text drawn alone:\nwith the icon %v\nalone         %v", with[cell:], without[cell:])
	}
	if with[cell-1] == 0 {
		t.Errorf("the icon has no ink in the last column of its own cell")
	}
}

// BenchmarkPromptWithIcons is a terminal line of 100 cells with six icons, each followed by a
// blank, in four colors rasterized as ClearType again and again: the steady state of a prompt
// that stays on the screen, where the reach of each icon is known and only the columns are
// planned, and the first call of each size, where it is measured.
func BenchmarkPromptWithIcons(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	engine, err := theTextEngine()
	if err != nil {
		b.Skipf("no DirectWrite: %v", err)
	}
	family, unregister, err := engine.RegisterFont(testIconsFont)
	if err != nil {
		b.Skipf("register the test font: %v", err)
	}
	defer unregister()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		b.Fatal(err)
	}
	defer rasterizer.close()
	var value []byte
	var runs []gfx.TextRun
	column := 0
	for index := 0; index < 6; index++ {
		color := terminalColors[index%len(terminalColors)]
		text := "user@host-" + string(rune('a'+index)) + " "
		runs = append(runs, gfx.TextRun{Column: int32(column), Bytes: int32(len(text)), Cells: int32(len(text)), Color: color})
		value = append(value, text...)
		column += len(text)
		runs = append(runs, gfx.TextRun{Column: int32(column), Bytes: int32(len(testIcon)), Cells: 1, Color: color})
		value = append(value, testIcon...)
		column++
	}
	label := platform.TextLabel{
		Value: string(value), Width: 8 * 100, Height: 20, FontSize: 13, CellWidth: 8,
		Mono: true, MonoFace: gfx.MonoFamily(family), Runs: runs,
	}
	b.ReportAllocs()
	for b.Loop() {
		if _, _, err := rasterizer.rasterize(label, 1, 800, 20, true, 0); err != nil {
			b.Fatal(err)
		}
	}
}
