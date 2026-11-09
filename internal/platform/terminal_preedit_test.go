package platform

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTerminalPreeditRunsPlaceCharactersOnCells(t *testing.T) {
	color := gfx.Color{Red: 1, Alpha: 1}
	for _, test := range []struct {
		name  string
		value string
		want  []gfx.TextRun
	}{
		{"pinyin shares one run", "ni'hao", []gfx.TextRun{{Column: 0, Bytes: 6, Cells: 6}}},
		{"wide characters take two cells each", "你好", []gfx.TextRun{{Column: 0, Bytes: 3, Cells: 2}, {Column: 2, Bytes: 3, Cells: 2}}},
		{"mixed", "ni好", []gfx.TextRun{{Column: 0, Bytes: 2, Cells: 2}, {Column: 2, Bytes: 3, Cells: 2}}},
		{"a narrow non-ASCII letter is on its own cell", "lü", []gfx.TextRun{{Column: 0, Bytes: 1, Cells: 1}, {Column: 1, Bytes: 2, Cells: 1}}},
		{"a combining mark stays with its base", "éx", []gfx.TextRun{{Column: 0, Bytes: 3, Cells: 1}, {Column: 1, Bytes: 1, Cells: 1}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			for index := range test.want {
				test.want[index].Color = color
			}
			if got := TerminalPreeditRuns(nil, test.value, color); !slices.Equal(got, test.want) {
				t.Fatalf("runs = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestTerminalPreeditColumnCountsCells(t *testing.T) {
	const value = "ni好éx"
	for runes, want := range []int{0, 1, 2, 4, 5, 5, 6} {
		if got := TerminalPreeditColumn(value, runes); got != want {
			t.Errorf("column before rune %d = %d, want %d", runes, got, want)
		}
	}
	if got := TerminalPreeditColumn(value, 99); got != 6 {
		t.Errorf("column past the end = %d, want the width 6", got)
	}
}
