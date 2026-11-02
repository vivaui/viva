package windows

// A fixed-pitch run of printable ASCII is rasterized by GDI's ClearType, which fits the
// stems of a glyph to the pixel grid, where every other label is rasterized by Direct2D's
// natural rendering, which does not. The two are the same glyphs at the same size, so the
// tests here pin what must hold whichever draws a run: every character on its cell, the
// decorations of a run, the brightness of its text; and the choice itself: ASCII runs are
// GDI's, and ligatures, characters the font lacks and fonts GDI does not have stay with
// Direct2D, even in a line that holds both.

import (
	"bytes"
	"fmt"
	"math"
	"runtime"
	"testing"

	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/ui"
)

var (
	termWhite = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	termInk   = ui.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}
)

// gridRender is the coverage a rasterizer made of a label: BGRA, alpha their luminance.
type gridRender struct {
	pixels        []byte
	width, height int
}

// rasterizeGrid rasterizes label at scale and returns a copy of its coverage. directOnly
// leaves every run to Direct2D.
func rasterizeGrid(t *testing.T, label platform.TextLabel, scale float32, directOnly bool) gridRender {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	rasterizer.directOnly = directOnly
	width, height := int(math.Ceil(float64(label.Width*scale))), int(math.Ceil(float64(label.Height*scale)))
	pixels, lcd, err := rasterizer.rasterize(label, scale, width, height, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !lcd {
		t.Fatal("a fixed-pitch label was not drawn as ClearType coverage")
	}
	return gridRender{pixels: append([]byte(nil), pixels...), width: width, height: height}
}

// cover is the coverage of a pixel.
func (render gridRender) cover(x, y int) int { return int(render.pixels[(y*render.width+x)*4+3]) }

// column is the summed coverage of column x.
func (render gridRender) column(x int) int {
	total := 0
	for y := 0; y < render.height; y++ {
		total += render.cover(x, y)
	}
	return total
}

// between is the summed coverage of columns from to to.
func (render gridRender) between(from, to int) int {
	total := 0
	for x := from; x < to; x++ {
		total += render.column(x)
	}
	return total
}

// same reports whether the columns from to to hold the same coverage in both.
func (render gridRender) same(other gridRender, from, to int) bool {
	for y := 0; y < render.height; y++ {
		a := render.pixels[(y*render.width+from)*4 : (y*render.width+to)*4]
		b := other.pixels[(y*other.width+from)*4 : (y*other.width+to)*4]
		if !bytes.Equal(a, b) {
			return false
		}
	}
	return true
}

// gridLabel is a line as a terminal lays it out: runs of cells in the grid, each character
// on a cell. Every run is as long in cells as in bytes unless it says otherwise.
func gridLabel(cellWidth float32, runs ...gridRun) platform.TextLabel {
	label := platform.TextLabel{FontSize: 13, CellWidth: cellWidth, Mono: true, Height: 18}
	column := int32(0)
	for _, run := range runs {
		cells := run.cells
		if cells == 0 {
			cells = int32(len(run.text))
		}
		run.run.Column, run.run.Bytes, run.run.Cells = column, int32(len(run.text)), cells
		if run.run.Color == (ui.Color{}) {
			run.run.Color = termWhite
		}
		label.Value += run.text
		label.Runs = append(label.Runs, run.run)
		column += cells
	}
	label.Width = float32(column)*cellWidth + 2
	label.Color = platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}
	return label
}

type gridRun struct {
	text  string
	cells int32
	run   ui.TextRun
}

func asciiRun(value string) gridRun { return gridRun{text: value} }

// columnsOfRun is the device columns a run of a label owns.
func columnsOfRun(label platform.TextLabel, index int, scale float32) (from, to int) {
	pitch := float64(label.CellWidth * scale)
	run := label.Runs[index]
	return platform.GridColumns(float64(run.Column)*pitch, float64(run.Cells)*pitch, int(math.Ceil(float64(label.Width*scale))))
}

// TestFixedPitchCharactersStandOnTheirCells guards the grid: however it is drawn, character
// k of a line is on cell k, at the end of the line as well as at its start, whole device
// pixels per cell or not (125% scaling gives cells a fraction of a pixel). Each cell holds
// a narrow glyph, whose ink stands in the middle of its cell.
func TestFixedPitchCharactersStandOnTheirCells(t *testing.T) {
	const cells = 60
	value := string(bytes.Repeat([]byte{'I'}, cells))
	for _, scale := range []float32{1, 1.25, 1.5} {
		for _, cellWidth := range []float32{7, 7.4} {
			label := gridLabel(cellWidth, asciiRun(value))
			pitch := float64(cellWidth * scale)
			natural := rasterizeGrid(t, label, scale, true)
			for _, directOnly := range []bool{false, true} {
				t.Run(fmt.Sprintf("scale=%g/cell=%g/directOnly=%v", scale, cellWidth, directOnly), func(t *testing.T) {
					render := rasterizeGrid(t, label, scale, directOnly)
					if !directOnly && bytes.Equal(render.pixels, natural.pixels) {
						t.Error("the run was drawn by Direct2D")
					}
					for cell := 0; cell < cells; cell++ {
						left := int(math.Floor(float64(cell)*pitch + 0.5))
						right := int(math.Floor(float64(cell+1)*pitch+0.5)) + 1
						left = max(left-1, 0)
						right = min(right, render.width)
						sum, weighted := 0, 0.0
						for x := left; x < right; x++ {
							column := render.column(x)
							sum += column
							weighted += float64(column) * (float64(x) + 0.5)
						}
						if sum == 0 {
							t.Fatalf("cell %d holds no ink", cell)
						}
						if offset := weighted/float64(sum) - float64(cell)*pitch; offset < pitch*0.3 || offset > pitch*0.7 {
							t.Fatalf("the ink of cell %d is %.2f px from its edge, in a cell %.2f px wide", cell, offset, pitch)
						}
					}
				})
			}
		}
	}
}

// TestFixedPitchRunDecorationsAreDrawn guards the underline and strikethrough of a run: the
// terminal's SGR 4 and 9. Each is a line across the whole span of the run, below or through
// the glyphs, and it touches nothing outside the run.
func TestFixedPitchRunDecorationsAreDrawn(t *testing.T) {
	plain := gridLabel(8, asciiRun("ab "), asciiRun("cdefgh"), asciiRun(" ij "), asciiRun("klmnop"))
	decorated := gridLabel(8, asciiRun("ab "),
		gridRun{text: "cdefgh", run: ui.TextRun{Underline: true}}, asciiRun(" ij "),
		gridRun{text: "klmnop", run: ui.TextRun{Strike: true}})
	for _, directOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("directOnly=%v", directOnly), func(t *testing.T) {
			before := rasterizeGrid(t, plain, 1, directOnly)
			after := rasterizeGrid(t, decorated, 1, directOnly)
			baseline := platform.LabelBaseline(after.height, 13)
			for _, test := range []struct {
				name  string
				run   int
				below func(row int) bool
			}{
				{"underline", 1, func(row int) bool { return row > baseline }},
				{"strikethrough", 3, func(row int) bool { return row < baseline }},
			} {
				from, to := columnsOfRun(decorated, test.run, 1)
				best, bestRow := 0, -1
				for row := 0; row < after.height; row++ {
					added := 0
					for x := from; x < to; x++ {
						if gain := after.cover(x, row) - before.cover(x, row); gain > 0 {
							added += gain
						}
					}
					if added > best {
						best, bestRow = added, row
					}
				}
				if bestRow < 0 || !test.below(bestRow) {
					t.Fatalf("%s: the new line is at row %d, the baseline at %d", test.name, bestRow, baseline)
				}
				covered := 0
				for x := from; x < to; x++ {
					// A hairline may straddle two rows, so half coverage is a line; the gaps
					// between letters, which a line must cross, are not.
					if after.cover(x, bestRow) >= 40 {
						covered++
					}
				}
				if covered < (to-from)*9/10 {
					t.Errorf("%s: the line covers %d of the %d columns of its run", test.name, covered, to-from)
				}
			}
			// The runs without decoration are as they were: this cell's neighbours are ten
			// columns away, so the first run and the gap between the decorated ones are clean.
			if !before.same(after, 0, 2*8) {
				t.Error("a decoration changed the run before it")
			}
		})
	}
}

// TestFixedPitchDarkTextHasItsOwnCoverage guards the brightness of a run's text: ClearType
// thins light text and thickens dark text, so a dark run takes its coverage from text drawn
// dark, and a line of both takes each run's from its own drawing without a gap between them.
func TestFixedPitchDarkTextHasItsOwnCoverage(t *testing.T) {
	dark := ui.TextRun{Color: termInk}
	for _, directOnly := range []bool{false, true} {
		t.Run(fmt.Sprintf("directOnly=%v", directOnly), func(t *testing.T) {
			lightLine := gridLabel(8, asciiRun("IlOo@#$%"), asciiRun("IlOo@#$%"))
			darkLine := gridLabel(8, gridRun{text: "IlOo@#$%", run: dark}, gridRun{text: "IlOo@#$%", run: dark})
			mixed := gridLabel(8, asciiRun("IlOo@#$%"), gridRun{text: "IlOo@#$%", run: dark})
			lights := rasterizeGrid(t, lightLine, 1, directOnly)
			darks := rasterizeGrid(t, darkLine, 1, directOnly)
			both := rasterizeGrid(t, mixed, 1, directOnly)
			if bytes.Equal(lights.pixels, darks.pixels) {
				t.Fatal("dark text was given the coverage of light text")
			}
			// Away from the column the two halves share.
			if !both.same(lights, 0, 8*8-1) {
				t.Error("the light half of a mixed line is not what light text looks like")
			}
			if !both.same(darks, 8*8+1, 16*8-1) {
				t.Error("the dark half of a mixed line is not what dark text looks like")
			}
		})
	}
}

// TestFixedPitchRunsChooseTheirRenderer guards the choice in a line that holds both kinds of
// run, as a terminal lays out text with a wide character: the ASCII runs are GDI's, and the
// character the font may lack, whose run is its own, stays with Direct2D.
func TestFixedPitchRunsChooseTheirRenderer(t *testing.T) {
	label := gridLabel(8, asciiRun("Motty "), gridRun{text: "已", cells: 2}, gridRun{text: "连", cells: 2}, asciiRun(" connected"))
	hinted := rasterizeGrid(t, label, 1, false)
	natural := rasterizeGrid(t, label, 1, true)
	for index, wantHinted := range []bool{true, false, false, true} {
		from, to := columnsOfRun(label, index, 1)
		if hinted.between(from, to) == 0 {
			t.Errorf("run %d holds no ink", index)
		}
		// The column an edge shares with a neighbour holds the coverage of both.
		same := hinted.same(natural, from+1, to-1)
		if same == wantHinted {
			t.Errorf("run %d: drawn by GDI is %v, want %v", index, !same, wantHinted)
		}
	}
}

// TestFixedPitchRunsThatGDICannotDrawStayWithDirect2D guards the other fallbacks: a run that
// may ligate is shaped by Direct2D alone, and so is a family GDI does not have, which GDI
// would silently replace with another face.
func TestFixedPitchRunsThatGDICannotDrawStayWithDirect2D(t *testing.T) {
	ligatures := gridLabel(8, gridRun{text: "fi -> => != <=", run: ui.TextRun{Ligatures: true}})
	if !bytes.Equal(rasterizeGrid(t, ligatures, 1, false).pixels, rasterizeGrid(t, ligatures, 1, true).pixels) {
		t.Error("a ligature run was drawn by two renderers")
	}
	missing := gridLabel(8, asciiRun("root@host:~# ls -l"))
	missing.MonoFace = ui.MonoFamily("No Such Font Anywhere")
	hinted := rasterizeGrid(t, missing, 1, false)
	if !bytes.Equal(hinted.pixels, rasterizeGrid(t, missing, 1, true).pixels) {
		t.Error("a family GDI does not have was drawn by GDI")
	}
	if hinted.between(0, hinted.width) == 0 {
		t.Error("the line holds no ink")
	}
}

// TestFixedPitchRasterizerDeselectsItsFonts guards the rule for GDI objects: a font is not
// left selected in the context of the bitmap it was drawn on, because the context is deleted
// with the rasterizer and deleting an object that a context still holds is not defined.
// Windows 11 happens to defer that deletion; the rule is the documented one.
func TestFixedPitchRasterizerDeselectsItsFonts(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	label := gridLabel(8,
		asciiRun("root@host:~# "), gridRun{text: "ls", run: ui.TextRun{Weight: 0.4}},
		gridRun{text: " -l", run: ui.TextRun{Italic: true, Underline: true}})
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		t.Skipf("no DirectWrite: %v", err)
	}
	defer rasterizer.close()
	width, height := int(label.Width), int(label.Height)
	if _, _, err := rasterizer.rasterize(label, 1, width, height, true, 0); err != nil {
		t.Fatal(err)
	}
	if rasterizer.gdi == nil || len(rasterizer.gdi.fonts) < 3 {
		t.Fatal("the line was not drawn by GDI with a font for each of its styles")
	}
	selected, _, _ := graphics.NewProc("GetCurrentObject").Call(rasterizer.gdi.bitmap.dc, 6) // OBJ_FONT
	for key, font := range rasterizer.gdi.fonts {
		if font == selected {
			t.Errorf("the font %+v is still selected after the line is drawn", key)
		}
	}
}

// TestFixedPitchFontMustBeTheFamilyAsked guards the check of the face: a family GDI does
// not have is not given a font, where CreateFontIndirect would hand back another face.
func TestFixedPitchFontMustBeTheFamilyAsked(t *testing.T) {
	rasterizer := &gdiGridRasterizer{}
	defer rasterizer.close()
	if _, err := rasterizer.bind(64, 32); err != nil {
		t.Skipf("no GDI bitmap: %v", err)
	}
	if rasterizer.font(gdiFontKey{family: "Consolas", height: -13, weight: 400}) == 0 {
		t.Error("Consolas is not a font")
	}
	if font := rasterizer.font(gdiFontKey{family: "No Such Font Anywhere", height: -13, weight: 400}); font != 0 {
		t.Error("a family that does not exist was given a font")
	}
}

// BenchmarkFixedPitchLine measures one terminal line through both renderers a fixed-pitch
// label can take: GDI's ClearType and Direct2D's natural rendering.
func BenchmarkFixedPitchLine(b *testing.B) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	rasterizer, err := newDirectRasterizer()
	if err != nil {
		b.Fatal(err)
	}
	defer rasterizer.close()
	label := gridLabel(7, asciiRun("root@host:~# ls -la /var/log | head -20"))
	width, height := int(label.Width), int(label.Height)
	for _, directOnly := range []bool{true, false} {
		b.Run(fmt.Sprintf("hinted=%v", !directOnly), func(b *testing.B) {
			rasterizer.directOnly = directOnly
			b.ReportAllocs()
			for b.Loop() {
				if _, _, err := rasterizer.rasterize(label, 1, width, height, true, 0); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
