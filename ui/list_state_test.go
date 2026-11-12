package ui

import "testing"

// resize grows and shrinks incrementally; whatever it does must leave the
// tree agreeing with the heights it holds. This walks a list through grows,
// shrinks and measurements and checks the totals against a plain sum.
func TestListResizeKeepsTheTreeConsistent(t *testing.T) {
	state := ListState{}
	// The heights a measurement pass would report: positive once measured.
	measure := func(from, to int, h float32) {
		for i := from; i < to && i < len(state.heights); i++ {
			state.measured(i, h+float32(i%3))
		}
	}
	compare := func() {
		t.Helper()
		var wantTotal float64
		var wantCount int
		for _, h := range state.heights {
			if h > 0 {
				wantTotal += float64(h)
				wantCount++
			}
		}
		if state.total != wantTotal || state.count != wantCount {
			t.Fatalf("after resize to %d rows: total %v count %d, want %v and %d",
				len(state.heights), state.total, state.count, wantTotal, wantCount)
		}
		if prefix, measured := state.tree.prefix(len(state.heights)); prefix != wantTotal || int(measured) != wantCount {
			t.Fatalf("tree after resize to %d rows: prefix %v measured %d, want %v and %d",
				len(state.heights), prefix, measured, wantTotal, wantCount)
		}
	}

	state.resize(100)
	measure(0, 100, 20)
	compare()
	for range 3 {
		state.resize(len(state.heights) + 40)
		measure(len(state.heights)-40, len(state.heights), 20)
		compare()
		state.resize(len(state.heights) - 25)
		compare()
		state.resize(len(state.heights) + 60)
		measure(len(state.heights)-60, len(state.heights), 20)
		compare()
	}
	if len(state.heights) != 100+3*(40-25+60) {
		t.Fatalf("the walk left %d rows", len(state.heights))
	}
}
