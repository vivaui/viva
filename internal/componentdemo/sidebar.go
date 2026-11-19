package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/sidebarview"
)

var sidebarComponent = Component{
	ID: "sidebar", Title: "Sidebar", Category: "Navigation", Summary: "桌面应用导航栏、表面变体与图标折叠状态。",
	paint: (*State).paintSidebar,
	Examples: []Example{
		{ID: "default", Title: "应用导航", Description: "点击菜单项切换调用方持有的当前页面。", Snippet: `rail := ui.Slot(c).WidthPercent(42).MaxWidth(260).FillHeight()
items := tree.Sub(page, tree.Name("item"))
sidebar := components.BeginSidebar(canvas, theme, rail.Rect(), "Viva Studio")
sidebar.AddGroup("Workspace")
sidebar.AddMenuItem(components.SidebarMenuItem{
    ID: items.At(0), Label: "Overview", Icon: "O", Active: selected == 0,
})
sidebar.AddMenuItem(components.SidebarMenuItem{
    ID: items.At(1), Label: "Components", Icon: "C", Active: selected == 1,
})
if index, ok := canvas.ClickedIn(items); ok { selected = int(index) }`,
			PageSnippet: `m.nav.Reset().Title("Viva Studio").Fixed().
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").Badge("12").Active(m.section == "overview").
    Item(m.componentsID, "Components").Icon("C").Active(m.section == "components").
        Sub(m.buttonsID, "Buttons").SubActive(m.section == "buttons").
        Sub(m.inputsID, "Inputs").SubActive(m.section == "inputs").
    Item(m.benchmarksID, "Benchmarks").Icon("B").Active(m.section == "benchmarks").
    Account(m.accountID, "Skyler").Icon("S")
m.nav.OnSelect = m.selectSection
sidebarview.View(c, "Application navigation", &m.nav).Width(240).FillHeight()`,
			PageSnippetZh: `m.nav.Reset().Title("Viva Studio").Fixed().
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").Badge("12").Active(m.section == "overview").
    Item(m.componentsID, "Components").Icon("C").Active(m.section == "components").
        Sub(m.buttonsID, "Buttons").SubActive(m.section == "buttons").
        Sub(m.inputsID, "Inputs").SubActive(m.section == "inputs").
    Item(m.benchmarksID, "Benchmarks").Icon("B").Active(m.section == "benchmarks").
    Account(m.accountID, "Skyler").Icon("S")
m.nav.OnSelect = m.selectSection
sidebarview.View(c, "Application navigation", &m.nav).Width(240).FillHeight()`},
		{ID: "floating", Title: "浮动变体", Description: "使用官方 Floating 表面与描边菜单项。", Snippet: `rail := ui.Slot(c).WidthPercent(42).MaxWidth(260).FillHeight()
items := tree.Sub(page, tree.Name("item"))
sidebar := components.BeginSidebarWithOptions(canvas, theme, rail.Rect(), "Viva Studio",
    components.SidebarOptions{Collapsible: components.SidebarFixed, Variant: components.SidebarFloating})
sidebar.AddGroup("Workspace")
sidebar.AddMenuItem(components.SidebarMenuItem{ID: items.At(0), Label: "Overview", Icon: "O"})
sidebar.AddMenuItem(components.SidebarMenuItem{
    ID: items.At(1), Label: "Components", Icon: "C",
    Variant: components.SidebarMenuOutline,
})`,
			PageSnippet: `m.nav.Reset().Title("Viva Studio").Floating().Fixed().
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").
    Item(m.componentsID, "Components").Icon("C").Outline().
    Item(m.benchmarksID, "Benchmarks").Icon("B").
    Account(m.accountID, "Skyler").Icon("S")
sidebarview.View(c, "Application navigation", &m.nav).Width(240).FillHeight()`,
			PageSnippetZh: `m.nav.Reset().Title("Viva Studio").Floating().Fixed().
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").
    Item(m.componentsID, "Components").Icon("C").Outline().
    Item(m.benchmarksID, "Benchmarks").Icon("B").
    Account(m.accountID, "Skyler").Icon("S")
sidebarview.View(c, "Application navigation", &m.nav).Width(240).FillHeight()`},
		{ID: "collapsed", Title: "图标折叠", Description: "切换完整导航和桌面图标栏；折叠状态由调用方持有。", Snippet: `rail := ui.Slot(c).FillHeight()
if collapsed {
    rail.Width(theme.Metrics.ControlHeight + theme.Metrics.GapItem*2)
} else {
    rail.WidthPercent(42).MaxWidth(260)
}
sidebar := components.BeginSidebarWithOptions(canvas, theme, rail.Rect(), "Viva Studio",
    components.SidebarOptions{Collapsible: components.SidebarIcon, Collapsed: collapsed})
if ui.Button(c, "Toggle sidebar").Outline().Hit(collapseID, "Toggle sidebar").Clicked() {
    collapsed = !collapsed
}
sidebar.AddMenuItem(components.SidebarMenuItem{ID: tree.ID(page, "overview"), Label: "Overview", Icon: "O"})`,
			PageSnippet: `m.nav.Reset().Title("Viva Studio").IconCollapse().Collapsed(m.collapsed).
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").Active(m.section == "overview").
    Item(m.componentsID, "Components").Icon("C").Active(m.section == "components").
    Item(m.benchmarksID, "Benchmarks").Icon("B").
    Account(m.accountID, "Skyler").Icon("S")
m.nav.OnSelect = m.selectSection
sidebarview.View(c, "Application navigation", &m.nav).
    Width(c.Theme().Metrics.ControlHeight + c.Theme().Metrics.GapItem*2).FillHeight()`,
			PageSnippetZh: `m.nav.Reset().Title("Viva Studio").IconCollapse().Collapsed(m.collapsed).
    Group("Workspace").
    Item(m.overviewID, "Overview").Icon("O").Active(m.section == "overview").
    Item(m.componentsID, "Components").Icon("C").Active(m.section == "components").
    Item(m.benchmarksID, "Benchmarks").Icon("B").
    Account(m.accountID, "Skyler").Icon("S")
m.nav.OnSelect = m.selectSection
sidebarview.View(c, "Application navigation", &m.nav).
    Width(c.Theme().Metrics.ControlHeight + c.Theme().Metrics.GapItem*2).FillHeight()`},
	},
}

var (
	sidebarUI = tree.Feature("sidebar")
	// sidebarItem's item i is the i-th menu item.
	sidebarItem     = tree.Sub(sidebarUI, tree.Name("item"))
	sidebarSub      = tree.Sub(sidebarUI, tree.Name("sub"))
	sidebarAccount  = tree.ID(sidebarUI, "account")
	sidebarCollapse = tree.ID(sidebarUI, "collapse")
)

// sidebarDemo is the demo's reusable sidebar declaration. State cannot hold it
// without widening the preview model, so the demo keeps one configured view
// and rebuilds its declarations in every paint; handlers are made once.
var sidebarDemo = sidebarview.New()

func (state *State) paintSidebar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	collapsed := state.exampleID == "collapsed" && state.flags&1 != 0
	// A collapsed icon bar is one control wide plus its gaps; otherwise the
	// sidebar takes 42% of the stage, at most 260.
	var rail ui.Box
	state.drawStage(canvas, theme, size, func(c ui.C) {
		ui.Row(c).Fill().Pad(theme.Metrics.PadSurface).Gap(20).Children(func() {
			bar := sidebarDemo.Reset().Title("Viva Studio").Standard().Fixed().Collapsed(collapsed)
			switch state.exampleID {
			case "floating":
				bar.Floating()
			case "collapsed":
				bar.IconCollapse()
			}
			bar.Group("Workspace").
				Item(sidebarItem.At(0), "Overview").Icon("O").Badge("12").Active(state.values[0] == 0)
			if state.exampleID == "floating" {
				bar.Item(sidebarItem.At(1), "Components").Icon("C").Active(state.values[0] == 1).Outline()
			} else {
				bar.Item(sidebarItem.At(1), "Components").Icon("C").Active(state.values[0] == 1)
			}
			if state.exampleID == "default" {
				bar.Sub(sidebarSub.At(0), "Buttons").SubActive(state.values[1] == 0).
					Sub(sidebarSub.At(1), "Inputs").SubActive(state.values[1] == 1)
			}
			bar.Item(sidebarItem.At(2), "Benchmarks").Icon("B").Active(state.values[0] == 2).
				Account(sidebarAccount, "Skyler").Icon("S")
			rail = bar.View(c, "Application navigation")
			if collapsed {
				rail.Width(theme.Metrics.ControlHeight + theme.Metrics.GapItem*2).FillHeight()
			} else {
				rail.WidthPercent(42).MaxWidth(260).FillHeight()
			}
			ui.Column(c).Grow(1).Basis(0).Gap(16).PadTop(14).Children(func() {
				ui.Text(c, sidebarSelectionLabel(state.values[0])).FontSize(theme.Metrics.FontSizeM + 4).Weight(gfx.WeightSemibold)
				if state.exampleID == "collapsed" {
					label := "Collapse sidebar"
					if collapsed {
						label = "Expand sidebar"
					}
					if ui.Button(c, label).Outline().Self(ui.Start).Hit(sidebarCollapse, label).Clicked() {
						state.flags ^= 1
					}
				}
			})
		})
	})

	if index, ok := sidebarDemo.Selected().In(sidebarItem); ok {
		state.values[0] = int32(index)
	}
	if index, ok := sidebarDemo.Selected().In(sidebarSub); ok {
		state.values[1] = int32(index)
	}
}

func sidebarSelectionLabel(value int32) string {
	switch value {
	case 1:
		return "Components"
	case 2:
		return "Benchmarks"
	default:
		return "Overview"
	}
}
