package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
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
}`},
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
content.End()`},
	},
}

var (
	menubarUI    = tree.Feature("menubar")
	menubarFile  = tree.ID(menubarUI, "file")
	menubarView  = tree.ID(menubarUI, "view")
	menubarHelp  = tree.ID(menubarUI, "help")
	menubarGrid  = tree.ID(menubarUI, "grid")
	menubarFocus = tree.ID(menubarUI, "focus")
	// menubarItem's item i is the i-th plain item of the menu.
	menubarItem = tree.Sub(menubarUI, tree.Name("item"))
)

func (state *State) paintMenubar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	open := state.flags&1 != 0 || state.exampleID == "states"
	var strip ui.Box
	state.page(canvas, theme, size, 560, state.Text("Menubar · 桌面菜单", "Menubar · Desktop menu"), func(f *ui.Frame) {
		strip = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
		if !open {
			f.Text(state.Text("点击 File 查看弹出菜单。", "Click File to open the popup menu.")).FontSize(theme.Metrics.FontSizeS).Muted()
		}
	})
	bounds := strip.Rect()
	bar := components.BeginMenubar(canvas, theme, bounds)
	bar.AddTrigger(components.MenubarTrigger{ID: menubarFile, Label: "File", Open: open})
	bar.AddTrigger(components.MenubarTrigger{ID: menubarView, Label: "View"})
	bar.AddTrigger(components.MenubarTrigger{ID: menubarHelp, Label: "Help", Disabled: state.exampleID == "states"})
	if open {
		// Three 32px rows, a separator, and the surface's 4px top and bottom padding.
		popup := gfx.Rect{X: bounds.X, Y: bounds.Y + bounds.Height + 8, Width: min(bounds.Width, float32(260)), Height: 3*32 + 9 + 2*4}
		content := components.BeginMenubarContent(canvas, theme, popup, components.MenubarOverlayOptions{Layer: 2, Scope: menubarFocus, DismissID: menubarFile})
		content.AddItem(components.MenubarItem{ID: menubarItem.At(0), Label: "New window", Shortcut: "⌘N"})
		content.AddItem(components.MenubarItem{ID: menubarGrid, Label: "Show grid", Kind: components.MenubarCheckbox, Checked: state.flags&2 != 0})
		content.AddSeparator()
		content.AddItem(components.MenubarItem{ID: menubarItem.At(1), Label: "Delete workspace", Variant: components.MenubarItemDestructive, Disabled: state.exampleID == "states"})
		content.End()
	}
	// File opens and closes the menu; so does a press outside it (its DismissID).
	if canvas.Clicked(menubarFile) {
		state.flags ^= 1
	}
	if canvas.Clicked(menubarGrid) {
		state.flags ^= 2
	}
}
