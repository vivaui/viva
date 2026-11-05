package platform

import (
	"math"

	"github.com/vivaui/viva/ui"
)

// GridInk is how far the ink of one run's glyphs reaches past the cells of the run, in
// device pixels, to the left and to the right. A glyph that stays inside its cells has none.
// Nerd Font icons do not: they have the advance of one cell and ink up to half a cell
// wider, drawn over the blank cell the prompt leaves after them.
type GridInk struct{ Left, Right float32 }

// GridSpan is the device columns [From, To) of a label's texture that one run owns.
type GridSpan struct{ From, To int }

// GridOwner decides which run owns each device column of a fixed-pitch label.
//
// The cells of a run own the columns GridColumns gives them, and no column belongs to
// two runs: a column drawn for two runs would be painted twice, once in each color
// (t8y2/motty#279). A run whose ink reaches past its cells also owns the columns it
// reaches, where the cells beside it are blank (a space, or no run at all), which is
// what lets an icon, drawn with the advance of one cell and the ink of one and a half,
// show whole in its own color (t8y2/motty#503). The cells beside it that hold any other
// character are never taken: the ink is cut at the cell edge there.
//
// A GridOwner keeps its buffers between labels, so planning allocates nothing once they
// have grown to the widest label.
type GridOwner struct {
	owner []int32 // run that owns each device column, -1 for none
	blank []bool  // whether a column may be taken: a blank cell's, or no run's
	base  []GridSpan
	spans []GridSpan
}

// extraColumns is the part of a pixel the ClearType filter spreads a glyph's ink past
// its outline, in pixels: one subpixel on each side.
const extraColumns = 1.0 / 3

// Plan returns the columns each run owns, one entry per run (empty for a run that
// holds no cell), in a label of width device columns whose cells are pitch device pixels
// wide. value is the label's text, which the runs split in order, and ink the reach of
// each run's glyphs, one entry per run or nil when no glyph reaches out. The result is
// valid until the next call.
func (owner *GridOwner) Plan(runs []ui.TextRun, value string, pitch float64, width int, ink []GridInk) []GridSpan {
	owner.owner = growInt32(owner.owner, width)
	owner.blank = growBool(owner.blank, width)
	if cap(owner.spans) < len(runs) {
		owner.spans = make([]GridSpan, len(runs))
		owner.base = make([]GridSpan, len(runs))
	}
	owner.spans, owner.base = owner.spans[:len(runs)], owner.base[:len(runs)]
	for column := range owner.owner {
		owner.owner[column], owner.blank[column] = -1, true
	}
	offset := 0
	for index, run := range runs {
		end := offset + int(run.Bytes)
		if run.Bytes < 0 || end > len(value) {
			end = offset
		}
		text := value[offset:end]
		offset = end
		owner.base[index] = GridSpan{}
		if run.Cells <= 0 {
			continue
		}
		left, span := float64(run.Column)*pitch, float64(run.Cells)*pitch
		from, to := GridColumns(left, span, width)
		owner.base[index] = GridSpan{From: from, To: to}
		for column := from; column < to; column++ {
			owner.owner[column], owner.blank[column] = int32(index), false
		}
		if len(text) != int(run.Cells) {
			continue
		}
		// One byte per cell: the cells of the run are the characters of its text.
		for cell := 0; cell < len(text); cell++ {
			if text[cell] != ' ' {
				continue
			}
			from, to := GridColumns(left+float64(cell)*pitch, pitch, width)
			for column := from; column < to; column++ {
				owner.blank[column] = true
			}
		}
	}
	if len(ink) > 0 {
		// Rightward first, from the left, so that where two runs reach into one blank
		// cell the left one has the columns next to it, and the right one the rest.
		for index, run := range runs {
			if reach := columnsReached(ink, index, run, false); reach > 0 {
				to := owner.base[index].To
				owner.take(int32(index), to, to+reach, 1)
			}
		}
		for index := len(runs) - 1; index >= 0; index-- {
			if reach := columnsReached(ink, index, runs[index], true); reach > 0 {
				from := owner.base[index].From
				owner.take(int32(index), from-1, from-1-reach, -1)
			}
		}
	}
	for index := range owner.spans {
		owner.spans[index] = GridSpan{}
	}
	for column, run := range owner.owner {
		if run < 0 {
			continue
		}
		span := &owner.spans[run]
		if span.To == 0 {
			span.From = column
		}
		span.To = column + 1
	}
	return owner.spans
}

// columnsReached is the whole columns the ink of a run reaches out on one side: the
// reach and the part of a pixel the ClearType filter adds, rounded up.
func columnsReached(ink []GridInk, index int, run ui.TextRun, left bool) int {
	if index >= len(ink) || run.Cells <= 0 {
		return 0
	}
	reach := ink[index].Right
	if left {
		reach = ink[index].Left
	}
	if !(reach > 0) || math.IsInf(float64(reach), 0) {
		return 0
	}
	return int(math.Ceil(float64(reach) + extraColumns))
}

// take gives run the columns from first toward limit, one step at a time, up to the first
// column that is not blank or already taken.
func (owner *GridOwner) take(run int32, first, limit, step int) {
	for column := first; column != limit; column += step {
		if column < 0 || column >= len(owner.owner) || !owner.blank[column] {
			return
		}
		owner.owner[column], owner.blank[column] = run, false
	}
}

func growInt32(buffer []int32, n int) []int32 {
	if cap(buffer) < n {
		return make([]int32, n)
	}
	return buffer[:n]
}

func growBool(buffer []bool, n int) []bool {
	if cap(buffer) < n {
		return make([]bool, n)
	}
	return buffer[:n]
}
