package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var navigationMenuComponent = Component{
	ID: "navigation-menu", Title: "Navigation Menu", Category: "Navigation", Summary: "带调用方开合状态的顶层导航与浮层链接。",
	paint: (*State).paintNavigationMenu,
	Examples: []Example{
		{ID: "default", Title: "产品导航", Description: "点击 Products 打开或关闭生产导航浮层。", Snippet: `options := components.NavigationMenuOptions{
    Open: open, Content: panel, Layer: 2,
    DismissID: tree.ID(page, "dismiss"), Scope: tree.ID(page, "content"),
}
menu := components.BeginNavigationMenu(canvas, theme, bounds, options)
menu.AddTrigger(components.NavigationMenuTrigger{
    ID: tree.ID(page, "products"), Label: "Products", Open: open,
})
content := menu.BeginContent()
content.AddLink(components.NavigationMenuLink{
    ID: tree.ID(page, "guide"), Label: "Getting started",
    Description: "Build a native window and retained scene.", Value: "/guide",
})
if canvas.Clicked(tree.ID(page, "products")) { open = !open }`, Height: 310},
		{ID: "direct-content", Title: "直接内容", Description: "以直接内容模式展示无触发器指示的菜单面板。", Snippet: `trigger := tree.ID(page, "products")
options := components.NavigationMenuOptions{
    Open: true, DirectContent: true, Content: panel, Layer: 2,
    Scope: tree.ID(page, "content"),
}
menu := components.BeginNavigationMenu(canvas, theme, bounds, options)
menu.AddTrigger(components.NavigationMenuTrigger{
    ID: trigger, Label: "Products", Open: true,
})
content := menu.BeginContent()
content.AddLink(components.NavigationMenuLink{
    ID: tree.ID(page, "guide"), Label: "Getting started",
    Description: "Build a native window and retained scene.", Value: "/guide",
})`, Height: 310},
	},
}

var (
	navigationMenuUI      = tree.Feature("navigation-menu")
	navigationMenuTrigger = tree.ID(navigationMenuUI, "trigger")
	navigationMenuDocs    = tree.ID(navigationMenuUI, "docs")
	navigationMenuGuide   = tree.ID(navigationMenuUI, "guide")
	navigationMenuAPI     = tree.ID(navigationMenuUI, "api")
	navigationMenuDismiss = tree.ID(navigationMenuUI, "dismiss")
	navigationMenuScope   = tree.ID(navigationMenuUI, "content")
)

func (state *State) paintNavigationMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	open := state.flags&1 != 0 || state.exampleID == "direct-content"
	status := state.Text("Products 已关闭", "Products closed")
	if open {
		status = state.Text("Products 已打开；Escape 使用稳定 dismiss ID", "Products open; Escape dismisses through a stable ID")
	}
	var brand, bar ui.Box
	state.drawPage(canvas, theme, size, 620, "Navigation Menu", func(c ui.C) {
		caption(c, status)
		ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
			brand = ui.Text(c, "Viva").FontSize(theme.Metrics.FontSizeM + 2).Weight(gfx.WeightSemibold)
			bar = ui.Slot(c).Grow(1).Height(theme.Metrics.ControlHeight)
		})
	})

	// The panel drops from the bar, over the blank part of the page below it.
	bounds := bar.Rect()
	// The panel hangs from the menu when it fits there, and from the page's edge
	// when the stage is too narrow for that.
	contentX, contentWidth := bounds.X, float32(420)
	if right := bounds.X + bounds.Width; contentX+contentWidth > right {
		contentX = brand.Rect().X
		contentWidth = min(right-contentX, contentWidth)
	}
	options := components.NavigationMenuOptions{
		Open: open, DirectContent: state.exampleID == "direct-content",
		// Two description rows (56 + 4 gap each) inside the content's 8px padding.
		Content: gfx.Rect{X: contentX, Y: bounds.Y + bounds.Height + 8, Width: contentWidth, Height: 8 + 2*(56+4) + 8},
		Layer:   2, Scope: navigationMenuScope, DismissID: navigationMenuDismiss,
	}
	menu := components.BeginNavigationMenu(canvas, theme, bounds, options)
	menu.AddTrigger(components.NavigationMenuTrigger{
		ID: navigationMenuTrigger, Label: "Products", Hint: "Open product navigation", Open: open,
	})
	menu.AddLink(components.NavigationMenuLink{
		ID: navigationMenuDocs, Label: "Docs", Value: "/docs", Active: state.values[0] == 1,
	})

	content := menu.BeginContent()
	content.AddLink(components.NavigationMenuLink{
		ID: navigationMenuGuide, Label: "Getting started", Description: "Build a native window and retained scene.", Value: "/guide",
		Active: state.values[0] == 2,
	})
	content.AddLink(components.NavigationMenuLink{
		ID: navigationMenuAPI, Label: "Component API", Description: "Read the source-owned recipes.", Value: "/components",
		Active: state.values[0] == 3,
	})

	switch {
	case canvas.Clicked(navigationMenuTrigger):
		state.flags ^= 1
	case canvas.Clicked(navigationMenuDocs):
		state.values[0], state.flags = 1, state.flags&^1
	case canvas.Clicked(navigationMenuGuide):
		state.values[0], state.flags = 2, state.flags&^1
	case canvas.Clicked(navigationMenuAPI):
		state.values[0], state.flags = 3, state.flags&^1
	case canvas.Clicked(navigationMenuDismiss):
		state.flags &^= 1
	}
}
