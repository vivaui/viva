package ui

import (
	"unsafe"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
)

// TreeNode is what a Tree shows of one of its visible rows: how deep it is, whether
// it has children and shows them, and whether it is the selected one.
type TreeNode struct {
	Depth              int
	Branch, Expanded   bool
	Selected, Disabled bool
}

// TreeState is a Tree's: the visible rows of the application's tree, flattened in
// order, described by Node; ListState's Key names them by an item's ID so that a
// row keeps its focus as rows open and close above it. RowHeight and Indent are
// the height of a row and the step of a level (a control tall and 16 when zero).
// Keep one in the model.
//
// The tree's structure is the application's: a press on a branch's chevron, or
// Left and Right on it, is Toggled, and the application opens or closes it and
// gives the Tree the rows that then show; a click or Enter on a row is Chosen.
type TreeState struct {
	ListState
	Node      func(i int) TreeNode
	RowHeight float32
	Indent    float32

	toggled, chosen int
	scope           gfx.Scope // the ids of the rows (scope.At(0)) and chevrons (At(1))
	rows, chevrons  gfx.Scope
	scroll          *ScrollState // the List's, between paints: its area's state lives while it is declared
	n               int
	declared        []treeRow // the rows of the last paint
	declare         func(i int)
	paint           func(*DrawContext)
	c               C
	label           func(int) string
	content         func(int)
}

type treeRow struct {
	index int
	key   int
	node  int32 // the row element
	item  components.TreeItem
}

// Toggled reports which row's chevron this paint delivers a press on, or Left or
// Right on a branch: the application opens or closes it.
func (s *TreeState) Toggled() (int, bool) { return s.toggled, s.toggled >= 0 }

// Chosen reports which row this paint delivers a click or Enter on.
func (s *TreeState) Chosen() (int, bool) { return s.chosen, s.chosen >= 0 }

// Tree is a scrolling tree of n visible rows, of which it declares only those on
// screen, as List does: a chevron opens and closes a branch, the arrows move
// between rows (Left to the parent, Right to the first child, opening or closing a
// branch on the way), and Enter or a click chooses one. label names row i, for
// assistive technology and as its content when content is nil; content declares
// what is in the row after its chevron (an icon, the name, a count). Keep label and
// content in the model, as Node is: a method value made in each paint allocates.
//
//	ui.Tree(c, &m.files, len(m.visible), m.name, nil).Grow(1)
//	if i, ok := m.files.Toggled(); ok { m.toggle(i) }
//	if i, ok := m.files.Chosen(); ok { m.open(i) }
//
// Not inside a Memo.
func Tree(c C, state *TreeState, n int, label func(i int) string, content func(i int)) Box {
	p := c.page
	p.forbidInMemo("Tree")
	if state.Node == nil {
		panic("ui: Tree needs TreeState.Node, which describes each visible row")
	}
	if state.declare == nil {
		state.declare, state.paint = state.row, state.draw // made once: method values made in each paint allocate
	}
	scope := p.boundScope(pkTree, unsafe.Pointer(state))
	if scope != state.scope {
		state.scope, state.rows, state.chevrons = scope, p.tree.Sub(scope, gfx.Index(0)), p.tree.Sub(scope, gfx.Index(1))
	}
	state.toggled, state.chosen = -1, -1
	state.c, state.label, state.content, state.n = c, label, content, n
	state.declared = state.declared[:0]
	area := List(c, &state.ListState, n, state.declare)
	state.scroll = c.f.exts[c.f.nodes[area.i].ext].scrollState
	state.c, state.label, state.content = nil, nil, nil // the model's functions: not kept past the paint
	p.trees = append(p.trees, state)
	if p.treeRoute == nil {
		p.keyRoute, p.treeRoute = (*Page).routeKey, (*Page).treeKey
	}
	return area
}

func (s *TreeState) rowHeight(theme *Theme) float32 {
	if s.RowHeight > 0 {
		return s.RowHeight
	}
	return theme.Metrics.ControlHeight
}

func (s *TreeState) indent(theme *Theme) float32 {
	if s.Indent > 0 {
		return s.Indent
	}
	return 16 * theme.Metrics.ControlHeight / 32
}

func (s *TreeState) key(i int) int {
	if s.Key != nil {
		return s.Key(i)
	}
	return i
}

// row declares row i: the row element, painted by components.Tree, holding what
// the caller declares after the chevron.
func (s *TreeState) row(i int) {
	c := s.c
	f := c.f
	node := s.Node(i)
	key := s.key(i)
	item := components.TreeItem{
		ID: s.rows.At(key), Label: s.label(i), Depth: uint8(min(max(node.Depth, 0), 31)),
		Branch: node.Branch, Expanded: node.Expanded, Selected: node.Selected, Disabled: node.Disabled,
	}
	if node.Branch {
		item.DisclosureID = s.chevrons.At(key)
	}
	if canvas := f.canvas; canvas != nil && !node.Disabled {
		if canvas.Clicked(item.ID) {
			s.chosen = i
		}
		if node.Branch && canvas.Clicked(item.DisclosureID) {
			s.toggled = i
		}
	}
	theme := &f.theme
	left := float32(item.Depth) * s.indent(theme)
	if node.Branch {
		left += 24 // the chevron, as components.Tree places it
	}
	b := Row(c).Height(s.rowHeight(theme)).Shrink(0).PadSides(0, 8, 0, left).Gap(theme.Metrics.GapItem).Items(Center).Draw(s.paint)
	s.declared = append(s.declared, treeRow{index: i, key: key, node: b.i, item: item})
	b.Children(func() {
		if s.content != nil {
			s.content(i)
		} else {
			Text(c, item.Label)
		}
	})
}

// draw paints the row the context is for: its highlight, chevron and hits.
func (s *TreeState) draw(d *DrawContext) {
	for i := range s.declared {
		row := &s.declared[i]
		if &d.frame.nodes[row.node] != d.node {
			continue
		}
		tree := components.BeginTree(d.Canvas, *d.Theme, d.Rect).RowHeight(d.Rect.Height).Indent(s.indent(d.Theme))
		tree.Item(row.item)
		return
	}
}

// treeKey moves through a Tree from the focused row with the arrows: Up and Down to
// the row above and below, Home and End, Page Up and Down by a screen, Right to open
// a branch or go to its first child, Left to close it or go to the parent.
func (p *Page) treeKey(event gfx.KeyEvent) (gfx.EventResult, bool) {
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}, false
	}
	for _, s := range p.trees {
		var current *treeRow
		for i := range s.declared {
			if s.declared[i].item.ID == event.Target {
				current = &s.declared[i]
			}
		}
		if current == nil {
			continue
		}
		i, n := current.index, s.n
		node := s.Node(i)
		page := max(len(s.declared)-2, 1)
		target := i
		switch event.Key {
		case gfx.KeyUp:
			target = max(i-1, 0)
		case gfx.KeyDown:
			target = min(i+1, n-1)
		case gfx.KeyHome:
			target = 0
		case gfx.KeyEnd:
			target = n - 1
		case gfx.KeyPageUp:
			target = max(i-page, 0)
		case gfx.KeyPageDown:
			target = min(i+page, n-1)
		case gfx.KeyRight:
			switch {
			case node.Branch && !node.Expanded:
				return gfx.EventResult{Handled: true, Activate: current.item.DisclosureID}, true
			case node.Branch && i+1 < n && s.Node(i+1).Depth > node.Depth:
				target = i + 1
			}
		case gfx.KeyLeft:
			if node.Branch && node.Expanded {
				return gfx.EventResult{Handled: true, Activate: current.item.DisclosureID}, true
			}
			for j := i - 1; j >= 0; j-- {
				if s.Node(j).Depth < node.Depth {
					target = j
					break
				}
			}
		default:
			return gfx.EventResult{}, false
		}
		s.reveal(target)
		return gfx.EventResult{Handled: true, Repaint: true, Focus: s.rows.At(s.key(target)), FocusVisible: true}, true
	}
	return gfx.EventResult{}, false
}

// reveal scrolls the tree so that row i shows; the repaint declares it, and the
// focus goes to it there.
func (s *TreeState) reveal(i int) {
	scroll := s.scroll
	if scroll == nil || scroll.Viewport <= 0 {
		return
	}
	top, bottom := s.top(i), s.top(i+1)
	switch {
	case top < scroll.Offset:
		scroll.ScrollTo(top)
	case bottom > scroll.Offset+scroll.Viewport:
		scroll.ScrollTo(bottom - scroll.Viewport)
	}
}
