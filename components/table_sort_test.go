package components

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTableSortStateAlternatesAndStartsNewColumnsAscending(t *testing.T) {
	var state TableSortState
	for _, step := range []struct {
		column int
		want   TableSortDirection
	}{{2, TableSortAscending}, {2, TableSortDescending}, {2, TableSortAscending}, {0, TableSortAscending}, {0, TableSortDescending}} {
		state = state.Toggle(step.column)
		if state.Column != step.column || state.Direction != step.want {
			t.Fatalf("column %d: got %+v, want %v", step.column, state, step.want)
		}
	}
	if state.Toggle(-1) != state {
		t.Fatal("an invalid column changed the ordering")
	}
}

func TestSortableTableHeaderClipsAndExposesStateWithoutChangingRows(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			var canvas gfx.Canvas
			canvas.SetPixelScale(scale)
			canvas.Clip(gfx.Rect{Width: 175, Height: 150})
			columns := [...]TableColumn{{Width: 90}, {Width: 90, Alignment: TableAlignRight}, {}}
			table := BeginTable(&canvas, gfx.LightTheme(), gfx.Rect{X: 5, Y: 12, Width: 180, Height: 100}, columns[:])
			table.SortableHeader([]TableHeader{
				{ID: gfx.Item(1), Label: "Name", Sort: TableSortAscending, Description: "已按名称升序"},
				{ID: gfx.Item(2), Label: "Long column heading", Sort: TableSortDescending, Disabled: true, FocusableWhenDisabled: true},
				{Label: "Static"},
			})
			table.Header([]string{"ignored"})
			table.SortableHeader([]TableHeader{{ID: gfx.Item(3), Label: "ignored"}})
			table.Row(gfx.Item(4), []string{"selected", "10", "x"}, true)
			hits := canvas.Hits()
			if len(hits) != 3 || hits[0].ID != gfx.Item(1) || hits[0].Description != "已按名称升序" || hits[0].Role != gfx.RoleButton || !hits[1].Disabled || !hits[1].FocusableDisabled {
				t.Fatalf("header semantics: %+v", hits)
			}
			if hits[1].Bounds.X+hits[1].Bounds.Width > 175 || hits[2].Bounds.Y != 12+TableHeaderHeight(gfx.LightTheme()) {
				t.Fatal("sortable headers escaped clipping or advanced the body more than once")
			}
			for _, text := range canvas.Texts() {
				if text.Value == "ignored" {
					t.Fatal("a repeated header declaration painted a second header")
				}
				if text.Value == "Name" && text.Bounds.Y+text.Bounds.Height/2 != hits[0].Bounds.Y+hits[0].Bounds.Height/2 {
					t.Fatal("header text is not vertically centered")
				}
			}
		})
	}
}

func TestSortableTableHeaderPaintingAllocatesNothingAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	columns := [...]TableColumn{{Width: 160}, {Width: 120, Alignment: TableAlignRight}, {}}
	headers := [...]TableHeader{{ID: gfx.Item(1), Label: "Name"}, {ID: gfx.Item(2), Label: "Size", Sort: TableSortDescending}, {Label: "Notes"}}
	paint := func() {
		canvas.Reset()
		table := BeginTable(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 480, Height: 100}, columns[:])
		table.SortableHeader(headers[:])
	}
	paint()
	if got := testing.AllocsPerRun(100, paint); got != 0 {
		t.Fatalf("sortable header allocated %v times per paint", got)
	}
}

func BenchmarkTableSortableHeader(b *testing.B) {
	var canvas gfx.Canvas
	columns := [...]TableColumn{{Width: 160}, {Width: 120, Alignment: TableAlignRight}, {}}
	headers := [...]TableHeader{{ID: gfx.Item(1), Label: "Name"}, {ID: gfx.Item(2), Label: "Size", Sort: TableSortDescending}, {Label: "Notes"}}
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		table := BeginTable(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 480, Height: 100}, columns[:])
		table.SortableHeader(headers[:])
	}
}

// A header cell's hover background must sit under the row's divider rather than
// paint it out, and a heading without an ID still paints like an ordinary one.
func TestSortableTableHeaderKeepsItsRuleAndStaticCell(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	columns := [...]TableColumn{{Width: 90}, {Width: 90}, {Width: 90}}
	table := BeginTable(&canvas, theme, gfx.Rect{Width: 270, Height: 100}, columns[:])
	var zero gfx.ID
	canvas.SetInteraction(gfx.Item(1), zero, zero, false)
	table.SortableHeader([]TableHeader{{ID: gfx.Item(1), Label: "Name"}, {ID: gfx.Item(2), Label: "Size"}, {Label: "Notes"}})

	var static bool
	for _, text := range canvas.Texts() {
		static = static || text.Value == "Notes"
	}
	if !static {
		t.Fatal("a header cell without an ID did not paint its label")
	}

	ruleY := TableHeaderHeight(theme) - 1
	var topmost gfx.Rectangle
	hovered := false
	for _, rectangle := range canvas.Rectangles() {
		if rectangle.Bounds.Y <= ruleY && ruleY < rectangle.Bounds.Y+rectangle.Bounds.Height {
			topmost = rectangle
		}
		hovered = hovered || rectangle.Color == theme.Hover
	}
	if !hovered {
		t.Fatal("hovering a sortable header painted no background")
	}
	if topmost.Color != theme.Border || topmost.Bounds.Height != 1 {
		t.Fatalf("header rule is covered by %+v, want the border across its last pixel", topmost.Bounds)
	}
}
