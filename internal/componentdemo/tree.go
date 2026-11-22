package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var treeComponent = Component{
	ID: "tree", Title: "Tree", Category: "Data", Summary: "调用方扁平化层级、持有展开与选择状态的紧凑树。",
	paint: (*State).paintTree, initialize: initializeTree,
	Examples: []Example{
		{ID: "default", Title: "主机目录", Description: "分组 disclosure 与叶子内容使用独立稳定 ID。", Snippet: `hosts := components.BeginTree(canvas, theme, bounds).RowHeight(rowHeight)
group := components.TreeItem{
    ID: tree.ID(page, "group"), DisclosureID: tree.ID(page, "disclosure"),
    Label: "Production", Branch: true, Expanded: expanded,
}
hosts.Item(group)
if canvas.Clicked(group.DisclosureID) { expanded = !expanded }
if expanded {
    hosts.Item(components.TreeItem{ID: tree.ID(page, "gateway"), Label: "Gateway", Depth: 1})
    hosts.Item(components.TreeItem{ID: tree.ID(page, "database"), Label: "Database", Depth: 1})
    hosts.Item(components.TreeItem{ID: tree.ID(page, "worker"), Label: "Worker", Depth: 1})
}`, PageSnippet: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: m.expanded, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Height: 44}
}
rows := 1
if m.expanded {
    rows = 4
}
ui.Tree(c, &m.tree, rows, func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(float32(rows) * 44)
if _, ok := m.tree.Toggled(); ok {
    m.expanded = !m.expanded
}`, PageSnippetZh: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: m.expanded, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Height: 44}
}
rows := 1
if m.expanded {
    rows = 4
}
ui.Tree(c, &m.tree, rows, func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(float32(rows) * 44)
if _, ok := m.tree.Toggled(); ok {
    m.expanded = !m.expanded
}`},
		{ID: "selection", Title: "选择状态", Description: "点击主机行切换调用方持有的当前项。", Snippet: `hosts := components.BeginTree(canvas, theme, bounds)
hosts.Item(components.TreeItem{
    ID: tree.ID(page, "group"), DisclosureID: tree.ID(page, "disclosure"),
    Label: "Production", Branch: true, Expanded: true,
})
for index, name := range [...]string{"Gateway", "Database", "Worker"} {
    id := tree.ID(page, name)
    hosts.Item(components.TreeItem{
        ID: id, Label: name, Depth: 1, Selected: selected == index,
    })
    if canvas.Clicked(id) { selected = index }
}`, PageSnippet: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: true, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Selected: i-1 == m.selected, Height: 44}
}
ui.Tree(c, &m.tree, len(labels), func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(4 * 44)
if i, ok := m.tree.Chosen(); ok && i > 0 {
    m.selected = i - 1
}`, PageSnippetZh: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: true, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Selected: i-1 == m.selected, Height: 44}
}
ui.Tree(c, &m.tree, len(labels), func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(4 * 44)
if i, ok := m.tree.Chosen(); ok && i > 0 {
    m.selected = i - 1
}`},
		{ID: "disabled", Title: "禁用分支", Description: "保留层级和语义，但分组与叶子均不可交互。", Snippet: `hosts := components.BeginTree(canvas, theme, bounds)
hosts.Item(components.TreeItem{
    ID: tree.ID(page, "group"), DisclosureID: tree.ID(page, "disclosure"),
    Label: "Production", Branch: true, Expanded: true, Disabled: true,
})
for _, name := range [...]string{"Gateway", "Database", "Worker"} {
    hosts.Item(components.TreeItem{
        ID: tree.ID(page, name), Label: name, Depth: 1, Disabled: true,
    })
}`, PageSnippet: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: true, Disabled: true, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Disabled: true, Height: 44}
}
ui.Tree(c, &m.tree, len(labels), func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(4 * 44)`, PageSnippetZh: `labels := []string{"Production", "Gateway", "Database", "Worker"}
addresses := []string{"gw.example.com", "db.example.com", "worker.example.com"}
m.tree.Node = func(i int) ui.TreeNode {
    if i == 0 {
        return ui.TreeNode{Branch: true, Expanded: true, Disabled: true, Height: 44}
    }
    return ui.TreeNode{Depth: 1, Disabled: true, Height: 44}
}
ui.Tree(c, &m.tree, len(labels), func(i int) string { return labels[i] }, func(i int) {
    if i == 0 {
        ui.Column(c).Grow(1).Children(func() {
            ui.Text(c, labels[0]).Weight(gfx.WeightSemibold)
        })
        ui.Text(c, "3 hosts").Muted()
        return
    }
    ui.Column(c).Grow(1).Gap(1).Children(func() {
        ui.Text(c, labels[i]).Weight(gfx.WeightMedium)
        ui.Text(c, addresses[i-1]).Muted()
    })
}).Height(4 * 44)`},
	},
}

func initializeTree(state *State) {
	state.flags = 1
	state.values[0] = 1
}

// treeDemo keeps the page Tree's expanded and selected rows and the stable
// callbacks that describe and paint them.
type treeDemo struct {
	state    ui.TreeState
	labels   [4]string
	address  [3]string
	node     func(int) ui.TreeNode
	label    func(int) string
	content  func(int)
	c        ui.C
	expanded bool
	disabled bool
	selected int32
	choose   bool
	height   float32
}

var treeDemos = map[*State]*treeDemo{}

func treeDemoFor(state *State) *treeDemo {
	if demo := treeDemos[state]; demo != nil {
		return demo
	}
	demo := &treeDemo{
		labels: [4]string{"Production", "Gateway", "Database", "Worker"},
		address: [3]string{
			"gw.example.com",
			"db.example.com",
			"worker.example.com",
		},
	}
	demo.node = demo.describe
	demo.label = demo.nameOf
	demo.content = demo.paintContent
	demo.state.Node = demo.node
	treeDemos[state] = demo
	return demo
}

func (demo *treeDemo) nameOf(i int) string { return demo.labels[i] }

func (demo *treeDemo) describe(i int) ui.TreeNode {
	node := ui.TreeNode{Height: demo.height, Disabled: demo.disabled}
	if i == 0 {
		node.Branch, node.Expanded = true, demo.expanded
		return node
	}
	node.Depth = 1
	node.Selected = demo.choose && demo.selected == int32(i-1)
	return node
}

func (demo *treeDemo) paintContent(i int) {
	if i == 0 {
		ui.Column(demo.c).Grow(1).Children(func() {
			ui.Text(demo.c, demo.labels[0]).Weight(gfx.WeightSemibold)
		})
		ui.Text(demo.c, "3 hosts").Muted()
		return
	}
	ui.Column(demo.c).Grow(1).Gap(1).Children(func() {
		ui.Text(demo.c, demo.labels[i]).Weight(gfx.WeightMedium)
		ui.Text(demo.c, demo.address[i-1]).Muted()
	})
}

func (state *State) paintTree(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := treeDemoFor(state)
	demo.expanded = state.flags&1 != 0
	demo.disabled = state.exampleID == "disabled"
	demo.choose = state.exampleID == "selection"
	demo.selected = state.values[0]
	demo.height = theme.Metrics.ControlHeight + 12
	rows := 1
	if demo.expanded {
		rows = 4
	}
	state.drawPage(canvas, theme, size, 600, state.Text("Tree · 主机目录", "Tree · Host directory"), func(c ui.C) {
		demo.c = c
		demo.state.Node = demo.node
		ui.Tree(c, &demo.state, rows, demo.label, demo.content).Height(float32(rows) * demo.height)
	})
	if i, ok := demo.state.Toggled(); ok && i == 0 {
		state.flags ^= 1
	}
	if i, ok := demo.state.Chosen(); ok && i > 0 {
		state.values[0] = int32(i - 1)
	}
}
