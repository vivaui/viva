package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTreePaintsStableHierarchyAndSemantics(t *testing.T) {
	var canvas gfx.Canvas
	tree := BeginTree(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 320, Height: 240}).RowHeight(40).Indent(22)
	branch := tree.Item(TreeItem{ID: gfx.Item(1), DisclosureID: gfx.Item(2), Label: "Production", Branch: true, Expanded: true, Selected: true})
	leaf := tree.Item(TreeItem{ID: gfx.Item(3), Label: "Gateway", Depth: 1})
	if branch.Bounds.Y != 20 || leaf.Bounds.Y != 60 || leaf.Content.X != 32 {
		t.Fatalf("tree geometry branch=%+v leaf=%+v", branch, leaf)
	}
	if len(canvas.Hits()) != 3 {
		t.Fatalf("hits = %+v", canvas.Hits())
	}
	if disclosure := canvas.Hits()[0]; disclosure.ID != gfx.Item(2) || !disclosure.Expanded || disclosure.Role != gfx.RoleButton {
		t.Fatalf("disclosure semantics = %+v", disclosure)
	}
	if row := canvas.Hits()[1]; row.ID != gfx.Item(1) || !row.Selected || row.Bounds.X <= branch.Bounds.X {
		t.Fatalf("branch row semantics = %+v", row)
	}
	if row := canvas.Hits()[2]; row.ID != gfx.Item(3) || row.Selected {
		t.Fatalf("leaf row semantics = %+v", row)
	}
}

func TestTreeDisabledDisclosureAndSteadyPaintAreAllocationFree(t *testing.T) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	paint := func() {
		canvas.Reset()
		tree := BeginTree(&canvas, theme, gfx.Rect{Width: 300, Height: 200})
		tree.Item(TreeItem{ID: gfx.Item(11), DisclosureID: gfx.Item(12), Label: "Hidden", Branch: true, Disabled: true})
		tree.Item(TreeItem{ID: gfx.Item(13), Label: "Child", Depth: 1})
	}
	paint()
	if len(canvas.Hits()) != 3 || !canvas.Hits()[0].Disabled || canvas.Hits()[0].Focusable || !canvas.Hits()[1].Disabled {
		t.Fatalf("disabled hits = %+v", canvas.Hits())
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("tree paint allocations = %v", allocations)
	}
}

func BenchmarkTreePaint(b *testing.B) {
	var canvas gfx.Canvas
	theme := gfx.DarkTheme()
	for b.Loop() {
		canvas.Reset()
		tree := BeginTree(&canvas, theme, gfx.Rect{Width: 600, Height: 900}).RowHeight(44)
		for index := 0; index < 24; index++ {
			tree.Item(TreeItem{ID: gfx.Item(int(uint32(index + 1))), Label: "Host", Depth: uint8(index % 3), Selected: index == 7})
		}
	}
}

// SelectedColor replaces the theme's Accent for the selected row; a zero color
// leaves the theme's, so a theme's own fade or tint still applies.
func TestTreeSelectedRowColourIsTheTrees(t *testing.T) {
	theme := gfx.LightTheme()
	selected := gfx.Color{Red: 0.2, Green: 0.5, Blue: 0.4, Alpha: 1}
	paint := func(color gfx.Color) []gfx.Rectangle {
		var canvas gfx.Canvas
		tree := BeginTree(&canvas, theme, gfx.Rect{Width: 200, Height: 32}).SelectedColor(color)
		tree.Item(TreeItem{ID: gfx.Item(1), Label: "Selected", Selected: true})
		return canvas.Rectangles()
	}
	found := false
	for _, rectangle := range paint(selected) {
		if rectangle.Color == selected {
			found = true
		}
		if rectangle.Color == theme.Accent {
			t.Fatal("the selected row painted the theme's Accent instead of the tree's colour")
		}
	}
	if !found {
		t.Fatal("the selected row painted no background in the tree's colour")
	}
	found = false
	for _, rectangle := range paint(gfx.Color{}) {
		if rectangle.Color == theme.Accent {
			found = true
		}
	}
	if !found {
		t.Fatal("a zero SelectedColor did not keep the theme's Accent")
	}
}
