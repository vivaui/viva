package componentdemo

import (
	"github.com/vivaui/viva/components"
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
}`},
	},
}

var (
	treeUI              = tree.Feature("tree")
	treeGroupRow        = tree.ID(treeUI, "group")
	treeGroupDisclosure = tree.ID(treeUI, "disclosure")
	// treeLeaf's item i is the i-th host.
	treeLeaf = tree.Sub(treeUI, tree.Name("host"))
)

func initializeTree(state *State) {
	state.flags = 1
	state.values[0] = 1
}

func (state *State) paintTree(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	// The demo draws a two-line row; the extra 12px is its own content, not
	// the tree's default single-line row height.
	rowHeight := theme.Metrics.ControlHeight + 12
	var stage ui.Box
	state.drawPage(canvas, theme, size, 600, state.Text("Tree · 主机目录", "Tree · Host directory"), func(c ui.C) {
		stage = ui.Slot(c).Height(rowHeight * 4)
	})
	disabled := state.exampleID == "disabled"
	expanded := state.flags&1 != 0
	hosts := components.BeginTree(canvas, theme, stage.Rect()).RowHeight(rowHeight).Indent(22)
	group := components.TreeItem{
		ID: treeGroupRow, DisclosureID: treeGroupDisclosure, Label: "Production", Branch: true,
		Expanded: expanded, Disabled: disabled,
	}
	row := hosts.Item(group)
	canvas.Label("Production", gfx.Rect{X: row.Content.X + 2, Y: row.Content.Y + 10, Width: row.Content.Width - 80, Height: 24}, gfx.TextStyle{
		Size: theme.Metrics.FontSizeM, Weight: gfx.WeightSemibold, Color: theme.Foreground,
	})
	canvas.Label("3 hosts", gfx.Rect{X: row.Bounds.X + row.Bounds.Width - 72, Y: row.Bounds.Y + 11, Width: 64, Height: 22}, gfx.TextStyle{
		Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground,
	})
	if canvas.Clicked(treeGroupRow) || canvas.Clicked(treeGroupDisclosure) {
		state.flags ^= 1
	}
	if !expanded {
		return
	}
	names := [...]string{"Gateway", "Database", "Worker"}
	addresses := [...]string{"gw.example.com", "db.example.com", "worker.example.com"}
	for index := range names {
		selected := state.exampleID == "selection" && state.values[0] == int32(index)
		row = hosts.Item(components.TreeItem{
			ID: treeLeaf.At(index), Label: names[index], Depth: 1, Selected: selected, Disabled: disabled,
		})
		canvas.Label(names[index], gfx.Rect{X: row.Content.X + 10, Y: row.Content.Y + 4, Width: 120, Height: 20}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground,
		})
		canvas.Label(addresses[index], gfx.Rect{X: row.Content.X + 10, Y: row.Content.Y + 23, Width: row.Content.Width - 20, Height: 18}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground,
		})
	}
	if index, ok := canvas.ClickedIn(treeLeaf); ok {
		state.values[0] = int32(index)
	}
}
