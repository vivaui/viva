package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var treeNavigationComponent = Component{
	ID: "tree-navigation", Title: "Tree Navigation", Category: "Data",
	Summary: "为应用管理的可见树行和展开状态提供键盘导航。",
	paint:   (*State).paintTreeNavigation, key: (*State).keyTreeNavigation,
	Examples: []Example{
		{ID: "default", Title: "键盘导航", Description: "聚焦 Production 行。按 → 展开或进入第一个子项，按 ← 返回父项或收起当前分支。", Snippet: `next, action, handled := ui.NavigateTree(visible, current, m.key, pageRows)
if handled {
    switch action {
    case ui.TreeNavigationExpand:
        expanded = true
    case ui.TreeNavigationCollapse:
        expanded = false
    }
    selected = next
    return gfx.EventResult{Focus: rowIDs[next], FocusVisible: true, Handled: true, Repaint: true}
}`, PageSnippet: `// The page Tree paints the rows; the application owns the keys.
rows := 1
if m.expanded {
    rows = 4
}
ui.Tree(c, &m.tree, rows, m.label, nil).TreeKeys(false)

// m.key is the key event's key.
items := []ui.TreeNavigationItem{
    {Branch: true, Expanded: m.expanded},
    {Depth: 1}, {Depth: 1}, {Depth: 1},
}
visible := items[:1]
if m.expanded {
    visible = items[:]
}
next, action, handled := ui.NavigateTree(visible, m.selected, m.key, len(items))
if handled {
    switch action {
    case ui.TreeNavigationExpand:
        m.expanded = true
    case ui.TreeNavigationCollapse:
        m.expanded = false
    }
    m.selected = next
    return gfx.EventResult{Focus: m.tree.RowID(next), FocusVisible: true, Handled: true, Repaint: true}
}`, PageSnippetZh: `// 页面 Tree 负责绘制行，键盘导航由应用决定。
rows := 1
if m.expanded {
    rows = 4
}
ui.Tree(c, &m.tree, rows, m.label, nil).TreeKeys(false)

// m.key 是按键事件的键。
items := []ui.TreeNavigationItem{
    {Branch: true, Expanded: m.expanded},
    {Depth: 1}, {Depth: 1}, {Depth: 1},
}
visible := items[:1]
if m.expanded {
    visible = items[:]
}
next, action, handled := ui.NavigateTree(visible, m.selected, m.key, len(items))
if handled {
    switch action {
    case ui.TreeNavigationExpand:
        m.expanded = true
    case ui.TreeNavigationCollapse:
        m.expanded = false
    }
    m.selected = next
    return gfx.EventResult{Focus: m.tree.RowID(next), FocusVisible: true, Handled: true, Repaint: true}
}`},
	},
}

// treeNavigationDemo is the page Tree's model: the rows, where the keyboard is,
// and the callbacks that describe and paint them. It lives in a map because the
// component's key handler runs outside the paint, as treeDemo does.
type treeNavigationDemo struct {
	state    ui.TreeState
	labels   [4]string
	node     func(int) ui.TreeNode
	label    func(int) string
	expanded bool
	selected int32
	height   float32
}

var treeNavigationDemos = map[*State]*treeNavigationDemo{}

func treeNavigationDemoFor(state *State) *treeNavigationDemo {
	if demo := treeNavigationDemos[state]; demo != nil {
		return demo
	}
	demo := &treeNavigationDemo{labels: [4]string{"Production", "Gateway", "Database", "Worker"}}
	demo.node = demo.describe
	demo.label = demo.nameOf
	demo.state.Node = demo.node
	treeNavigationDemos[state] = demo
	return demo
}

func (demo *treeNavigationDemo) nameOf(i int) string { return demo.labels[i] }

func (demo *treeNavigationDemo) describe(i int) ui.TreeNode {
	node := ui.TreeNode{Height: demo.height}
	if i == 0 {
		node.Branch, node.Expanded, node.Selected = true, demo.expanded, demo.selected == 0
		return node
	}
	node.Depth, node.Selected = 1, demo.selected == int32(i)
	return node
}

// rowOf is the visible row an identity names, or -1: a row's own hit or the
// disclosure of a branch.
func (demo *treeNavigationDemo) rowOf(id gfx.ID) int {
	if id.IsZero() {
		return -1
	}
	for i := range demo.labels {
		if demo.state.RowID(i) == id || demo.state.DisclosureID(i) == id {
			return i
		}
	}
	return -1
}

func (state *State) paintTreeNavigation(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := treeNavigationDemoFor(state)
	demo.expanded = state.flags&1 != 0
	demo.selected = state.values[0]
	demo.height = theme.Metrics.ControlHeight
	rows := 1
	if demo.expanded {
		rows = 4
	}
	state.drawPage(canvas, theme, size, 440, state.Text("Tree Navigation · 键盘导航", "Tree Navigation · Keyboard"), func(c ui.C) {
		ui.Tree(c, &demo.state, rows, demo.label, nil).
			FillWidth().Height(float32(rows) * demo.height).
			TreeKeys(false)
		caption(c, state.Text("聚焦行后，使用方向键浏览、展开或收起。", "Focus a row and use the arrow keys to move, expand, or collapse."))
	})
	if i, ok := demo.state.Toggled(); ok && i == 0 {
		state.flags ^= 1
	}
	if i, ok := demo.state.Chosen(); ok {
		state.values[0] = int32(i)
	}
}

func (state *State) keyTreeNavigation(event gfx.KeyEvent) gfx.EventResult {
	demo := treeNavigationDemoFor(state)
	if event.Release || event.Modifiers != 0 {
		return gfx.EventResult{}
	}
	current := demo.rowOf(event.Target)
	if current < 0 {
		return gfx.EventResult{}
	}
	expanded := state.flags&1 != 0
	items := [...]ui.TreeNavigationItem{
		{Branch: true, Expanded: expanded}, {Depth: 1}, {Depth: 1}, {Depth: 1},
	}
	visible := items[:1]
	if expanded {
		visible = items[:]
	}
	next, action, handled := ui.NavigateTree(visible, current, event.Key, len(items))
	if !handled {
		return gfx.EventResult{}
	}
	switch action {
	case ui.TreeNavigationExpand:
		state.flags |= 1
	case ui.TreeNavigationCollapse:
		state.flags &^= 1
	}
	changed := action != ui.TreeNavigationNone || state.values[0] != int32(next)
	state.values[0] = int32(next)
	demo.expanded, demo.selected = state.flags&1 != 0, int32(next)
	return gfx.EventResult{Focus: demo.state.RowID(next), FocusVisible: true, Handled: true, Repaint: changed}
}
