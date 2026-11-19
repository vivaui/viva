package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
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
			PageSnippet: `type navItem struct {
    Icon, Label, Trailing string
}
navItems := []navItem{
    {Icon: "O", Label: "Overview", Trailing: "12"},
    {Icon: "C", Label: "Components"},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Row(c).Gap(20).Children(func() {
    ui.Column(c).Width(240).FillHeight().Gap(8).Children(func() {
        ui.Text(c, "Viva Studio").Bold()
        ui.Text(c, "Workspace").Muted()
        ui.List(c, &m.nav, len(navItems), func(row ui.ListRow) {
            item := navItems[row.Index]
            row.Element.PadX(8).Gap(8).Items(ui.Center).Children(func() {
                ui.Text(row.C, item.Icon)
                ui.Text(row.C, item.Label).Grow(1)
                ui.Text(row.C, item.Trailing).Muted()
            })
        }).Grow(1)
        ui.Column(c).Gap(2).PadLeft(24).Children(func() {
            if ui.Button(c, "Buttons").Ghost().Small().Self(ui.Start).Clicked() {
                m.section = "buttons"
            }
            if ui.Button(c, "Inputs").Ghost().Small().Self(ui.Start).Clicked() {
                m.section = "inputs"
            }
        })
        ui.Divider(c)
        if ui.Button(c, "Settings").Ghost().Small().Self(ui.Start).Clicked() {
            m.section = "settings"
        }
        if ui.Button(c, "Skyler").Ghost().Self(ui.Start).Clicked() {
            m.account = true
        }
    })
    ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
        ui.Text(c, m.selection).Bold()
    })
})`,
			PageSnippetZh: `type navItem struct {
    Icon, Label, Trailing string
}
navItems := []navItem{
    {Icon: "O", Label: "Overview", Trailing: "12"},
    {Icon: "C", Label: "Components"},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Row(c).Gap(20).Children(func() {
    ui.Column(c).Width(240).FillHeight().Gap(8).Children(func() {
        ui.Text(c, "Viva Studio").Bold()
        ui.Text(c, "Workspace").Muted()
        ui.List(c, &m.nav, len(navItems), func(row ui.ListRow) {
            item := navItems[row.Index]
            row.Element.PadX(8).Gap(8).Items(ui.Center).Children(func() {
                ui.Text(row.C, item.Icon)
                ui.Text(row.C, item.Label).Grow(1)
                ui.Text(row.C, item.Trailing).Muted()
            })
        }).Grow(1)
        ui.Column(c).Gap(2).PadLeft(24).Children(func() {
            if ui.Button(c, "Buttons").Ghost().Small().Self(ui.Start).Clicked() {
                m.section = "buttons"
            }
            if ui.Button(c, "Inputs").Ghost().Small().Self(ui.Start).Clicked() {
                m.section = "inputs"
            }
        })
        ui.Divider(c)
        if ui.Button(c, "Settings").Ghost().Small().Self(ui.Start).Clicked() {
            m.section = "settings"
        }
        if ui.Button(c, "Skyler").Ghost().Self(ui.Start).Clicked() {
            m.account = true
        }
    })
    ui.Column(c).Grow(1).Basis(0).Gap(16).Children(func() {
        ui.Text(c, m.selection).Bold()
    })
})`},
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
			PageSnippet: `type navItem struct {
    Icon, Label string
    Outline     bool
}
navItems := []navItem{
    {Icon: "O", Label: "Overview"},
    {Icon: "C", Label: "Components", Outline: true},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Column(c).Width(240).FillHeight().Pad(c.Theme().Metrics.PadSurface).
    Background(c.Theme().Card).Border(1, c.Theme().Border).
    Radius(c.Theme().Metrics.RadiusSurface).Gap(8).Children(func() {
        ui.Text(c, "Viva Studio").Bold()
        ui.Text(c, "Workspace").Muted()
        ui.List(c, &m.nav, len(navItems), func(row ui.ListRow) {
            item := navItems[row.Index]
            if item.Outline {
                ui.Button(row.C, item.Icon+"  "+item.Label).Outline().Small().Self(ui.Start)
                return
            }
            row.Element.PadX(8).Gap(8).Items(ui.Center).Children(func() {
                ui.Text(row.C, item.Icon)
                ui.Text(row.C, item.Label).Grow(1)
            })
        }).Grow(1)
        if ui.Button(c, "Settings").Ghost().Small().Self(ui.Start).Clicked() {
            m.section = "settings"
        }
    })`,
			PageSnippetZh: `type navItem struct {
    Icon, Label string
    Outline     bool
}
navItems := []navItem{
    {Icon: "O", Label: "Overview"},
    {Icon: "C", Label: "Components", Outline: true},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Column(c).Width(240).FillHeight().Pad(c.Theme().Metrics.PadSurface).
    Background(c.Theme().Card).Border(1, c.Theme().Border).
    Radius(c.Theme().Metrics.RadiusSurface).Gap(8).Children(func() {
        ui.Text(c, "Viva Studio").Bold()
        ui.Text(c, "Workspace").Muted()
        ui.List(c, &m.nav, len(navItems), func(row ui.ListRow) {
            item := navItems[row.Index]
            if item.Outline {
                ui.Button(row.C, item.Icon+"  "+item.Label).Outline().Small().Self(ui.Start)
                return
            }
            row.Element.PadX(8).Gap(8).Items(ui.Center).Children(func() {
                ui.Text(row.C, item.Icon)
                ui.Text(row.C, item.Label).Grow(1)
            })
        }).Grow(1)
        if ui.Button(c, "Settings").Ghost().Small().Self(ui.Start).Clicked() {
            m.section = "settings"
        }
    })`},
		{ID: "collapsed", Title: "图标折叠", Description: "切换完整导航和桌面图标栏。", Snippet: `rail := ui.Slot(c).FillHeight()
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
			PageSnippet: `type navItem struct {
    Icon, Label string
}
navItems := []navItem{
    {Icon: "O", Label: "Overview"},
    {Icon: "C", Label: "Components"},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Column(c).FillHeight().Gap(6).Children(func() {
    if m.collapsed {
        for _, item := range navItems {
            if ui.Button(c, item.Icon).Ghost().IconOnly().Clicked() {
                m.selection = item.Label
            }
        }
        if ui.Button(c, "»").Ghost().IconOnly().Clicked() {
            m.collapsed = false
        }
        return
    }
    ui.Text(c, "Viva Studio").Bold()
    for _, item := range navItems {
        if ui.Button(c, item.Icon+"  "+item.Label).Ghost().Self(ui.Start).Clicked() {
            m.selection = item.Label
        }
    }
    if ui.Button(c, "Collapse sidebar").Ghost().Self(ui.Start).Clicked() {
        m.collapsed = true
    }
})`,
			PageSnippetZh: `type navItem struct {
    Icon, Label string
}
navItems := []navItem{
    {Icon: "O", Label: "Overview"},
    {Icon: "C", Label: "Components"},
    {Icon: "B", Label: "Benchmarks"},
}
ui.Column(c).FillHeight().Gap(6).Children(func() {
    if m.collapsed {
        for _, item := range navItems {
            if ui.Button(c, item.Icon).Ghost().IconOnly().Clicked() {
                m.selection = item.Label
            }
        }
        if ui.Button(c, "»").Ghost().IconOnly().Clicked() {
            m.collapsed = false
        }
        return
    }
    ui.Text(c, "Viva Studio").Bold()
    for _, item := range navItems {
        if ui.Button(c, item.Icon+"  "+item.Label).Ghost().Self(ui.Start).Clicked() {
            m.selection = item.Label
        }
    }
    if ui.Button(c, "Collapse sidebar").Ghost().Self(ui.Start).Clicked() {
        m.collapsed = true
    }
})`},
	},
}

var (
	sidebarUI = tree.Feature("sidebar")
	// sidebarItem's item i is the i-th menu item.
	sidebarItem     = tree.Sub(sidebarUI, tree.Name("item"))
	sidebarSub      = tree.Sub(sidebarUI, tree.Name("sub"))
	sidebarSettings = tree.ID(sidebarUI, "settings")
	sidebarMore     = tree.ID(sidebarUI, "more")
	sidebarAccount  = tree.ID(sidebarUI, "account")
	sidebarCollapse = tree.ID(sidebarUI, "collapse")
)

func (state *State) paintSidebar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	options := components.SidebarOptions{Collapsible: components.SidebarFixed}
	if state.exampleID == "floating" {
		options.Variant = components.SidebarFloating
	}
	if state.exampleID == "collapsed" {
		options.Collapsible = components.SidebarIcon
		options.Collapsed = state.flags&1 != 0
	}
	// A collapsed icon bar is one control wide plus its gaps; otherwise the
	// sidebar takes 42% of the stage, at most 260.
	var rail ui.Box
	state.drawStage(canvas, theme, size, func(c ui.C) {
		ui.Row(c).Fill().Pad(theme.Metrics.PadSurface).Gap(20).Children(func() {
			rail = ui.Slot(c).FillHeight()
			if options.Collapsed {
				rail.Width(theme.Metrics.ControlHeight + theme.Metrics.GapItem*2)
			} else {
				rail.WidthPercent(42).MaxWidth(260)
			}
			ui.Column(c).Grow(1).Basis(0).Gap(16).PadTop(14).Children(func() {
				ui.Text(c, sidebarSelectionLabel(state.values[0])).FontSize(theme.Metrics.FontSizeM + 4).Weight(gfx.WeightSemibold)
				if state.exampleID == "collapsed" {
					label := "Collapse sidebar"
					if options.Collapsed {
						label = "Expand sidebar"
					}
					if ui.Button(c, label).Outline().Self(ui.Start).Hit(sidebarCollapse, label).Clicked() {
						state.flags ^= 1
					}
				}
			})
		})
	})

	sidebar := components.BeginSidebarWithOptions(canvas, theme, rail.Rect(), "Viva Studio", options)
	sidebar.AddFooterMenuItem(components.SidebarMenuItem{ID: sidebarAccount, Label: "Skyler", Icon: "S"})
	sidebar.AddGroup("Workspace")
	items := [...]components.SidebarMenuItem{
		{ID: sidebarItem.At(0), Label: "Overview", Icon: "O", Trailing: "12"},
		{ID: sidebarItem.At(1), Label: "Components", Icon: "C"},
		{ID: sidebarItem.At(2), Label: "Benchmarks", Icon: "B", Action: components.SidebarMenuAction{ID: sidebarMore, Icon: moreIcon, Label: "More", ShowOnHover: true}},
	}
	for index, item := range items {
		item.Active = state.values[0] == int32(index)
		if state.exampleID == "floating" && index == 1 {
			item.Variant = components.SidebarMenuOutline
		}
		sidebar.AddMenuItem(item)
		if index == 1 {
			sidebar.AddSubMenu([]components.SidebarSubItem{
				{ID: sidebarSub.At(0), Label: "Buttons", Active: state.values[1] == 0},
				{ID: sidebarSub.At(1), Label: "Inputs", Active: state.values[1] == 1},
			})
		}
	}
	sidebar.AddSeparator()
	sidebar.AddMenuItem(components.SidebarMenuItem{ID: sidebarSettings, Label: "Settings", Icon: "S", Size: components.SidebarMenuSizeSmall})

	if index, ok := canvas.ClickedIn(sidebarItem); ok {
		state.values[0] = int32(index)
	}
	if index, ok := canvas.ClickedIn(sidebarSub); ok {
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
