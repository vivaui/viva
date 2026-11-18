package ui

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// row declares row i: the row element, painted by components.Tree, holding what
// the caller declares after the chevron.
func (s *TreeState) row(i int, rowBox Box) {
	c := s.c
	f := c.f
	node := s.Node(i)
	key := s.key(i)
	item := components.TreeItem{
		ID: s.rows.At(key), Label: s.rowLabel(i), Depth: uint8(min(max(node.Depth, 0), 31)),
		Branch: node.Branch, Expanded: node.Expanded, Selected: node.Selected, Disabled: node.Disabled,
	}
	if node.Branch {
		item.DisclosureID = s.chevrons.At(key)
	}
	if canvas := f.Surface; canvas != nil && !node.Disabled {
		if canvas.Clicked(item.ID) {
			s.chosen = i
		}
		if node.Branch && canvas.Clicked(item.DisclosureID) {
			s.toggled = i
		}
	}
	theme := &f.Palette
	left := float32(item.Depth) * s.indent(theme)
	if node.Branch {
		left += 24 // the chevron, as components.Tree places it
	}
	rowBox.Children(func() {
		b := Row(c).Height(s.heightOf(node, theme)).Shrink(0).PadRight(8).PadLeft(left).Gap(theme.Metrics.GapItem).Items(Center).Draw(s.paint)
		s.declared = append(s.declared, treeRow{index: i, key: key, node: b.i, gen: b.gen, item: item})
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
func (s *TreeState) draw(d *DrawContext) {
	for i := range s.declared {
		row := &s.declared[i]
		if row.gen != d.frame.gen || &d.frame.Nodes[row.node] != d.node {
			continue
		}
		tree := components.BeginTree(d.Canvas, *d.Theme, d.Rect).RowHeight(d.Rect.Height).Indent(s.indent(d.Theme))
		tree.Item(row.item)
		if s.drag.active {
			s.dragOver(row, d)
		}
		return
	}
}

// treeType moves to the next row whose name begins with the letters typed, from
// the focused row of a Tree, as a file list does.
func (p *Page) TreeType(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Modifiers&^gfx.ModifierShift != 0 {
		return gfx.EventResult{}, false
	}
	text := event.Text
	if text == "" && event.Key == gfx.KeySpace {
		text = " "
	}
	if text == "" {
		return gfx.EventResult{}, false
	}
	for _, s := range p.trees {
		if _, inside := event.Target.In(s.rows); !inside || s.nameOf == nil || s.f == nil || s.f.Surface == nil {
			continue
		}
		s.names = s.names[:0]
		for i := 0; i < s.n; i++ {
			s.names = append(s.names, components.MenuTypeaheadItem{ID: s.rows.At(s.key(i)), Label: s.nameOf(i), Disabled: s.Node(i).Disabled})
		}
		now := clockEpoch.Add(s.f.Surface.Now())
		typing := text == " " && s.typeahead.Typing(now)
		if id, ok := s.typeahead.TypeItems(text, now, s.names, event.Target); ok {
			for i := range s.names {
				if s.names[i].ID == id {
					s.reveal(i)
				}
			}
			return gfx.EventResult{Handled: true, Repaint: true, Focus: id, FocusVisible: true}, true
		}
		if typing {
			return gfx.EventResult{Handled: true}, true
		}
		return gfx.EventResult{}, false
	}
	return gfx.EventResult{}, false
}
