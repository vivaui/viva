package gfx

import (
	"math"
	"slices"
	"testing"
	"unicode/utf8"
)

// glyphShapes is the geometry of r drawn in cells cells of the given size.
func glyphShapes(t *testing.T, r rune, cellWidth, cellHeight, scale, size float32, cells int32) CellGlyphShapes {
	t.Helper()
	var canvas Canvas
	if scale > 0 {
		canvas.SetPixelScale(scale)
	}
	value := string(r)
	canvas.LabelRuns(value, Rect{Width: float32(cells) * cellWidth, Height: cellHeight},
		TextStyle{Size: size}, cellWidth,
		[]TextRun{{Column: 0, Bytes: int32(len(value)), Cells: cells, Color: Color{Red: 1, Alpha: 1}}})
	texts := canvas.Texts()
	if len(texts) == 0 {
		t.Fatalf("%U: no text was declared", r)
	}
	return canvas.CellGlyphGeometry(CellGlyphShapes{}, texts[len(texts)-1])
}

// glyphCell is the rectangles of r drawn in cells cells of the given size.
func glyphCell(t *testing.T, r rune, cellWidth, cellHeight, scale, size float32, cells int32) []Rectangle {
	t.Helper()
	return glyphShapes(t, r, cellWidth, cellHeight, scale, size, cells).Rectangles
}

// Braille draws its dots as squares on one grid: one gap between any two dots — inside a cell and
// across the edge to the next one — so a spinner reads as a grid, not as pairs of dots.
func TestBrailleDrawsItsDots(t *testing.T) {
	if dots := glyphCell(t, 0x2800, 10, 20, 1, 15, 1); len(dots) != 0 {
		t.Fatalf("the blank pattern draws %d dots", len(dots))
	}
	dots := glyphCell(t, 0x28FF, 10, 20, 1, 15, 1)
	if len(dots) != 8 {
		t.Fatalf("a full pattern draws %d dots, want 8", len(dots))
	}
	if dot := dots[0].Bounds; math.Abs(float64(dot.Width-dot.Height)) > 1e-4 {
		t.Fatalf("a dot is %gx%g, want a square", dot.Width, dot.Height)
	}
	for _, size := range []struct{ width, height float32 }{{10, 20}, {10, 25}, {7, 13}} {
		for _, scale := range []float32{1, 2} {
			dots := glyphCell(t, 0x28FF, size.width, size.height, scale, 15, 1)
			if len(dots) != 8 {
				t.Fatalf("%gx%g at scale %g: %d dots", size.width, size.height, scale, len(dots))
			}
			// Bit order: left and right of the top row, of the upper, of the lower, then the
			// bottom row.
			dot := dots[0].Bounds.Width
			for _, axis := range []struct {
				name   string
				starts []float32
				extent float32
			}{
				{"columns", []float32{dots[0].Bounds.X, dots[3].Bounds.X}, size.width},
				{"rows", []float32{dots[0].Bounds.Y, dots[1].Bounds.Y, dots[2].Bounds.Y, dots[6].Bounds.Y}, size.height},
			} {
				first := axis.starts[0]
				gap := axis.starts[1] - (first + dot)
				across := axis.extent - (axis.starts[len(axis.starts)-1] + dot) + first
				if math.Abs(float64(across-gap)) > float64(1/scale)+1e-4 {
					t.Errorf("%gx%g at scale %g: %s are %g apart inside a cell but %g across its edge",
						size.width, size.height, scale, axis.name, gap, across)
				}
				for index := 2; index < len(axis.starts); index++ {
					if other := axis.starts[index] - (axis.starts[index-1] + dot); math.Abs(float64(other-gap)) > float64(1/scale)+1e-4 {
						t.Errorf("%gx%g at scale %g: %s gaps are %g and %g", size.width, size.height, scale, axis.name, gap, other)
						break
					}
				}
			}
			for _, square := range dots {
				if bounds := square.Bounds; bounds.X < 0 || bounds.Y < 0 ||
					bounds.X+bounds.Width > size.width || bounds.Y+bounds.Height > size.height {
					t.Fatalf("%gx%g: a dot at %+v is outside its cell", size.width, size.height, bounds)
				}
			}
		}
	}
	// A small cell still shows a dot rather than a dotted nothing.
	for _, square := range glyphCell(t, 0x28FF, 4, 8, 1, 10, 1) {
		if square.Bounds.Width < 1 || square.Bounds.Height < 1 {
			t.Fatalf("a 4x8 cell draws a dot %+v", square.Bounds)
		}
	}
}

// filledArea is the area of one triangle from its three points.
func filledArea(triangle Triangle) float64 {
	a, b, c := triangle.Points[0], triangle.Points[1], triangle.Points[2]
	return math.Abs(float64((b.X-a.X)*(c.Y-a.Y)-(b.Y-a.Y)*(c.X-a.X))) / 2
}

// The corner triangles fill or outline the corner their name gives: a filled one is one triangle
// over half the cell, an outlined one its two legs and a stroked hypotenuse.
func TestGeometricShapesFillTheirCorner(t *testing.T) {
	edges := map[rune][2]string{
		0x25E2: {"right", "bottom"}, // ◢
		0x25E3: {"left", "bottom"},  // ◣
		0x25E4: {"left", "top"},     // ◤
		0x25E5: {"right", "top"},    // ◥
		0x25F8: {"left", "top"},     // ◸
		0x25F9: {"right", "top"},    // ◹
		0x25FA: {"left", "bottom"},  // ◺
		0x25FF: {"right", "bottom"}, // ◿
	}
	const width, height = 10, 20
	for r, want := range edges {
		shapes := glyphShapes(t, r, width, height, 1, 15, 1)
		if len(shapes.Rectangles) == 0 && len(shapes.Triangles) == 0 {
			t.Fatalf("%U draws nothing", r)
		}
		box := geometryBox(shapes.Rectangles, shapes.Triangles)
		reaches := map[string]bool{
			"left":   box.X <= 0,
			"right":  box.X+box.Width >= width,
			"top":    box.Y <= 0,
			"bottom": box.Y+box.Height >= height,
		}
		for _, edge := range want {
			if !reaches[edge] {
				t.Errorf("%U does not reach the %s edge", r, edge)
			}
		}
		if solid := r >= 0x25E2 && r <= 0x25E5; solid {
			// One triangle: its hypotenuse is a real edge, not a column of pixels.
			if len(shapes.Rectangles) != 0 || len(shapes.Triangles) != 1 {
				t.Errorf("%U is drawn as %d rectangles and %d triangles, want one triangle",
					r, len(shapes.Rectangles), len(shapes.Triangles))
				continue
			}
			if area := filledArea(shapes.Triangles[0]); math.Abs(area/(width*height)-0.5) > 1e-3 {
				t.Errorf("%U fills %.3f of its cell, want half", r, area/(width*height))
			}
			continue
		}
		// One closed stroke, inset by half a rule, so the ink stays inside the cell.
		if len(shapes.Rectangles) != 0 || len(shapes.Triangles) == 0 {
			t.Errorf("%U is drawn as %d rectangles and %d triangles, want one stroked outline",
				r, len(shapes.Rectangles), len(shapes.Triangles))
		}
		if box.X < 0 || box.Y < 0 || box.X+box.Width > width || box.Y+box.Height > height {
			t.Errorf("%U draws %+v outside its %vx%v cell", r, box, width, height)
		}
		var area float64
		for _, triangle := range shapes.Triangles {
			area += filledArea(triangle)
		}
		if area/(width*height) > 0.35 {
			t.Errorf("%U outlines its cell but fills %.2f of it", r, area/(width*height))
		}
	}
}

// A character that takes two cells draws across both of them.
func TestWideCellGlyphSpansItsCells(t *testing.T) {
	full := glyphCell(t, '█', 10, 20, 1, 15, 2)
	if len(full) != 1 {
		t.Fatalf("a two-cell block is %d rectangles, want 1", len(full))
	}
	if bounds := full[0].Bounds; bounds.X != 0 || bounds.Width != 20 || bounds.Height != 20 {
		t.Fatalf("a two-cell block covers %+v, want the whole 20x20", bounds)
	}
	half := glyphCell(t, '▀', 10, 20, 1, 15, 2)
	if len(half) != 1 {
		t.Fatalf("a two-cell upper half is %d rectangles, want 1", len(half))
	}
	if bounds := half[0].Bounds; bounds.Width != 20 || bounds.Height != 10 {
		t.Fatalf("a two-cell upper half covers %+v, want 20x10", bounds)
	}
}

// Every family drawn as geometry replaces its characters with spaces for the font, while the
// line's other text and runs stay where they were.
func TestReplaceCellGlyphs(t *testing.T) {
	for _, value := range []string{"█", "─", "⠋", "◢", "◸", "\ue0b0", "\U0001fb00", "\U0001ce90"} {
		if replaced, _ := ReplaceCellGlyphs(value, []TextRun{{Bytes: int32(len(value)), Cells: 1}}); replaced != " " {
			t.Errorf("%q was replaced with %q, want a space", value, replaced)
		}
	}
	for _, value := range []string{"a", "é", "中", "♠"} {
		if replaced, _ := ReplaceCellGlyphs(value, []TextRun{{Bytes: int32(len(value)), Cells: 1}}); replaced != value {
			t.Errorf("%q is not geometry but was replaced with %q", value, replaced)
		}
	}

	value := "a█é▀b"
	runs := []TextRun{{Column: 0, Bytes: 1, Cells: 1}, {Column: 1, Bytes: 3, Cells: 1}, {Column: 2, Bytes: 2, Cells: 1}, {Column: 3, Bytes: 3, Cells: 1}, {Column: 4, Bytes: 1, Cells: 1}}
	text, replaced := ReplaceCellGlyphs(value, runs)
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

	four := "a\U0001fb00b"
	fourRuns := []TextRun{{Bytes: 1}, {Bytes: 4}, {Bytes: 1}}
	if text, runs := ReplaceCellGlyphs(four, fourRuns); text != "a b" || runs[0].Bytes != 1 || runs[1].Bytes != 1 || runs[2].Bytes != 1 {
		t.Fatalf("a four-byte glyph became %q with runs %+v", text, runs)
	}

	plain := "ls é \xe2\x96 \xff"
	plainRuns := []TextRun{{Bytes: int32(len(plain)), Cells: 12}}
	text, same := ReplaceCellGlyphs(plain, plainRuns)
	if text != plain || &same[0] != &plainRuns[0] {
		t.Fatal("text without cell glyphs was rebuilt")
	}
	broken := "x\xe2\x96\x80\xe2\x96"
	text, _ = ReplaceCellGlyphs(broken, []TextRun{{Bytes: int32(len(broken))}})
	if text != "x \xe2\x96" {
		t.Fatalf("invalid UTF-8 changed: %q", text)
	}
	if allocations := testing.AllocsPerRun(100, func() { ReplaceCellGlyphs(plain, plainRuns) }); allocations != 0 {
		t.Fatalf("a line with no cell glyphs allocated %v times", allocations)
	}
}

// FuzzReplaceCellGlyphs feeds terminal bytes to ReplaceCellGlyphs split into runs at
// arbitrary byte offsets, including the middle of a glyph: whatever walks those bytes,
// the text stays the same characters with every whole glyph standing in for one space,
// and the runs still cover exactly the text it returns.
func FuzzReplaceCellGlyphs(f *testing.F) {
	f.Add("─┼╭", "")
	f.Add("x\xe2\x96\x80\xe2\x96", "\x00")
	f.Add("a⠀b▄", "\x01\x02")
	f.Add("──│ mixed ──", "\x02")
	f.Add("", "\x00")
	f.Fuzz(func(t *testing.T, value, cuts string) {
		runs := runsOver(value, cuts)
		text, replaced := ReplaceCellGlyphs(value, runs)
		if got, want := utf8.RuneCountInString(text), utf8.RuneCountInString(value); got != want {
			t.Fatalf("%d runes in %q, %d in the value it replaced", got, text, want)
		}
		if len(text) > len(value) {
			t.Fatalf("%d bytes out of %d in %q", len(text), len(value), value)
		}
		if !hasCellGlyph(value) {
			if text != value || &replaced[0] != &runs[0] {
				t.Fatalf("text without cell glyphs was rebuilt: %q became %q", value, text)
			}
		}
		var covered int32
		for _, run := range replaced {
			if run.Bytes < 0 || covered+run.Bytes > int32(len(text)) {
				t.Fatalf("run of %d bytes at %d does not fit %d bytes of %q", run.Bytes, covered, len(text), text)
			}
			covered += run.Bytes
		}
		if covered != int32(len(text)) {
			t.Fatalf("runs cover %d of %d bytes of %q", covered, len(text), text)
		}
	})
}

// A cell glyph's strokes are the face's own (MonoCellMetrics.Rule and HeavyRule) at the
// line's scale, each rounded to the nearest device pixel and never thinner than one: a box
// drawing character drawn as geometry then weighs what the font's own glyph weighs, light
// or heavy, and a face whose heavy line is not twice its light one keeps both weights.
func TestCellGlyphRulesFollowTheFacesOwnStrokes(t *testing.T) {
	for _, test := range []struct {
		name      string
		rule      float32 // logical pixels, as the engine reports them
		heavy     float32
		scale     float32
		wantRule  float32 // device pixels
		wantHeavy float32
	}{
		{"thinner than a device pixel at 1x", 0.4, 0.8, 1, 1, 1},
		{"a fraction rounded down", 0.65, 1.1, 2, 1, 2},
		{"a fraction rounded up", 1.2, 2.4, 2, 2, 5},
		// The face from the issue: a 16 px Cascadia Mono cell at 175 percent, whose heavy
		// stem is 4.24 times its light one and not twice its underline.
		{"a font whose heavy line is over twice its light one", 1.656, 3.312, 1.75, 3, 6},
	} {
		for _, glyph := range []struct {
			value string
			want  float32
		}{{"│", test.wantRule}, {"┃", test.wantHeavy}} {
			var canvas Canvas
			canvas.SetPixelScale(test.scale)
			canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{
				Advance: 0.6, Ascent: 0.8, Descent: 0.2, Rule: test.rule, HeavyRule: test.heavy,
			}})
			cellWidth, cellHeight := float32(10), float32(20)
			canvas.LabelRuns(glyph.value, Rect{Width: cellWidth, Height: cellHeight}, TextStyle{Size: 10}, cellWidth,
				[]TextRun{{Column: 0, Bytes: int32(len(glyph.value)), Cells: 1, Color: Color{Red: 1, Alpha: 1}}})
			texts := canvas.Texts()
			shapes := canvas.CellGlyphGeometry(CellGlyphShapes{}, texts[len(texts)-1])
			var strokes int
			for _, shape := range shapes.Rectangles {
				bounds := shape.Bounds
				if bounds.Height < cellHeight/2 || bounds.Width >= cellWidth/2 {
					continue // not a vertical stroke of the glyph
				}
				strokes++
				if got := bounds.Width * test.scale; math.Abs(float64(got-glyph.want)) > 1e-3 {
					t.Errorf("%s, %s at %gx: a vertical stroke is %g device pixels, want %g",
						test.name, glyph.value, test.scale, got, glyph.want)
				}
			}
			if strokes == 0 {
				t.Errorf("%s: %s draws no vertical stroke", test.name, glyph.value)
			}
		}
	}
}

// A face that does not report a stroke falls back to the reference weight, and one that
// reports a light line but no heavy one draws the heavy line twice as wide.
func TestCellGlyphRulesFallBackToTheReferenceWeight(t *testing.T) {
	for _, test := range []struct {
		name      string
		metrics   MonoCellMetrics
		wantRule  float32
		wantHeavy float32
	}{
		{"no measured strokes", MonoCellMetrics{Advance: 0.6, Ascent: 0.8, Descent: 0.2}, 1, 2},
		{"a light line only", MonoCellMetrics{Advance: 0.6, Ascent: 0.8, Descent: 0.2, Rule: 1}, 2, 4},
	} {
		for _, glyph := range []struct {
			value string
			want  float32
		}{{"│", test.wantRule}, {"┃", test.wantHeavy}} {
			var canvas Canvas
			canvas.SetPixelScale(2)
			canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: test.metrics})
			cellWidth, cellHeight := float32(10), float32(20)
			canvas.LabelRuns(glyph.value, Rect{Width: cellWidth, Height: cellHeight}, TextStyle{Size: 10}, cellWidth,
				[]TextRun{{Column: 0, Bytes: int32(len(glyph.value)), Cells: 1, Color: Color{Red: 1, Alpha: 1}}})
			texts := canvas.Texts()
			shapes := canvas.CellGlyphGeometry(CellGlyphShapes{}, texts[len(texts)-1])
			var strokes int
			for _, shape := range shapes.Rectangles {
				bounds := shape.Bounds
				if bounds.Height < cellHeight/2 || bounds.Width >= cellWidth/2 {
					continue
				}
				strokes++
				if got := bounds.Width * 2; math.Abs(float64(got-glyph.want)) > 1e-3 {
					t.Errorf("%s, %s: a vertical stroke is %g device pixels, want %g", test.name, glyph.value, got, glyph.want)
				}
			}
			if strokes == 0 {
				t.Errorf("%s: %s draws no vertical stroke", test.name, glyph.value)
			}
		}
	}
}

// runsOver covers value with one run per stretch between the byte offsets the cuts
// name, so a cut can fall anywhere, glyphs included.
func runsOver(value, cuts string) []TextRun {
	if len(value) == 0 {
		return []TextRun{{Bytes: 0}}
	}
	edges := make([]int, 0, len(cuts)+1)
	for _, cut := range []byte(cuts) {
		edges = append(edges, int(cut)%(len(value)+1))
	}
	slices.Sort(edges)
	runs := make([]TextRun, 0, len(edges)+1)
	start := 0
	for _, edge := range edges {
		if edge > start {
			runs = append(runs, TextRun{Bytes: int32(edge - start)})
			start = edge
		}
	}
	return append(runs, TextRun{Bytes: int32(len(value) - start)})
}
