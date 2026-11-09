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
