package gfx

import "testing"

func TestRowNextAdvancesWithGap(t *testing.T) {
	row := BeginRow(Rect{X: 10, Y: 4, Width: 300, Height: 30}, 8)
	first := row.Next(100)
	second := row.Next(50)
	if first.X != 10 || first.Width != 100 || first.Height != 30 {
		t.Fatalf("first slot = %+v", first)
	}
	if second.X != 10+100+8 || second.Width != 50 {
		t.Fatalf("second slot = %+v, want x=118", second)
	}
}

func TestRowFlexConsumesRemainder(t *testing.T) {
	row := BeginRow(Rect{X: 0, Y: 0, Width: 200, Height: 30}, 8)
	row.Next(60)
	grown := row.NextFlex()
	if grown.X != 68 || grown.Width != 132 {
		t.Fatalf("flex slot = %+v, want x=68 width=132", grown)
	}
	if remaining := row.Remaining(); remaining != 0 {
		t.Fatalf("remaining after flex = %f", remaining)
	}
}

func TestRowEvenSplitsWithGaps(t *testing.T) {
	row := BeginRow(Rect{X: 0, Y: 0, Width: 316, Height: 30}, 8)
	slots := row.Even(4)
	if len(slots) != 4 {
		t.Fatalf("even returned %d slots", len(slots))
	}
	want := float32(316-8*3) / 4
	if slots[0].Width != want || slots[3].X != (want+8)*3 {
		t.Fatalf("even slots = %+v, want width %f", slots, want)
	}
}

func TestColumnNextAndFlex(t *testing.T) {
	column := BeginColumn(Rect{X: 5, Y: 10, Width: 100, Height: 200}, 12)
	first := column.Next(40)
	grown := column.NextFlex()
	if first.Y != 10 || first.Height != 40 || first.Width != 100 {
		t.Fatalf("first slot = %+v", first)
	}
	if grown.Y != 10+40+12 || grown.Height != 200-40-12 {
		t.Fatalf("flex slot = %+v", grown)
	}
}

func TestRectInsetClampsAtZero(t *testing.T) {
	inset := Rect{X: 10, Y: 10, Width: 20, Height: 20}.Inset(All(15))
	if inset.Width != 0 || inset.Height != 0 {
		t.Fatalf("over-inset rect = %+v", inset)
	}
	if inset.X != 25 || inset.Y != 25 {
		t.Fatalf("over-inset origin = %+v", inset)
	}
}

func TestRowZeroAllocation(t *testing.T) {
	row := BeginRow(Rect{Width: 300, Height: 30}, 8)
	allocs := testing.AllocsPerRun(100, func() {
		row.Next(50)
		row.NextFlex()
		row.Remaining()
	})
	if allocs != 0 {
		t.Fatalf("row layout allocated %f per iteration", allocs)
	}
}

func TestGridStretchesCellsToFillWidth(t *testing.T) {
	spec := GridSpec{MinCellWidth: 232, CellHeight: 124, Gap: 14}
	for _, width := range []float32{232, 480, 700, 1116, 1500, 2400} {
		grid := BeginGrid(Rect{X: 32, Y: 10, Width: width, Height: 400}, spec)
		columns := grid.Columns()
		var first, last Rect
		for index := 0; index < columns; index++ {
			cell := grid.Next()
			if index == 0 {
				first = cell
			}
			last = cell
			if cell.Width < spec.MinCellWidth-0.001 {
				t.Fatalf("width %v: cell %+v is narrower than the minimum", width, cell)
			}
		}
		if first.X != 32 {
			t.Fatalf("width %v: first cell starts at %v, want the left edge", width, first.X)
		}
		if right := last.X + last.Width; right < 32+width-0.01 || right > 32+width+0.01 {
			t.Fatalf("width %v: grid ends at %v, want %v: cells do not fill the container", width, right, 32+width)
		}
		if columns < int(width/(spec.MinCellWidth+spec.Gap)) {
			t.Fatalf("width %v: %d columns leave room for more", width, columns)
		}
	}
}

func TestGridColumnCountTracksWidth(t *testing.T) {
	spec := GridSpec{MinCellWidth: 232, CellHeight: 124, Gap: 14}
	for _, test := range []struct {
		width   float32
		columns int
	}{
		{-5, 1}, {0, 1}, {100, 1}, {232, 1}, {477, 1}, {478, 2}, {723, 2}, {724, 3}, {1116, 4}, {1230, 5},
	} {
		if got := spec.Columns(test.width); got != test.columns {
			t.Errorf("Columns(%v) = %d, want %d", test.width, got, test.columns)
		}
	}
}

func TestGridNarrowContainerShrinksInsteadOfOverflowing(t *testing.T) {
	grid := BeginGrid(Rect{Width: 120}, GridSpec{MinCellWidth: 232, CellHeight: 40, Gap: 14})
	cell := grid.Next()
	if grid.Columns() != 1 || cell.Width != 120 {
		t.Fatalf("narrow grid = %d columns, first cell %+v; want one cell of the container width", grid.Columns(), cell)
	}
}

func TestGridWrapsRowsAndMeasuresHeight(t *testing.T) {
	spec := GridSpec{MinCellWidth: 100, CellHeight: 50, Gap: 10}
	grid := BeginGrid(Rect{X: 5, Y: 7, Width: 230}, spec) // 2 columns of 110
	cells := [5]Rect{}
	for index := range cells {
		cells[index] = grid.Next()
	}
	if cells[2].X != 5 || cells[2].Y != 7+50+10 || cells[1].X != 5+110+10 {
		t.Fatalf("cells did not wrap with the gap: %+v", cells)
	}
	if rows := spec.Rows(230, 5); rows != 3 {
		t.Fatalf("Rows = %d, want 3", rows)
	}
	if got, want := spec.Height(230, 5), float32(3*50+2*10); got != want {
		t.Fatalf("Height = %v, want %v", got, want)
	}
	if spec.Height(230, 0) != 0 || spec.Rows(230, 0) != 0 {
		t.Fatal("an empty grid must have no height")
	}
	if bottom := cells[4].Y + cells[4].Height; bottom != 7+spec.Height(230, 5) {
		t.Fatalf("last cell bottom %v disagrees with Height", bottom)
	}
}

func TestGridZeroAllocation(t *testing.T) {
	spec := GridSpec{MinCellWidth: 232, CellHeight: 124, Gap: 14}
	allocs := testing.AllocsPerRun(100, func() {
		grid := BeginGrid(Rect{Width: 1116}, spec)
		for index := 0; index < 40; index++ {
			grid.Next()
		}
		spec.Height(1116, 40)
	})
	if allocs != 0 {
		t.Fatalf("grid layout allocated %f per iteration", allocs)
	}
}
