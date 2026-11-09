package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var contextMenuComponent = Component{
	ID: "context-menu", Title: "Context Menu", Category: "Overlays", Summary: "调用方定位的上下文菜单与多种菜单项语义。",
	paint: (*State).paintContextMenu, pointer: (*State).pointerContextMenu, initialize: (*State).initializeContextMenu,
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
if canvas.Clicked(toggleID) { checked = !checked }`},
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
if index, ok := canvas.ClickedIn(items); ok && index < 2 { sortBy = int(index) }`},
	},
}

const contextMenuOpenMask uint64 = 1 << 1

var (
	contextMenuUI      = tree.Feature("context-menu")
	contextMenuToggle  = tree.ID(contextMenuUI, "details")
	contextMenuSurface = tree.ID(contextMenuUI, "surface")
	// contextMenuItem's item i is the i-th plain item of the example.
	contextMenuItem = tree.Sub(contextMenuUI, tree.Name("item"))
)

func (state *State) initializeContextMenu() {
	state.flags |= contextMenuOpenMask
}

func (state *State) pointerContextMenu(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown || event.Button != gfx.PointerButtonSecondary || event.Target != contextMenuSurface {
		return gfx.EventResult{}
	}
	changed := state.flags&contextMenuOpenMask == 0 || state.points[0] != event.X || state.points[1] != event.Y
	state.flags |= contextMenuOpenMask
	state.points[0], state.points[1] = event.X, event.Y
	return gfx.EventResult{Handled: true, Repaint: changed}
}

func (state *State) paintContextMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var area ui.Box
	state.page(canvas, theme, size, 1000, state.Text("Context Menu · 上下文菜单", "Context Menu · Pointer actions"), func(f *ui.Frame) {
		f.Text(state.Text("在空白区域按次要按钮打开菜单。", "Right-click the empty area to open the menu.")).FontSize(theme.Metrics.FontSizeS).Muted()
		area = f.Slot().FillWidth().Height(max(size.Height-124, 120))
	})
	surface := area.Rect()
	canvas.RoundedBorder(surface, theme.Metrics.RadiusSurface, 1, theme.Border, theme.Muted)
	canvas.PushFocusScope(contextMenuSurface)
	canvas.HitWithOptions(contextMenuSurface, surface, "Context menu surface", gfx.HitOptions{SkipFocus: true})
	canvas.PopFocusScope()
	if state.flags&contextMenuOpenMask == 0 {
		return
	}
	menuWidth := min(size.Width-48, float32(260))
	menuX, menuY := (size.Width-menuWidth)/2, surface.Y+4
	if state.points[0] != 0 || state.points[1] != 0 {
		menuX = min(max(state.points[0], float32(8)), max(size.Width-menuWidth-8, 8))
		menuY = min(max(state.points[1], surface.Y), max(size.Height-266, surface.Y))
	}
	// Four 32px rows, a separator, and the surface's 4px top and bottom padding.
	menuHeight := float32(4*32 + 9 + 2*4)
	menu := components.BeginContextMenu(canvas, theme, gfx.Rect{X: menuX, Y: menuY, Width: menuWidth, Height: min(menuHeight, size.Height-menuY-8)}, components.ContextMenuOptions{Layer: 1})
	if state.exampleID == "default" {
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(0), Label: "Open", Shortcut: "⌘O"})
		menu.AddItem(components.ContextMenuItem{ID: contextMenuToggle, Label: "Show details", Kind: components.ContextMenuCheckbox, Checked: state.flags&1 != 0})
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(1), Label: "Open with", Kind: components.ContextMenuSubmenu, Expanded: true})
		menu.AddSeparator()
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(2), Label: "Move to trash", Variant: components.ContextMenuItemDestructive})
	} else {
		menu.AddLabel("Sort by", true)
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(3), Label: "Name", Kind: components.ContextMenuRadio, Selected: true})
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(4), Label: "Modified", Kind: components.ContextMenuRadio})
		menu.AddSeparator()
		menu.AddItem(components.ContextMenuItem{ID: contextMenuItem.At(5), Label: "Unavailable action", Disabled: true})
	}
	menu.End()
	if canvas.Clicked(contextMenuToggle) {
		state.flags ^= 1
	}
}
