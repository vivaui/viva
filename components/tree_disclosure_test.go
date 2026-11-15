package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A tree that ships its own disclosure arrows keeps its rows in step: the column the
// arrow takes is the one the application asked for, and a leaf keeps only the indent.
func TestTreeDisclosureSlotMovesRowContent(t *testing.T) {
	var canvas gfx.Canvas
	tree := BeginTree(&canvas, gfx.LightTheme(), gfx.Rect{Width: 320, Height: 240}).RowHeight(24).Indent(20)
	tree = tree.Disclosure(TreeDisclosure{Slot: 17, Size: 16})
	branch := tree.Item(TreeItem{ID: gfx.Item(1), DisclosureID: gfx.Item(2), Label: "src", Branch: true, Expanded: true})
	if branch.Content.X != 17 {
		t.Fatalf("content X = %v, want the requested 17 point slot", branch.Content.X)
	}
	leaf := tree.Item(TreeItem{ID: gfx.Item(3), Label: "app.go", Depth: 1})
	if leaf.Content.X != 20 {
		t.Fatalf("leaf content X = %v, want one 20 point indent", leaf.Content.X)
	}
}

// Leaving the disclosure option out keeps Viva's own chevron column, so a tree that
// sets nothing looks and lays out as before.
func TestTreeDisclosureDefaultsToTheChevronColumn(t *testing.T) {
	var canvas gfx.Canvas
	tree := BeginTree(&canvas, gfx.LightTheme(), gfx.Rect{Width: 320, Height: 240}).RowHeight(24).Indent(20)
	branch := tree.Item(TreeItem{ID: gfx.Item(1), DisclosureID: gfx.Item(2), Label: "src", Branch: true})
	if branch.Content.X != 24 {
		t.Fatalf("content X = %v, want the 24 point chevron column", branch.Content.X)
	}
}
