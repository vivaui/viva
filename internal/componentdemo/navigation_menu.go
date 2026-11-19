package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/navigationmenuview"
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
if canvas.Clicked(tree.ID(page, "products")) { open = !open }`,
			PageSnippet: `// m.navigation.Selected holds the link chosen during the last paint.
products, docs, guide, api := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
nav := m.navigation
switch nav.Selected {
case docs:
    m.page = "docs"
case guide:
    m.page = "guide"
case api:
    m.page = "components"
}
nav.Selected = gfx.ID{}
nav.Reset()
nav.Trigger(products, "Products").Hint("Open product navigation")
nav.Link(docs, "Docs").Value("/docs").Active(m.page == "docs")
nav.ContentLink(guide, "Getting started").
    Description("Build a native window and retained scene.").
    Value("/guide").Active(m.page == "guide")
nav.ContentLink(api, "Component API").
    Description("Read the source-owned recipes.").
    Value("/components").Active(m.page == "components")
navigationmenuview.View(c, "Navigation menu", nav).FillWidth().Height(36)`,
			PageSnippetZh: `// m.navigation.Selected 保存上一次绘制中选中的链接。
products, docs, guide, api := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
nav := m.navigation
switch nav.Selected {
case docs:
    m.page = "docs"
case guide:
    m.page = "guide"
case api:
    m.page = "components"
}
nav.Selected = gfx.ID{}
nav.Reset()
nav.Trigger(products, "Products").Hint("Open product navigation")
nav.Link(docs, "Docs").Value("/docs").Active(m.page == "docs")
nav.ContentLink(guide, "Getting started").
    Description("Build a native window and retained scene.").
    Value("/guide").Active(m.page == "guide")
nav.ContentLink(api, "Component API").
    Description("Read the source-owned recipes.").
    Value("/components").Active(m.page == "components")
navigationmenuview.View(c, "Navigation menu", nav).FillWidth().Height(36)`},
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
})`,
			PageSnippet: `products, docs, guide, api := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
nav := m.navigation
switch nav.Selected {
case docs:
    m.page = "docs"
case guide:
    m.page = "guide"
case api:
    m.page = "components"
}
nav.Selected = gfx.ID{}
nav.Reset()
nav.DirectContent = true
nav.Open = products
nav.Trigger(products, "Products").Hint("Open product navigation")
nav.Link(docs, "Docs").Value("/docs").Active(m.page == "docs")
nav.ContentLink(guide, "Getting started").
    Description("Build a native window and retained scene.").
    Value("/guide").Active(m.page == "guide")
nav.ContentLink(api, "Component API").
    Description("Read the source-owned recipes.").
    Value("/components").Active(m.page == "components")
navigationmenuview.View(c, "Navigation menu", nav).FillWidth().Height(36)`,
			PageSnippetZh: `products, docs, guide, api := ui.ID(c), ui.ID(c), ui.ID(c), ui.ID(c)
nav := m.navigation
switch nav.Selected {
case docs:
    m.page = "docs"
case guide:
    m.page = "guide"
case api:
    m.page = "components"
}
nav.Selected = gfx.ID{}
nav.Reset()
nav.DirectContent = true
nav.Open = products
nav.Trigger(products, "Products").Hint("Open product navigation")
nav.Link(docs, "Docs").Value("/docs").Active(m.page == "docs")
nav.ContentLink(guide, "Getting started").
    Description("Build a native window and retained scene.").
    Value("/guide").Active(m.page == "guide")
nav.ContentLink(api, "Component API").
    Description("Read the source-owned recipes.").
    Value("/components").Active(m.page == "components")
navigationmenuview.View(c, "Navigation menu", nav).FillWidth().Height(36)`},
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

	// navigationMenuDemo is the preview's page control. The preview has one
	// navigation menu.
	navigationMenuDemo      navigationmenuview.NavigationMenu
	navigationMenuDemoState *State
)

func (state *State) paintNavigationMenu(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	nav := &navigationMenuDemo
	if navigationMenuDemoState != state {
		navigationMenuDemoState = state
		nav.Open, nav.Selected = gfx.ID{}, gfx.ID{}
		nav.DirectContent = false
	}
	direct := state.exampleID == "direct-content"
	nav.DirectContent = direct
	if direct {
		nav.Open = navigationMenuTrigger
	}
	open := direct || !nav.Open.IsZero()
	status := state.Text("Products 已关闭", "Products closed")
	if open {
		status = state.Text("Products 已打开；Escape 使用稳定 dismiss ID", "Products open; Escape dismisses through a stable ID")
	}

	nav.Scope = navigationMenuScope
	nav.DismissID = navigationMenuDismiss
	nav.Layer = 2
	nav.Content = gfx.Rect{Width: min(float32(420), size.Width-32)}
	nav.Reset()
	nav.Trigger(navigationMenuTrigger, "Products").Hint(state.Text("打开产品导航", "Open product navigation"))
	nav.Link(navigationMenuDocs, "Docs").Value("/docs").Active(state.values[0] == 1)
	nav.ContentLink(navigationMenuGuide, "Getting started").
		Description("Build a native window and retained scene.").
		Value("/guide").Active(state.values[0] == 2)
	nav.ContentLink(navigationMenuAPI, "Component API").
		Description("Read the source-owned recipes.").
		Value("/components").Active(state.values[0] == 3)

	state.drawPage(canvas, theme, size, 620, "Navigation Menu", func(c ui.C) {
		caption(c, status)
		ui.Row(c).Gap(12).Items(ui.Center).Children(func() {
			ui.Text(c, "Viva").FontSize(theme.Metrics.FontSizeM + 2).Weight(gfx.WeightSemibold)
			navigationmenuview.View(c, "Navigation menu", nav).Grow(1).Height(36)
		})
	})

	// Mirror the control's caller-owned state into the demo's flags and
	// values, which the tests and the caption read.
	state.flags &^= 1
	if !nav.Open.IsZero() {
		state.flags |= 1
	}
	state.values[0] = 0
	switch nav.Selected {
	case navigationMenuDocs:
		state.values[0] = 1
	case navigationMenuGuide:
		state.values[0] = 2
	case navigationMenuAPI:
		state.values[0] = 3
	}
	nav.Selected = gfx.ID{}
}
