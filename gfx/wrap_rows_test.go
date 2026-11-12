package gfx

import (
	"strings"
	"testing"
)

// The rows of a text are the lines WrapLines draws, with the spaces it drops
// kept on the row they hang from, so that every byte but a newline is on exactly
// one row: that is what lets an editor map any caret offset to a place.
func TestWrapRowsAgreeWithWrapLinesAndCoverTheText(t *testing.T) {
	canvas := wrapper()
	alphabet := []rune("ab c  \n\r主机，。x-/:")
	random := newTestRandom(11)
	for round := 0; round < 3000; round++ {
		runes := make([]rune, random(48))
		for index := range runes {
			runes[index] = alphabet[random(len(alphabet))]
		}
		value := string(runes)
		width := float32(1 + random(12))
		rows := canvas.WrapRows(nil, value, 1, 0, width)
		lines := canvas.WrapLines(nil, value, 1, 0, width, 1000)

		var shown []string
		position := 0
		for index, row := range rows {
			if row.Start < position || row.End < row.Start || row.End > len(value) {
				t.Fatalf("%q at %v: row %d %+v after %d", value, width, index, row, position)
			}
			// What lies between two rows is a line break or nothing.
			if gap := value[position:row.Start]; gap != "" && gap != "\n" && gap != "\r\n" {
				t.Fatalf("%q at %v: row %d leaves %q out", value, width, index, gap)
			}
			shown = append(shown, strings.TrimRight(value[row.Start:row.End], " "))
			position = row.End
		}
		if tail := value[position:]; tail != "" && tail != "\r" {
			t.Fatalf("%q at %v: %q is on no row", value, width, tail)
		}
		// A text that ends in a newline, or is empty, ends with an empty row for the
		// caret to sit on, which WrapLines has no line for.
		if last := shown[len(shown)-1]; last == "" && (value == "" || strings.HasSuffix(value, "\n")) {
			shown = shown[:len(shown)-1]
		}
		if strings.Join(shown, "|") != strings.Join(lines, "|") || len(shown) != len(lines) {
			t.Fatalf("%q at %v: rows %q, WrapLines %q", value, width, shown, lines)
		}
	}
}

func TestWrapRowKeepsSpacesHangingAndEmptyLines(t *testing.T) {
	canvas := wrapper()
	value := "one two  three\n\nlast\n"
	rows := canvas.WrapRows(nil, value, 1, 0, 8)
	var got []string
	for _, row := range rows {
		got = append(got, value[row.Start:row.End])
	}
	want := []string{"one two  ", "three", "", "last", ""}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("rows = %q, want %q", got, want)
	}
	if rows := canvas.WrapRows(nil, "a long unwrapped paragraph\nnext", 1, 0, 0); len(rows) != 2 {
		t.Fatalf("a width of zero wrapped: %+v", rows)
	}
}

func TestWrapRowsAllocateNothingWithRoom(t *testing.T) {
	canvas := wrapper()
	rows := make([]TextRow, 0, 16)
	const text = "Connection failed: connect to 10.255.255.1:22 dial tcp i/o timeout\nsecond paragraph here"
	if allocations := testing.AllocsPerRun(100, func() { rows = canvas.WrapRows(rows[:0], text, 1, 0, 30) }); allocations != 0 {
		t.Fatalf("WrapRows allocations = %v", allocations)
	}
}

func BenchmarkWrapRows(b *testing.B) {
	canvas := wrapper()
	rows := make([]TextRow, 0, 16)
	text := "Connection failed: connect to 10.255.255.1:22: dial tcp 10.255.255.1:22: i/o timeout"
	b.ReportAllocs()
	for range b.N {
		rows = canvas.WrapRows(rows[:0], text, 1, 0, 30)
	}
}
