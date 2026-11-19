package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/menubarview"
)

var menubarComponent = Component{
	ID: "menubar", Title: "Menubar", Category: "Navigation", Summary: "持久菜单栏与调用方控制的弹出内容。",
	paint: (*State).paintMenubar,
	Examples: []Example{
		{ID: "default", Title: "文件菜单", Description: "点击 File 打开生产 MenubarContent。", Snippet: `file := tree.ID(page, "file")
bar := components.BeginMenubar(canvas, theme, bounds)
bar.AddTrigger(components.MenubarTrigger{ID: file, Label: "File", Open: open})
bar.AddTrigger(components.MenubarTrigger{ID: tree.ID(page, "view"), Label: "View"})
if open {
    options := components.MenubarOverlayOptions{Layer: 2, DismissID: file}
    content := components.BeginMenubarContent(canvas, theme, popup, options)
    content.AddItem(components.MenubarItem{ID: tree.ID(page, "new"), Label: "New window"})
    content.End()
}
if canvas.Clicked(file) {
    open = !open
}`,
			PageSnippet: `// m.menubar.Selected holds the row chosen during the last paint.
file, view, help := ui.ID(c), ui.ID(c), ui.ID(c)
newWindow, showGrid, deleteItem, reload := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
bar := &m.menubar
if bar.Selected == showGrid {
    m.grid = !m.grid
    bar.Selected = gfx.ID{}
}
bar.Reset()
bar.Trigger(file, "File").
    Item(newWindow, "New window").Shortcut("⌘N").
    Item(showGrid, "Show grid").Checkbox(m.grid).
    Separator().
    Item(deleteItem, "Delete workspace").Destructive()
bar.Trigger(view, "View").Item(reload, "Reload")
bar.Trigger(help, "Help")
menubarview.View(c, "Menubar", bar).FillWidth().Height(c.Theme().Metrics.ControlHeight)`,
			PageSnippetZh: `// m.menubar.Selected 保存上一次绘制中选中的行。
file, view, help := ui.ID(c), ui.ID(c), ui.ID(c)
newWindow, showGrid, deleteItem, reload := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
bar := &m.menubar
if bar.Selected == showGrid {
    m.grid = !m.grid
    bar.Selected = gfx.ID{}
}
bar.Reset()
bar.Trigger(file, "File").
    Item(newWindow, "New window").Shortcut("⌘N").
    Item(showGrid, "Show grid").Checkbox(m.grid).
    Separator().
    Item(deleteItem, "Delete workspace").Destructive()
bar.Trigger(view, "View").Item(reload, "Reload")
bar.Trigger(help, "Help")
menubarview.View(c, "Menubar", bar).FillWidth().Height(c.Theme().Metrics.ControlHeight)`},
		{ID: "states", Title: "菜单项状态", Description: "复选、禁用和危险菜单项。", Snippet: `options := components.MenubarOverlayOptions{
    Layer: 2, DismissID: tree.ID(page, "file"),
}
content := components.BeginMenubarContent(canvas, theme, popup, options)
content.AddItem(components.MenubarItem{
    ID: tree.ID(page, "grid"), Label: "Show grid",
    Kind: components.MenubarCheckbox, Checked: grid,
})
content.AddSeparator()
content.AddItem(components.MenubarItem{
    ID: tree.ID(page, "delete"), Label: "Delete workspace",
    Variant: components.MenubarItemDestructive, Disabled: true,
})
content.End()`,
			PageSnippet: `file, view, help := ui.ID(c), ui.ID(c), ui.ID(c)
newWindow, showGrid, deleteItem, reload := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
bar := &m.menubar
if bar.Selected == showGrid {
    m.grid = !m.grid
    bar.Selected = gfx.ID{}
}
bar.Reset()
bar.Open = file
bar.Trigger(file, "File").
    Item(newWindow, "New window").Shortcut("⌘N").
    Item(showGrid, "Show grid").Checkbox(m.grid).
    Separator().
    Item(deleteItem, "Delete workspace").Destructive().Disabled()
bar.Trigger(view, "View").Item(reload, "Reload")
bar.Trigger(help, "Help").Disabled()
menubarview.View(c, "Menubar", bar).FillWidth().Height(c.Theme().Metrics.ControlHeight)`,
			PageSnippetZh: `file, view, help := ui.ID(c), ui.ID(c), ui.ID(c)
newWindow, showGrid, deleteItem, reload := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
bar := &m.menubar
if bar.Selected == showGrid {
    m.grid = !m.grid
    bar.Selected = gfx.ID{}
}
bar.Reset()
bar.Open = file
bar.Trigger(file, "File").
    Item(newWindow, "New window").Shortcut("⌘N").
    Item(showGrid, "Show grid").Checkbox(m.grid).
    Separator().
    Item(deleteItem, "Delete workspace").Destructive().Disabled()
bar.Trigger(view, "View").Item(reload, "Reload")
bar.Trigger(help, "Help").Disabled()
menubarview.View(c, "Menubar", bar).FillWidth().Height(c.Theme().Metrics.ControlHeight)`},
	},
}

var (
	menubarUI    = tree.Feature("menubar")
	menubarFile  = tree.ID(menubarUI, "file")
	menubarView  = tree.ID(menubarUI, "view")
	menubarHelp  = tree.ID(menubarUI, "help")
	menubarGrid  = tree.ID(menubarUI, "grid")
	menubarFocus = tree.ID(menubarUI, "focus")
	menubarDrop  = tree.ID(menubarUI, "dismiss")
	// menubarItem's item i is the i-th plain item of the menu.
	menubarItem = tree.Sub(menubarUI, tree.Name("item"))

	// menubarDemo is the preview's page control. The preview has one menubar.
	menubarDemo      menubarview.Menubar
	menubarDemoState *State
)

func (state *State) paintMenubar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	bar := &menubarDemo
	if menubarDemoState != state {
		menubarDemoState = state
		bar.Open, bar.Selected = gfx.ID{}, gfx.ID{}
	}
	open := state.flags&3 != 0 || state.exampleID == "states"
	bar.Scope = menubarFocus
	bar.DismissID = menubarDrop
	bar.Layer = 2
	if state.exampleID == "states" {
		bar.Open = menubarFile
	}
	bar.Reset()
	bar.Trigger(menubarFile, "File").
		Item(menubarItem.At(0), "New window").Shortcut("⌘N").
		Item(menubarGrid, "Show grid").Checkbox(state.flags&4 != 0).
		Separator().
		Item(menubarItem.At(1), "Delete workspace").Destructive()
	if state.exampleID == "states" {
		bar.Disabled()
	}
	bar.Trigger(menubarView, "View").Item(menubarItem.At(2), "Reload")
	bar.Trigger(menubarHelp, "Help")
	if state.exampleID == "states" {
		bar.Disabled()
	}

	state.drawPage(canvas, theme, size, 560, state.Text("Menubar · 桌面菜单", "Menubar · Desktop menu"), func(c ui.C) {
		if !open {
			ui.Text(c, state.Text("点击 File 查看弹出菜单。", "Click File to open the popup menu.")).FontSize(theme.Metrics.FontSizeS).Muted()
		}
		menubarview.View(c, "Menubar", bar).FillWidth().Height(theme.Metrics.ControlHeight)
	})

	// Mirror the control's caller-owned state into the demo's flags, which the
	// tests and the caption read.
	state.flags &^= 3
	switch bar.Open {
	case menubarFile:
		state.flags |= 1
	case menubarView:
		state.flags |= 2
	}
	if selected := bar.Selected; !selected.IsZero() {
		if selected == menubarGrid {
			state.flags ^= 4
		}
		bar.Selected = gfx.ID{}
	}
}
