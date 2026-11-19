package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var contextMenuComponent = Component{
	ID: "context-menu", Title: "Context Menu", Category: "Overlays", Summary: "调用方定位的上下文菜单与多种菜单项语义。",
	paint: (*State).paintContextMenu,
	Examples: []Example{
		{ID: "default", Title: "文件操作", Description: "动作、快捷键和子菜单由生产 ContextMenu 绘制。", Snippet: `menu := components.BeginContextMenu(canvas, theme, at, components.ContextMenuOptions{Layer: 1})
menu.AddItem(components.ContextMenuItem{ID: tree.ID(page, "open"), Label: "Open", Shortcut: "⌘O"})
menu.AddItem(components.ContextMenuItem{
    ID: toggleID, Label: "Show details", Kind: components.ContextMenuCheckbox, Checked: checked,
})
menu.AddItem(components.ContextMenuItem{
    ID: tree.ID(page, "open-with"), Label: "Open with", Kind: components.ContextMenuSubmenu, Expanded: true,
})
menu.AddSeparator()
menu.AddItem(components.ContextMenuItem{
    ID: tree.ID(page, "trash"), Label: "Move to trash", Variant: components.ContextMenuItemDestructive,
})
if canvas.Clicked(toggleID) { checked = !checked }`, PageSnippet: `area := ui.Row(c).FillWidth().Height(120).Hit(ui.ID(c), "Context menu area")
ui.ContextMenu(c, area, func(menu *ui.MenuItems) {
    menu.Item("Open").Keys(ui.Primary, "O")
    menu.Item("Show details").Checked(false)
    menu.Sub("Open with", func(sub *ui.MenuItems) {})
    menu.Separator()
    menu.Item("Move to trash").Destructive()
})`, PageSnippetZh: `area := ui.Row(c).FillWidth().Height(120).Hit(ui.ID(c), "Context menu area")
ui.ContextMenu(c, area, func(menu *ui.MenuItems) {
    menu.Item("Open").Keys(ui.Primary, "O")
    menu.Item("Show details").Checked(false)
    menu.Sub("Open with", func(sub *ui.MenuItems) {})
    menu.Separator()
    menu.Item("Move to trash").Destructive()
})`},
		{ID: "states", Title: "菜单状态", Description: "复选、单选、禁用和危险项。", Snippet: `items := tree.Sub(page, tree.Name("item"))
menu := components.BeginContextMenu(canvas, theme, at, components.ContextMenuOptions{Layer: 1})
menu.AddLabel("Sort by", true)
for index, label := range []string{"Name", "Modified"} {
    menu.AddItem(components.ContextMenuItem{
        ID: items.At(index), Label: label, Kind: components.ContextMenuRadio, Selected: sortBy == index,
    })
}
menu.AddSeparator()
menu.AddItem(components.ContextMenuItem{ID: items.At(2), Label: "Unavailable action", Disabled: true})
if index, ok := canvas.ClickedIn(items); ok && index < 2 { sortBy = int(index) }`, PageSnippet: `area := ui.Row(c).FillWidth().Height(120).Hit(ui.ID(c), "Context menu area")
ui.ContextMenu(c, area, func(menu *ui.MenuItems) {
    menu.Label("Sort by")
    menu.Item("Name").Chosen(true)
    menu.Item("Modified").Chosen(false)
    menu.Separator()
    menu.Item("Unavailable action").Disabled(true)
})`, PageSnippetZh: `area := ui.Row(c).FillWidth().Height(120).Hit(ui.ID(c), "Context menu area")
ui.ContextMenu(c, area, func(menu *ui.MenuItems) {
    menu.Label("Sort by")
    menu.Item("Name").Chosen(true)
    menu.Item("Modified").Chosen(false)
    menu.Separator()
    menu.Item("Unavailable action").Disabled(true)
})`},
	},
}

func (state *State) paintContextMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 1000, state.Text("Context Menu · 上下文菜单", "Context Menu · Pointer actions"), func(c ui.C) {
		ui.Text(c, state.Text("在空白区域按次要按钮打开菜单。", "Right-click the empty area to open the menu.")).FontSize(theme.Metrics.FontSizeS).Muted()
		area := ui.Row(c).FillWidth().Height(max(size.Height-124, 120)).
			Radius(theme.Metrics.RadiusSurface).Background(theme.Muted).
			Hit(ui.ID(c), state.Text("上下文菜单区域", "Context menu area"))
		ui.ContextMenu(c, area, func(menu *ui.MenuItems) {
			if state.exampleID != "default" {
				menu.Label("Sort by")
				if menu.Item("Name").Chosen(state.values[0] == 0).Clicked() {
					state.values[0] = 0
				}
				if menu.Item("Modified").Chosen(state.values[0] == 1).Clicked() {
					state.values[0] = 1
				}
				menu.Separator()
				menu.Item("Unavailable action").Disabled(true)
				return
			}
			menu.Item("Open").Keys(ui.Primary, "O")
			if menu.Item("Show details").Checked(state.flags&1 != 0).Clicked() {
				state.flags ^= 1
			}
			menu.Sub("Open with", func(sub *ui.MenuItems) {})
			menu.Separator()
			menu.Item("Move to trash").Destructive()
		})
	})
}
