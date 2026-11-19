package ui

import (
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/components"
)

// row declares row i: the row element, painted by components.Tree, holding what
// the caller declares after the chevron.
func treeStateRow(s *TreeState, i int, rowBox Box) {
	c := s.GetC()
	f := c.GetF()
	node := s.Node(i)
	key := s.GetKey()(i)
	depth := node.Depth
	if s.DisplayDepth != nil {
		depth = s.DisplayDepth(depth)
	}
	item := components.TreeItem{
		ID: s.GetRows().At(key), Label: s.GetRowLabel()(i), Depth: uint8(min(max(depth, 0), 31)),
		Branch: node.Branch, Expanded: node.Expanded, Selected: node.Selected, Disabled: node.Disabled,
	}
	if node.Branch {
		item.DisclosureID = s.GetChevrons().At(key)
	}
	if canvas := f.Surface; canvas != nil && !node.Disabled {
		if canvas.Clicked(item.ID) {
			s.SetChosenIndex(i)
		}
		if node.Branch && canvas.Clicked(item.DisclosureID) {
			s.SetToggledIndex(i)
		}
	}
	theme := &f.Palette
	left := float32(item.Depth) * s.IndentValue(theme)
	if node.Branch {
		left += 24 // the chevron, as components.Tree places it
	}
	rowBox.Children(func() {
		b := Row(c).Height(s.HeightOf(node, theme)).Shrink(0).PadRight(8).PadLeft(left).Gap(theme.Metrics.GapItem).Items(Center).Draw(s.GetPaint())
		s.SetDeclaredRows(append(s.GetDeclared(), base.NewTreeRow(i, key, b.GetI(), b.GetGen(), item)))
		b.Children(func() {
			if s.Content != nil {
				s.Content(i)
			} else {
				Text(c, item.Label)
			}
		})
	})
}

// draw paints the row the context is for: its highlight, chevron and hits, and
// where a drag over it would drop.
func treeStateDraw(s *TreeState, d *DrawContext) {
	for i := range s.GetDeclared() {
		row := &s.GetDeclared()[i]
		if row.GetGen() != d.Frame.GetGen() || &d.Frame.Nodes[row.Node] != d.Node {
			continue
		}
		tree := components.BeginTree(d.Canvas, *d.Theme, d.Rect).RowHeight(d.Rect.Height).Indent(s.IndentValue(d.Theme))
		tree.Item(row.GetItem())
		if s.DragPtr().GetActive() {
			s.DragOver(row, d)
		}
		return
	}
}
