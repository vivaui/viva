package platform

import "testing"

// Neighbouring runs of a line must never own the same column: it would be drawn twice, and a
// glyph that fills its cell would paint the shared column in the second run's color.
func TestGridColumnsOfNeighbouringRunsDoNotOverlap(t *testing.T) {
	for _, pitch := range []float64{7, 8, 8.4, 9.75, 10.000001, 11.999999, 12.5, 15.6} {
		const width = 4000
		previous := 0
		for column := 0; column < 200; column++ {
			from, to := GridColumns(float64(column)*pitch, pitch, width)
			if from != previous && column > 0 {
				t.Fatalf("pitch %v: cell %d starts at column %d, the one before it ended at %d", pitch, column, from, previous)
			}
			if to <= from {
				t.Fatalf("pitch %v: cell %d owns no column", pitch, column)
			}
			previous = to
		}
	}
}

func TestGridColumnsOfARunSpanCells(t *testing.T) {
	from, to := GridColumns(3*8, 5*8, 400)
	if from != 24 || to != 64 {
		t.Fatalf("five cells from column 3 = %d,%d, want 24,64", from, to)
	}
}

func TestGridColumnsStayInsideTheTexture(t *testing.T) {
	if from, to := GridColumns(-3, 10, 20); from != 0 || to != 7 {
		t.Errorf("a run left of the texture = %d,%d, want 0,7", from, to)
	}
	if from, to := GridColumns(15, 10, 20); from != 15 || to != 20 {
		t.Errorf("a run past its right edge = %d,%d, want 15,20", from, to)
	}
	if from, to := GridColumns(30, 10, 20); from != to {
		t.Errorf("a run beyond the texture owns columns %d,%d", from, to)
	}
}

func TestGridColumnsOfAnEmptyRunAreEmpty(t *testing.T) {
	if from, to := GridColumns(40, 0, 100); from != to {
		t.Errorf("a run without cells = %d,%d", from, to)
	}
}
