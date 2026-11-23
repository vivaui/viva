package components

import (
	"math/rand"
	"slices"
	"testing"
)

// TestCodeEditorRefitMatchesFullLayout edits text at random and checks that laying
// out only the edited lines gives the rows a layout of the whole text gives.
func TestCodeEditorRefitMatchesFullLayout(t *testing.T) {
	pieces := []string{"a", "bc ", "\n", "\r\n", "\t", "中文", "é", " word ", "\n\n", "x\ty"}
	random := rand.New(rand.NewSource(1))
	reps := []CodeRepresentation{{Character: ' ', Label: "·"}}
	for _, wrap := range []bool{false, true} {
		for _, represented := range []bool{false, true} {
			var live CodeEditor
			value := ""
			var layoutReps []CodeRepresentation
			if represented {
				layoutReps = reps
			}
			for step := 0; step < 2000; step++ {
				start := random.Intn(len(value) + 1)
				end := min(start+random.Intn(6), len(value))
				insert := ""
				for range random.Intn(4) {
					insert += pieces[random.Intn(len(pieces))]
				}
				if represented && random.Intn(5) == 0 {
					insert += " "
				}
				value = value[:start] + insert + value[end:]
				live.fitWithRepresentations(value, 7, wrap, 4, nil, layoutReps)
				live.measureIndents()
				var fresh CodeEditor
				fresh.fitWithRepresentations(value, 7, wrap, 4, nil, layoutReps)
				fresh.measureIndents()
				if !slices.Equal(live.indents, fresh.indents) {
					t.Fatalf("wrap=%v step %d: indents %v, want %v for %q", wrap, step, live.indents, fresh.indents, value)
				}
				if len(live.rows) != len(fresh.rows) || live.widest != fresh.widest || live.lineCount != fresh.lineCount {
					t.Fatalf("wrap=%v step %d: %d rows widest %d lines %d, want %d rows widest %d lines %d for %q",
						wrap, step, len(live.rows), live.widest, live.lineCount, len(fresh.rows), fresh.widest, fresh.lineCount, value)
				}
				for index := range fresh.rows {
					got, want := live.rows[index], fresh.rows[index]
					if got.start != want.start || got.end != want.end || got.lineStart != want.lineStart || got.line != want.line || got.cells != want.cells || live.rowText(got) != fresh.rowText(want) {
						t.Fatalf("wrap=%v step %d: row %d is %+v %q, want %+v %q", wrap, step, index, got, live.rowText(got), want, fresh.rowText(want))
					}
				}
				if len(value) > 400 {
					value = value[len(value)/2:]
				}
			}
		}
	}
}
