package base_test

import (
	"testing"

	. "github.com/vivaui/viva/base"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
)

// FitHeadings grows each fixed column narrower than its heading to the width
// the heading needs, and leaves a flexible column, a column without a heading,
// and the caller's own slice alone.
func TestTableFitHeadingsGrowsFixedColumnsToTheirHeadings(t *testing.T) {
	theme := gfx.LightTheme()
	columns := []style.TableColumn{
		{Width: 10}, {Width: 10}, {Width: 10},
		{Alignment: style.TableAlignRight},
	}
	headings := []string{"A long heading", "B", "", "D"}
	source := TableRows{{"a", "b", "c", "d"}}
	var state TableState
	var view View
	view.Draw(testCanvas(), theme, gfx.Rect{Width: 500, Height: 200}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			f.Table(&state, TableSpec{
				Rows:        tableScope,
				Columns:     titledColumns(columns, headings, nil),
				Source:      &source,
				FitHeadings: true,
			})
		})
	})
	for index, want := range []float32{10, 10, 10, 0} {
		if got := columns[index].Width; got != want {
			t.Errorf("the caller's column %d is %v wide, want %v", index, got, want)
		}
	}
	first, firstOK := state.CellRect(0, 0)
	second, secondOK := state.CellRect(0, 1)
	third, thirdOK := state.CellRect(0, 2)
	fourth, fourthOK := state.CellRect(0, 3)
	if !firstOK || !secondOK || !thirdOK || !fourthOK {
		t.Fatalf("painted cells: %v %v %v %v", firstOK, secondOK, thirdOK, fourthOK)
	}
	if first.Width <= 10 || second.Width <= 10 {
		t.Errorf("columns narrower than their headings stayed %v and %v wide", first.Width, second.Width)
	}
	if third.Width != 10 {
		t.Errorf("a column without a heading is %v wide, want its declared 10", third.Width)
	}
	if fourth.Width <= 0 {
		t.Errorf("the flexible last column is %v wide", fourth.Width)
	}
}

// A warm table that fits its headings does not allocate: the fitted columns are
// kept beside the declaration and reused.
func TestTableFitHeadingsAllocatesNothingOnceWarm(t *testing.T) {
	theme := gfx.LightTheme()
	columns := []style.TableColumn{{Width: 12}, {Width: 12}, {Width: 0}}
	headings := []string{"Long heading", "B", "C"}
	titled := titledColumns(columns, headings, nil)
	source := TableRows{{"a", "b", "c"}}
	var state TableState
	var view View
	canvas := testCanvas()
	run := func() {
		canvas.Reset()
		view.Draw(canvas, theme, gfx.Rect{Width: 400, Height: 200}, func(f *Frame) {
			f.Column().Fill().Children(func() {
				f.Table(&state, TableSpec{
					Rows:        tableScope,
					Columns:     titled,
					Source:      &source,
					FitHeadings: true,
				})
			})
		})
	}
	run()
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a warm table that fits its headings allocates %v times a frame, want 0", allocs)
	}
}

// A table none of whose columns has a width shares its own width between them, and
// leaves the columns the application declared as it wrote them.
func TestTableSharedColumnsLeaveTheDeclaredOnesAlone(t *testing.T) {
	columns := []style.TableColumn{{Title: "A"}, {Title: "B"}, {Title: "C"}}
	source := TableRows{{"a", "b", "c"}}
	var state TableState
	var view View
	canvas := testCanvas()
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 300, Height: 200}, func(f *Frame) {
		f.Column().Fill().Children(func() {
			f.Table(&state, TableSpec{Rows: tableScope, Columns: columns, Source: &source})
		})
	})
	for i, column := range columns {
		if column.Width != 0 {
			t.Fatalf("column %d was given a width of %v in the application's slice", i, column.Width)
		}
	}
	cells := map[float32]bool{}
	for _, text := range canvas.Texts() {
		if text.Value == "a" || text.Value == "b" || text.Value == "c" {
			cells[text.Bounds.X] = true
		}
	}
	if len(cells) != 3 {
		t.Fatalf("the three cells of the row start at %d distinct positions, want 3", len(cells))
	}
}
