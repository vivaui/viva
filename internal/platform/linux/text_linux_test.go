package linux

import (
	"bytes"
	"fmt"
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/platform"
)

func TestMeasureTextSupportsSystemFallback(t *testing.T) {
	measure, close, err := NewTextMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	for _, value := range []string{"Viva", "原生界面", "Viva 原生界面 👩‍💻"} {
		metrics, err := measure(value, 16, 0)
		if err != nil {
			t.Fatalf("measure(%q): %v", value, err)
		}
		if metrics.Width <= 0 || metrics.Height <= 0 || metrics.Ascent <= 0 || metrics.Descent < 0 {
			t.Fatalf("metrics for %q = %+v", value, metrics)
		}
	}
}

func TestMeasureTextRejectsInvalidFontSize(t *testing.T) {
	for _, size := range []float32{0, -1, float32(math.NaN()), float32(math.Inf(1))} {
		if _, err := MeasureText("Viva", size, 0); err == nil {
			t.Fatalf("invalid font size %v accepted", size)
		}
	}
}

func TestMeasureMonoReportsStableCell(t *testing.T) {
	width, height, err := MeasureMono(16, gfx.MonoSystem)
	if err != nil {
		t.Fatal(err)
	}
	if width <= 0 || height <= 0 || width >= height {
		t.Fatalf("monospace cell = %vx%v", width, height)
	}
	// The engine reports the same cell to the canvas, with the font's line box around its
	// baseline: a baseline inside the cell, and half the line gap above and below it.
	cell := sharedTextEngine().MonoCell(16, gfx.MonoSystem)
	if cell.Advance != width || cell.Line() != height || cell.LineGap < 0 {
		t.Fatalf("engine cell = %+v, MeasuredMono %vx%v", cell, width, height)
	}
	cellHeight, baseline := cell.CellHeight(), cell.CellBaseline()
	if cellHeight != float32(int(cellHeight)) || baseline <= 0 || baseline >= cellHeight {
		t.Fatalf("cell height %v, baseline %v: want a whole cell with a row inside it", cellHeight, baseline)
	}
}

func TestRasterizeTextSystemFallbackAndRuns(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	labels := []platform.TextLabel{
		{Value: "Viva 原生界面", Width: 240, Height: 32, FontSize: 16, Color: platform.Color{Red: 0.8, Green: 0.6, Blue: 0.4, Alpha: 0.75}},
		{Value: "A中", Width: 40, Height: 24, FontSize: 16, Mono: true, CellWidth: 10, Runs: []gfx.TextRun{
			{Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Alpha: 1}, Underline: true},
			{Column: 1, Bytes: 3, Cells: 2, Color: gfx.Color{Green: 1, Alpha: 0.8}, Weight: 0.35, Italic: true, Strike: true},
		}},
	}
	for _, label := range labels {
		pixels, _, err := services.rasterize(label, 1, int(label.Width), int(label.Height), false)
		if err != nil {
			t.Fatal(err)
		}
		visible := false
		for index := 0; index < len(pixels); index += 4 {
			alpha := pixels[index+3]
			visible = visible || alpha != 0
			if pixels[index] > alpha || pixels[index+1] > alpha || pixels[index+2] > alpha {
				t.Fatal("Cairo returned non-premultiplied text pixels")
			}
		}
		if !visible {
			t.Fatalf("rasterization of %q produced an empty bitmap", label.Value)
		}
	}
}

func TestRasterizeTextCentersInTallerBounds(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{Value: "Viva", Width: 80, Height: 28, FontSize: 16, Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}
	top := rasterAlphaBounds(t, services, label)
	label.Height += 20
	taller := rasterAlphaBounds(t, services, label)
	if shift := taller.minY - top.minY; shift != 10 {
		t.Fatalf("a label 20 pixels taller shifted text by %d pixels, want 10", shift)
	}
}

func TestRasterizeFixedPitchASCIIStartsAtItsColumn(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{
		Value: "A", Width: 32, Height: 32, FontSize: 20, Mono: true, CellWidth: 32,
		Runs: []gfx.TextRun{{Bytes: 1, Cells: 1, Color: gfx.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1}}},
	}
	bounds := rasterAlphaBounds(t, services, label)
	if bounds.minX > 3 {
		t.Fatalf("fixed-pitch ASCII starts at x=%d, want its column origin", bounds.minX)
	}
}

func TestRasterizeLigatureRunsDoesNotAccumulateScaling(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	services.useFonts(systemFonts().grayscale())
	label := platform.TextLabel{
		Value: "abcabc", Width: 64, Height: 24, FontSize: 16, Mono: true, CellWidth: 8,
		Runs: []gfx.TextRun{
			{Bytes: 3, Cells: 3, Color: gfx.RGB(255, 255, 255), Ligatures: true},
			{Column: 4, Bytes: 3, Cells: 3, Color: gfx.RGB(255, 255, 255), Ligatures: true},
		},
	}
	want, err := services.configureLayout("abc", monoFamily(label.MonoFace), label.FontSize, 0, false)
	if err != nil {
		t.Fatal(err)
	}
	var first []byte
	for frame := range 32 {
		pixels, _, err := services.rasterize(label, 1, 64, 24, false)
		if err != nil {
			t.Fatal(err)
		}
		if frame == 0 {
			first = pixels
		} else if !bytes.Equal(pixels, first) {
			t.Fatalf("frame %d changed an unchanged label's pixels", frame)
		}
		got, err := services.configureLayout("abc", monoFamily(label.MonoFace), label.FontSize, 0, false)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("frame %d left scaled text metrics: got %+v, want %+v", frame, got, want)
		}
	}
	rasterAlphaBounds(t, services, label)
}

func TestRasterizeLigatureRunFitsItsCells(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, value := range []string{"check", "followups", "issue-draft", "workbench_workspaces_e2e-vision-conversation-smoke"} {
			t.Run(fmt.Sprintf("%s@%g", value, scale), func(t *testing.T) {
				services, err := newTextServices()
				if err != nil {
					t.Fatal(err)
				}
				defer services.close()
				// The cells are narrower than the font's advance, so the fitting
				// scale is not the identity and a reshaped layout shows up in the
				// width the run is drawn at.
				label := platform.TextLabel{
					Value: value, Width: 600, Height: 28, FontSize: 14, Mono: true, CellWidth: 7,
					Runs: []gfx.TextRun{{Bytes: int32(len(value)), Cells: int32(len(value)), Color: gfx.RGB(255, 255, 255), Ligatures: true}},
				}
				services.useFonts(systemFonts().grayscale())
				measured, err := services.configureLayout(value, monoFamily(label.MonoFace), label.FontSize*scale, 0, false)
				if err != nil {
					t.Fatal(err)
				}
				show := services.showLayout
				services.showLayout = func(context, layout uintptr) {
					var logical pangoRectangle
					services.layoutGetExtents(layout, nil, &logical)
					if width := float32(logical.Width) / pangoScale; width != measured.Width {
						t.Errorf("run was reshaped after fitting: measured %g, drawing %g", measured.Width, width)
					}
					show(context, layout)
				}
				rasterAlphaBoundsAt(t, services, label, scale)
			})
		}
	}
}

type alphaBounds struct{ minX, minY, maxX, maxY int }

func rasterAlphaBounds(t *testing.T, services *textServices, label platform.TextLabel) alphaBounds {
	t.Helper()
	return rasterAlphaBoundsAt(t, services, label, 1)
}

func rasterAlphaBoundsAt(t *testing.T, services *textServices, label platform.TextLabel, scale float32) alphaBounds {
	t.Helper()
	width, height := int(math.Ceil(float64(label.Width*scale))), int(math.Ceil(float64(label.Height*scale)))
	pixels, _, err := services.rasterize(label, scale, width, height, false)
	if err != nil {
		t.Fatal(err)
	}
	bounds := alphaBounds{minX: width, minY: height, maxX: -1, maxY: -1}
	for y := range height {
		for x := range width {
			if pixels[(y*width+x)*4+3] == 0 {
				continue
			}
			bounds.minX = min(bounds.minX, x)
			bounds.minY = min(bounds.minY, y)
			bounds.maxX = max(bounds.maxX, x)
			bounds.maxY = max(bounds.maxY, y)
		}
	}
	if bounds.maxX < 0 {
		t.Fatal("text rasterization produced no alpha")
	}
	return bounds
}

func TestTextRunsMustCoverUTF8Label(t *testing.T) {
	services, err := newTextServices()
	if err != nil {
		t.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{Value: "中", Width: 20, Height: 20, FontSize: 14, CellWidth: 10, Runs: []gfx.TextRun{{Bytes: 1, Cells: 1}}}
	if _, _, err := services.rasterize(label, 1, 20, 20, false); err == nil {
		t.Fatal("a run splitting UTF-8 was accepted")
	}
}

func BenchmarkMeasureTextMixed(benchmark *testing.B) {
	measure, close, err := NewTextMeasurer()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer close()
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, err := measure("Viva 原生界面 👩‍💻", 16, 0); err != nil {
			benchmark.Fatal(err)
		}
	}
}

func BenchmarkRasterizeTextMixed(benchmark *testing.B) {
	services, err := newTextServices()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{
		Value: "Viva 原生界面 👩‍💻", Width: 240, Height: 32, FontSize: 16,
		Color: platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1},
	}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, _, err := services.rasterize(label, 1, 240, 32, false); err != nil {
			benchmark.Fatal(err)
		}
	}
}

func BenchmarkRasterizeLigatureRun(benchmark *testing.B) {
	services, err := newTextServices()
	if err != nil {
		benchmark.Fatal(err)
	}
	defer services.close()
	label := platform.TextLabel{
		Value: "workbench_workspaces_e2e-vision-conversation-smoke", Width: 480, Height: 24, FontSize: 16, Mono: true, CellWidth: 8,
	}
	label.Runs = []gfx.TextRun{{Bytes: int32(len(label.Value)), Cells: int32(len(label.Value)), Color: gfx.RGB(255, 255, 255), Ligatures: true}}
	benchmark.ReportAllocs()
	benchmark.ResetTimer()
	for range benchmark.N {
		if _, _, err := services.rasterize(label, 1, 480, 24, false); err != nil {
			benchmark.Fatal(err)
		}
	}
}

// A heavier weight is a wider line, and a label fitted at the regular weight
// would clip at the heavier one.
func TestMeasureTextIsWiderAtAHeavierWeight(t *testing.T) {
	measure, close, err := NewTextMeasurer()
	if err != nil {
		t.Fatal(err)
	}
	defer close()
	regular, err := measure("Import SSH Config", 14, 0)
	if err != nil {
		t.Fatal(err)
	}
	bold, err := measure("Import SSH Config", 14, 0.40)
	if err != nil {
		t.Fatal(err)
	}
	if bold.Width <= regular.Width {
		t.Fatalf("bold %.2f is not wider than regular %.2f", bold.Width, regular.Width)
	}
}

// The engine reports the thickness the font underlines with, in logical pixels at the size
// asked: that is the rule a cell glyph is drawn with, so box drawing geometry weighs what
// the font's own glyphs weigh. Pango reads it from the face; a face that files no underline
// thickness answers 0, and the canvas then uses the reference thickness.
func TestTheEngineReportsTheFontsUnderline(t *testing.T) {
	engine := sharedTextEngine()
	cell := engine.MonoCell(13, gfx.MonoSystem)
	if cell.Advance <= 0 {
		t.Fatal("no fixed-pitch cell on this runner")
	}
	if cell.Underline == 0 {
		t.Skip("the runner's fixed-pitch face reports no underline thickness")
	}
	if cell.Underline < 0 || cell.Underline >= cell.Advance {
		t.Fatalf("underline %v of a cell %v wide, want a positive stroke narrower than the cell", cell.Underline, cell.Advance)
	}
	if at26 := engine.MonoCell(26, gfx.MonoSystem); at26.Underline <= cell.Underline {
		t.Errorf("underline %v at size 26, %v at size 13, want it to follow the size", at26.Underline, cell.Underline)
	}
}
