package windows

// A terminal's fixed-pitch text is rasterized by GDI's ClearType, which fits the stems
// of a glyph to the pixel grid, where every other label is rasterized by Direct2D's
// natural rendering, which does not. The two are the same glyphs at the same size, so
// the tests here pin the choice: a line GDI can draw for a fixed-pitch font is drawn by
// GDI, and a line it cannot (ligatures, characters the font may lack) is left to
// Direct2D unchanged.

import (
	"bytes"
	"image"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

// renderHinted draws one label with GDI's ClearType for fixed-pitch text on or off.
func renderHinted(t *testing.T, label platform.TextLabel, hinted bool) *image.RGBA {
	t.Helper()
	previous := HintedGrid
	HintedGrid = hinted
	defer func() { HintedGrid = previous }()
	offscreen := newOffscreen(t, int(label.Width), int(label.Height), 1)
	return renderScene(t, offscreen, nil, []platform.TextLabel{label})
}

// inkOf is the summed coverage of a drawn line: how much text it holds.
func inkOf(pixels *image.RGBA) int {
	total := 0
	for index := 0; index < len(pixels.Pix); index += 4 {
		total += int(pixels.Pix[index]) + int(pixels.Pix[index+1]) + int(pixels.Pix[index+2])
	}
	return total
}

// gridLabel is a line as a terminal lays it out: one run of one color, each character on
// a cell of the grid.
func gridLabel(value string, ligatures bool) platform.TextLabel {
	return platform.TextLabel{
		Value: value, Width: float32(len(value)*7 + 2), Height: 16, FontSize: 13, CellWidth: 7,
		Mono: true, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		Runs: []ui.TextRun{{
			Bytes: int32(len(value)), Cells: int32(len(value)), Ligatures: ligatures,
			Color: ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
		}},
	}
}

// TestFixedPitchTextIsHinted guards that a fixed-pitch line of printable ASCII reaches
// GDI: its stems are fitted to the pixel grid, which shows as other pixels than
// Direct2D's natural rendering of the same cells, and no less ink.
func TestFixedPitchTextIsHinted(t *testing.T) {
	label := gridLabel("root@host:~# ls -l /root", false)
	natural := renderHinted(t, label, false)
	hinted := renderHinted(t, label, true)
	if bytes.Equal(natural.Pix, hinted.Pix) {
		t.Fatal("the hinted renderer drew the same pixels as the natural one")
	}
	if ink, plain := inkOf(hinted), inkOf(natural); ink < plain*9/10 {
		t.Errorf("the hinted line holds %d of ink against %d: it lost strokes", ink, plain)
	}
}

// TestFixedPitchHintingKeepsShapedLines guards the fallback: a run that may ligate is
// shaped by Direct2D alone, so hinting it would change nothing.
func TestFixedPitchHintingKeepsShapedLines(t *testing.T) {
	label := gridLabel("fi -> => != <=", true)
	if !bytes.Equal(renderHinted(t, label, false).Pix, renderHinted(t, label, true).Pix) {
		t.Fatal("a ligature run was drawn by two renderers")
	}
}

// TestFixedPitchHintingKeepsCharactersTheFontLacks guards the other fallback: GDI has no
// shaper and no fallback chain of ours, so a line with CJK stays with Direct2D.
func TestFixedPitchHintingKeepsCharactersTheFontLacks(t *testing.T) {
	label := gridLabel("Motty 已连接", false)
	if !bytes.Equal(renderHinted(t, label, false).Pix, renderHinted(t, label, true).Pix) {
		t.Fatal("a line with CJK was drawn by two renderers")
	}
}
