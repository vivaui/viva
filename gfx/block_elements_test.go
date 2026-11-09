package gfx

import (
	"math"
	"strings"
	"testing"
)

func blockLine(canvas *Canvas, value string, cellWidth float32, color Color) Text {
	runs := make([]TextRun, 0, len(value))
	column := int32(0)
	for _, r := range value {
		runs = append(runs, TextRun{Column: column, Bytes: int32(len(string(r))), Cells: 1, Color: color})
		column++
	}
	canvas.LabelRuns(value, Rect{X: 3.3, Y: 5.7, Width: float32(column) * cellWidth, Height: 19}, TextStyle{Size: 14}, cellWidth, runs)
	return canvas.Texts()[len(canvas.Texts())-1]
}

func covered(rectangles []Rectangle) float64 {
	var area float64
	for _, rectangle := range rectangles {
		area += float64(rectangle.Bounds.Width) * float64(rectangle.Bounds.Height) * float64(rectangle.Color.Alpha)
	}
	return area
}

// Every Block Element fills the fraction of its cell its Unicode name gives.
func TestBlockRectanglesFillTheirNamedFractionOfTheCell(t *testing.T) {
	sixtyFourths := map[rune]float64{0x2580: 32, 0x2590: 32, 0x2594: 8, 0x2595: 8}
	for r := rune(0x2581); r <= 0x2588; r++ {
		sixtyFourths[r] = float64(r-0x2580) * 8
	}
	for r := rune(0x2589); r <= 0x258F; r++ {
		sixtyFourths[r] = float64(0x2590-r) * 8
	}
	for r, shade := range map[rune]float64{0x2591: 16, 0x2592: 32, 0x2593: 48} {
		sixtyFourths[r] = shade
	}
	for r, count := range map[rune]float64{0x2596: 1, 0x2597: 1, 0x2598: 1, 0x2599: 3, 0x259A: 2, 0x259B: 3, 0x259C: 3, 0x259D: 1, 0x259E: 2, 0x259F: 3} {
		sixtyFourths[r] = 16 * count
	}
	if len(sixtyFourths) != 32 {
		t.Fatalf("table covers %d characters, want the 32 of U+2580 to U+259F", len(sixtyFourths))
	}
	var canvas Canvas
	canvas.SetPixelScale(1) // whole pixels: the cell is 16 by 24
	for r, want := range sixtyFourths {
		canvas.Reset()
		text := blockLine(&canvas, string(r), 16, Color{Red: 1, Alpha: 1})
		text.Bounds = Rect{Width: 16, Height: 24}
		got := covered(canvas.BlockRectangles(nil, text)) / (16 * 24) * 64
		if math.Abs(got-want) > 1e-6 {
			t.Errorf("%U fills %.2f/64 of its cell, want %.0f/64", r, got, want)
		}
	}
}

func TestBlockRectanglesPlaceTheirParts(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	cell := func(r rune) []Rect {
		canvas.Reset()
		text := blockLine(&canvas, string(r), 16, Color{Alpha: 1})
		text.Bounds = Rect{Width: 16, Height: 24}
		var parts []Rect
		for _, rectangle := range canvas.BlockRectangles(nil, text) {
			parts = append(parts, rectangle.Bounds)
		}
		return parts
	}
	for _, test := range []struct {
		name string
		r    rune
		want []Rect
	}{
		{"upper half", '▀', []Rect{{Width: 16, Height: 12}}},
		{"lower half", '▄', []Rect{{Y: 12, Width: 16, Height: 12}}},
		{"left half", '▌', []Rect{{Width: 8, Height: 24}}},
		{"right half", '▐', []Rect{{X: 8, Width: 8, Height: 24}}},
		{"upper left", '▘', []Rect{{Width: 8, Height: 12}}},
		{"diagonal", '▚', []Rect{{Width: 8, Height: 12}, {X: 8, Y: 12, Width: 8, Height: 12}}},
		{"anti-diagonal", '▞', []Rect{{X: 8, Width: 8, Height: 12}, {Y: 12, Width: 8, Height: 12}}},
	} {
		got := cell(test.r)
		if len(got) != len(test.want) {
			t.Errorf("%s: %v, want %v", test.name, got, test.want)
			continue
		}
		for index := range got {
			if got[index] != test.want[index] {
				t.Errorf("%s: %v, want %v", test.name, got, test.want)
				break
			}
		}
	}
}

// Cells of an application's grid that meet at a fractional pixel position meet at the same
// device pixel: nothing is left between them and nothing is drawn twice, at any scale.
func TestBlockRectanglesLeaveNoSeamBetweenCells(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		for _, cellWidth := range []float32{7.3, 8.4, 10.37, 11, 13.6} {
			var canvas Canvas
			canvas.SetPixelScale(scale)
			// Alternating shapes defeat merging: each cell is a rectangle of its own.
			text := blockLine(&canvas, "█▀█▄█▀█▄█▀█▄", cellWidth, Color{Green: 1, Alpha: 1})
			rectangles := canvas.BlockRectangles(nil, text)
			if len(rectangles) != 12 {
				t.Fatalf("scale %v cell %v: %d rectangles, want one per cell", scale, cellWidth, len(rectangles))
			}
			for index := 1; index < len(rectangles); index++ {
				previous, current := rectangles[index-1].Bounds, rectangles[index].Bounds
				end, start := float64(previous.X+previous.Width)*float64(scale), float64(current.X)*float64(scale)
				if math.Round(end) != math.Round(start) {
					t.Errorf("scale %v cell %v: cell %d ends at device pixel %v, the next starts at %v", scale, cellWidth, index-1, end, start)
				}
			}
			for _, rectangle := range rectangles {
				for _, edge := range []float32{rectangle.Bounds.X, rectangle.Bounds.X + rectangle.Bounds.Width, rectangle.Bounds.Y, rectangle.Bounds.Y + rectangle.Bounds.Height} {
					if device := float64(edge) * float64(scale); math.Abs(device-math.Round(device)) > 1e-4 {
						t.Errorf("scale %v cell %v: edge %v is not a whole device pixel", scale, cellWidth, edge)
					}
				}
			}
		}
	}
}

func TestBlockRectanglesMergeARunOfOneCharacterAndColor(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	red, blue := Color{Red: 1, Alpha: 1}, Color{Blue: 1, Alpha: 1}
	canvas.LabelRuns("▀▀▀▀▀▀", Rect{X: 10, Y: 20, Width: 66, Height: 18}, TextStyle{Size: 14}, 11, []TextRun{
		{Column: 0, Bytes: 3, Cells: 1, Color: red}, {Column: 1, Bytes: 3, Cells: 1, Color: red}, {Column: 2, Bytes: 3, Cells: 1, Color: red},
		{Column: 3, Bytes: 3, Cells: 1, Color: blue}, {Column: 4, Bytes: 3, Cells: 1, Color: blue}, {Column: 5, Bytes: 3, Cells: 1, Color: red},
	})
	rectangles := canvas.BlockRectangles(nil, canvas.Texts()[0])
	if len(rectangles) != 3 {
		t.Fatalf("%d rectangles, want red, blue and red", len(rectangles))
	}
	if rectangles[0].Bounds != (Rect{X: 10, Y: 20, Width: 33, Height: 9}) || rectangles[0].Color != red ||
		rectangles[1].Bounds != (Rect{X: 43, Y: 20, Width: 22, Height: 9}) || rectangles[1].Color != blue {
		t.Fatalf("rectangles = %+v", rectangles)
	}
}

func TestBlockRectanglesCarryLayerClipAndShadeOpacity(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	canvas.PushLayer(4)
	canvas.Clip(Rect{X: 0, Y: 0, Width: 30, Height: 30})
	text := blockLine(&canvas, "▒", 10, Color{Red: 1, Alpha: 0.8})
	rectangles := canvas.BlockRectangles(nil, text)
	if len(rectangles) != 1 || rectangles[0].Layer != 4 || !rectangles[0].HasClip || rectangles[0].ClipOf != (Rect{Width: 30, Height: 30}) {
		t.Fatalf("rectangles = %+v", rectangles)
	}
	if math.Abs(float64(rectangles[0].Color.Alpha)-0.4) > 1e-6 {
		t.Fatalf("medium shade at alpha 0.8 has alpha %v, want 0.4", rectangles[0].Color.Alpha)
	}
}

func TestBlockRectanglesIgnoreTextWithoutBlockElements(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	text := blockLine(&canvas, "ls -la ─│┌ é", 9, Color{Red: 1, Alpha: 1})
	scratch := make([]Rectangle, 0, 8)
	if got := canvas.BlockRectangles(scratch, text); len(got) != 0 {
		t.Fatalf("%d rectangles for text with no Block Elements", len(got))
	}
	line := blockLine(&canvas, strings.Repeat("█▄", 40), 9, Color{Red: 1, Alpha: 1})
	scratch = make([]Rectangle, 0, 128)
	if allocations := testing.AllocsPerRun(100, func() { scratch = canvas.BlockRectangles(scratch[:0], line) }); allocations != 0 {
		t.Fatalf("drawing a warm line of blocks allocated %v times", allocations)
	}
}

func TestReplaceBlockElementsKeepsEveryCellInPlace(t *testing.T) {
	value := "a█é▀b"
	runs := []TextRun{{Column: 0, Bytes: 1, Cells: 1}, {Column: 1, Bytes: 3, Cells: 1}, {Column: 2, Bytes: 2, Cells: 1}, {Column: 3, Bytes: 3, Cells: 1}, {Column: 4, Bytes: 1, Cells: 1}}
	text, replaced := ReplaceBlockElements(value, runs)
	if text != "a é b" {
		t.Fatalf("text = %q", text)
	}
	offset := 0
	for index, run := range replaced {
		if run.Column != runs[index].Column || run.Cells != runs[index].Cells {
			t.Fatalf("run %d moved: %+v", index, run)
		}
		offset += int(run.Bytes)
	}
	if offset != len(text) {
		t.Fatalf("runs cover %d of %d bytes", offset, len(text))
	}
	if runs[1].Bytes != 3 || value != "a█é▀b" {
		t.Fatal("the arguments were changed")
	}
}

func TestReplaceBlockElementsLeavesOtherTextAlone(t *testing.T) {
	value := "ls ─ é \xe2\x96 \xff"
	runs := []TextRun{{Bytes: int32(len(value)), Cells: 12}}
	text, replaced := ReplaceBlockElements(value, runs)
	if text != value || &replaced[0] != &runs[0] {
		t.Fatal("text without Block Elements was rebuilt")
	}
	broken := "x\xe2\x96\x80\xe2\x96"
	text, _ = ReplaceBlockElements(broken, []TextRun{{Bytes: int32(len(broken))}})
	if text != "x \xe2\x96" {
		t.Fatalf("invalid UTF-8 changed: %q", text)
	}
	if allocations := testing.AllocsPerRun(100, func() { ReplaceBlockElements(value, runs) }); allocations != 0 {
		t.Fatalf("a line with no blocks allocated %v times", allocations)
	}
}

func BenchmarkBlockRectangles(b *testing.B) {
	var canvas Canvas
	canvas.SetPixelScale(1.25)
	line := blockLine(&canvas, strings.Repeat("▀█▄ ▌", 24), 9.6, Color{Red: 1, Alpha: 1})
	scratch := make([]Rectangle, 0, 256)
	b.ReportAllocs()
	for b.Loop() {
		scratch = canvas.BlockRectangles(scratch[:0], line)
	}
}
