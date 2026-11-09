package platform

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

const testIcon = "" // three bytes, one cell

// iconAt is one icon run on a cell, and its three bytes of text.
func iconAt(column int32) gfx.TextRun { return gfx.TextRun{Column: column, Bytes: 3, Cells: 1} }

func textAt(column int32, text string) gfx.TextRun {
	return gfx.TextRun{Column: column, Bytes: int32(len(text)), Cells: int32(len(text))}
}

// Glyphs that stay inside their cells leave the columns exactly as GridColumns gave them.
func TestGridOwnerWithoutInkIsGridColumns(t *testing.T) {
	var owner GridOwner
	runs := []gfx.TextRun{textAt(0, "ab c"), iconAt(5), textAt(7, "de")}
	value := "ab c" + testIcon + "de"
	for _, pitch := range []float64{7, 8, 8.4, 10.25} {
		spans := owner.Plan(runs, value, pitch, 200, nil)
		for index, run := range runs {
			from, to := GridColumns(float64(run.Column)*pitch, float64(run.Cells)*pitch, 200)
			if spans[index] != (GridSpan{From: from, To: to}) {
				t.Errorf("pitch %v run %d owns %v, GridColumns gives %d,%d", pitch, index, spans[index], from, to)
			}
		}
		spans = owner.Plan(runs, value, pitch, 200, make([]GridInk, len(runs)))
		for index, run := range runs {
			from, to := GridColumns(float64(run.Column)*pitch, float64(run.Cells)*pitch, 200)
			if spans[index] != (GridSpan{From: from, To: to}) {
				t.Errorf("pitch %v run %d with no ink owns %v, GridColumns gives %d,%d", pitch, index, spans[index], from, to)
			}
		}
	}
}

// An icon drawn with the advance of one cell and the ink of one and a half owns the blank
// cell after it, as far as its ink goes, and the text after that cell keeps its columns.
func TestGridOwnerIconTakesTheBlankCellAfterIt(t *testing.T) {
	var owner GridOwner
	runs := []gfx.TextRun{iconAt(0), textAt(2, "ab")}
	value := testIcon + "ab"
	spans := owner.Plan(runs, value, 8, 100, []GridInk{{Right: 4}, {}})
	if spans[0].From != 0 || spans[0].To < 8+4 || spans[0].To > 8+5 {
		t.Errorf("the icon owns %v, want its cell and the 4 pixels of ink in the blank cell (and the filter's pixel)", spans[0])
	}
	if spans[1] != (GridSpan{From: 16, To: 32}) {
		t.Errorf("the text owns %v, want its own cells 16,32", spans[1])
	}
}

// The ink is cut at the edge of a cell that holds a character: the neighbor's glyph is not
// painted over.
func TestGridOwnerIconDoesNotTakeAnOccupiedCell(t *testing.T) {
	var owner GridOwner
	runs := []gfx.TextRun{iconAt(0), textAt(1, "ab")}
	spans := owner.Plan(runs, testIcon+"ab", 8, 100, []GridInk{{Right: 6}, {}})
	if spans[0] != (GridSpan{From: 0, To: 8}) || spans[1] != (GridSpan{From: 8, To: 24}) {
		t.Errorf("icon %v, text %v: an occupied cell was taken", spans[0], spans[1])
	}
}

// The blank cell the terminal folds into the run before it is blank all the same: an arrow
// whose ink reaches left takes the end of it, and the run before keeps the rest.
func TestGridOwnerTakesBlankCellsFoldedIntoTheRunBefore(t *testing.T) {
	var owner GridOwner
	runs := []gfx.TextRun{textAt(0, "ab  "), iconAt(4)}
	spans := owner.Plan(runs, "ab  "+testIcon, 8, 100, []GridInk{{}, {Left: 5}})
	if spans[1].To != 40 || spans[1].From > 32-5 || spans[1].From < 32-6 {
		t.Errorf("the arrow owns %v, want its cell 32,40 and 5 pixels (and the filter's) to the left", spans[1])
	}
	if spans[0].From != 0 || spans[0].To != spans[1].From {
		t.Errorf("the text owns %v, want the columns up to the arrow's %d", spans[0], spans[1].From)
	}
	// "ab" itself is not given away, however far the ink reaches.
	spans = owner.Plan(runs, "ab  "+testIcon, 8, 100, []GridInk{{}, {Left: 60}})
	if spans[0].From != 0 || spans[0].To < 16 {
		t.Errorf("the text owns %v, but the characters 'ab' are in columns 0..16", spans[0])
	}
}

// Two glyphs that both reach into one blank cell do not own it twice: the left one
// takes the columns it reaches, the right one what is left of its own reach.
func TestGridOwnerSharesABlankCellBetweenTwoIcons(t *testing.T) {
	var owner GridOwner
	runs := []gfx.TextRun{iconAt(0), iconAt(2)}
	spans := owner.Plan(runs, testIcon+testIcon, 8, 100, []GridInk{{Right: 3}, {Left: 3}})
	if spans[0].From != 0 || spans[1].To != 24 {
		t.Fatalf("icons own %v and %v", spans[0], spans[1])
	}
	if spans[0].To > spans[1].From {
		t.Errorf("icons own %v and %v: they overlap", spans[0], spans[1])
	}
	if spans[0].To <= 8 || spans[1].From >= 16 {
		t.Errorf("icons own %v and %v: one of them got nothing of the blank cell", spans[0], spans[1])
	}
	spans = owner.Plan(runs, testIcon+testIcon, 8, 100, []GridInk{{Right: 7}, {Left: 7}})
	if spans[0].To > spans[1].From || spans[0].From != 0 || spans[1].To != 24 {
		t.Errorf("icons that both reach all of the cell own %v and %v", spans[0], spans[1])
	}
}

// Ink past the end of the line, or of the texture, takes what is there and no more.
func TestGridOwnerStaysInsideTheTexture(t *testing.T) {
	var owner GridOwner
	spans := owner.Plan([]gfx.TextRun{iconAt(0), iconAt(3)}, testIcon+testIcon, 8, 30, []GridInk{{Left: 9}, {Right: 100}})
	if spans[0].From != 0 || spans[1].To != 30 {
		t.Errorf("icons own %v and %v of a 30 column texture", spans[0], spans[1])
	}
	spans = owner.Plan([]gfx.TextRun{iconAt(1)}, testIcon, 8, 100, []GridInk{{Left: 1000, Right: 1000}})
	if spans[0] != (GridSpan{From: 0, To: 100}) {
		t.Errorf("an icon with huge ink owns %v of an empty 100 column texture", spans[0])
	}
}

// Whatever the cells and the ink, the partition keeps its three promises: no column has
// two owners, a run keeps its own cells, and a column is only ever taken from a blank one.
func TestGridOwnerPartitionInvariants(t *testing.T) {
	random := rand.New(rand.NewSource(503))
	var owner GridOwner
	for round := 0; round < 3000; round++ {
		pitch := []float64{6, 7, 8, 9, 10, 12}[random.Intn(6)]
		cells := 1 + random.Intn(30)
		var runs []gfx.TextRun
		var icons []bool
		var value strings.Builder
		blank := make([]bool, cells)
		for cell := 0; cell < cells; {
			switch kind := random.Intn(4); kind {
			case 0: // a gap
				blank[cell] = true
				cell++
			case 1: // an icon
				runs, icons = append(runs, iconAt(int32(cell))), append(icons, true)
				value.WriteString(testIcon)
				cell++
			default: // ASCII, blanks folded in
				length := 1 + random.Intn(min(4, cells-cell))
				text := make([]byte, length)
				for i := range text {
					text[i] = "ab  "[random.Intn(4)]
					blank[cell+i] = text[i] == ' '
				}
				runs, icons = append(runs, textAt(int32(cell), string(text))), append(icons, false)
				value.Write(text)
				cell += length
			}
		}
		ink := make([]GridInk, len(runs))
		for index := range runs {
			if icons[index] && random.Intn(3) > 0 {
				ink[index] = GridInk{Left: random.Float32() * 20, Right: random.Float32() * 20}
			}
		}
		width := cells*int(pitch) + random.Intn(2*int(pitch))
		spans := owner.Plan(runs, value.String(), pitch, width, ink)
		// A cell's columns, as the plan sees them.
		blankColumn := func(column int) bool {
			for cell := range blank {
				from, to := GridColumns(float64(cell)*pitch, pitch, width)
				if column >= from && column < to {
					return blank[cell]
				}
			}
			return true // beyond the cells
		}
		taken := make([]int, width)
		for index, run := range runs {
			span := spans[index]
			if span.From < 0 || span.To > width || span.To < span.From {
				t.Fatalf("round %d: run %d owns %v of %d columns", round, index, span, width)
			}
			for column := span.From; column < span.To; column++ {
				taken[column]++
			}
			from, to := GridColumns(float64(run.Column)*pitch, float64(run.Cells)*pitch, width)
			if icons[index] {
				if span.From > from || span.To < to {
					t.Fatalf("round %d: the icon of run %d owns %v, less than its cells %d,%d", round, index, span, from, to)
				}
				for column := span.From; column < from; column++ {
					if !blankColumn(column) {
						t.Fatalf("round %d: the icon of run %d took column %d, which a character is in", round, index, column)
					}
				}
				for column := to; column < span.To; column++ {
					if !blankColumn(column) {
						t.Fatalf("round %d: the icon of run %d took column %d, which a character is in", round, index, column)
					}
				}
				continue
			}
			// Text only gives away blank columns.
			for column := from; column < to; column++ {
				if (column < span.From || column >= span.To) && !blankColumn(column) {
					t.Fatalf("round %d: the text of run %d lost column %d, which a character is in", round, index, column)
				}
			}
			if span.To > span.From && (span.From < from || span.To > to) {
				t.Fatalf("round %d: the text of run %d owns %v, more than its cells %d,%d", round, index, span, from, to)
			}
		}
		for column, owners := range taken {
			if owners > 1 {
				t.Fatalf("round %d: column %d has %d owners", round, column, owners)
			}
		}
	}
}

func BenchmarkGridOwnerPlan(b *testing.B) {
	var value strings.Builder
	var runs []gfx.TextRun
	var ink []GridInk
	for cell := 0; cell < 200; cell += 10 {
		runs = append(runs, textAt(int32(cell), "abc def "), iconAt(int32(cell+8)))
		value.WriteString("abc def " + testIcon)
		ink = append(ink, GridInk{}, GridInk{Left: 1, Right: 5})
	}
	var owner GridOwner
	b.ReportAllocs()
	for b.Loop() {
		owner.Plan(runs, value.String(), 8, 1400, ink)
	}
}
