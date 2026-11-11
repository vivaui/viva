package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var treeNavigationComponent = Component{
	ID: "tree-navigation", Title: "Tree Navigation", Category: "Data",
	Summary: "为应用管理的可见树行和展开状态提供键盘导航。",
	paint:   (*State).paintTreeNavigation, key: (*State).keyTreeNavigation,
	Examples: []Example{
		{ID: "default", Title: "键盘导航", Description: "聚焦 Production 行。按 → 展开或进入第一个子项，按 ← 返回父项或收起当前分支。", Snippet: `next, action, handled := components.NavigateTree(visible, current, event.Key, pageRows)
if handled {
    switch action {
    case components.TreeNavigationExpand:
        expanded = true
    case components.TreeNavigationCollapse:
        expanded = false
    }
    selected = next
    return gfx.EventResult{Focus: rowIDs[next], FocusVisible: true, Handled: true, Repaint: true}
}`},
	},
}

var (
	treeNavigationUI         = tree.Feature("tree-navigation")
	treeNavigationRows       = tree.Sub(treeNavigationUI, tree.Name("row"))
	treeNavigationDisclosure = tree.ID(treeNavigationUI, "disclosure")
)

func (state *State) paintTreeNavigation(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	rowHeight := theme.Metrics.ControlHeight
	var stage ui.Box
	state.drawPage(canvas, theme, size, 440, state.Text("Tree Navigation · 键盘导航", "Tree Navigation · Keyboard"), func(c ui.C) {
		stage = ui.Slot(c).FillWidth().Height(rowHeight * 4)
		caption(c, state.Text("聚焦行后，使用方向键浏览、展开或收起。", "Focus a row and use the arrow keys to move, expand, or collapse."))
	})

	expanded := state.flags&1 != 0
	rows := components.BeginTree(canvas, theme, stage.Rect()).RowHeight(rowHeight)
	for index, label := range [...]string{"Production", "Gateway", "Database", "Worker"} {
		if index > 0 && !expanded {
			break
		}
		item := components.TreeItem{ID: treeNavigationRows.At(index), Label: label, Selected: state.values[0] == int32(index)}
		if index == 0 {
			item.Branch, item.Expanded, item.DisclosureID = true, expanded, treeNavigationDisclosure
		} else {
			item.Depth = 1
		}
		row := rows.Item(item)
		canvas.Label(label, row.Content, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.Foreground})
		if canvas.Clicked(item.ID) {
			state.values[0] = int32(index)
		}
	}
	if canvas.Clicked(treeNavigationDisclosure) {
		state.flags ^= 1
		state.values[0] = 0
		canvas.RequestFocus(treeNavigationRows.At(0))
	}
}

func (state *State) keyTreeNavigation(event gfx.KeyEvent) gfx.EventResult {
	current := listIndex(event.Target, treeNavigationRows)
	if event.Target == treeNavigationDisclosure {
		current = 0
	}
	if event.Release || event.Modifiers != 0 || current < 0 {
		return gfx.EventResult{}
	}
	expanded := state.flags&1 != 0
	items := [...]components.TreeNavigationItem{
		{Branch: true, Expanded: expanded}, {Depth: 1}, {Depth: 1}, {Depth: 1},
	}
	visible := items[:1]
	if expanded {
		visible = items[:]
	}
	next, action, handled := components.NavigateTree(visible, current, event.Key, len(items))
	if !handled {
		return gfx.EventResult{}
	}
	switch action {
	case components.TreeNavigationExpand:
		state.flags |= 1
	case components.TreeNavigationCollapse:
		state.flags &^= 1
	}
	changed := action != components.TreeNavigationNone || state.values[0] != int32(next)
	state.values[0] = int32(next)
	return gfx.EventResult{Focus: treeNavigationRows.At(next), FocusVisible: true, Handled: true, Repaint: changed}
}
