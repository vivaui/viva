package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var breadcrumbComponent = Component{
	ID: "breadcrumb", Title: "Breadcrumb", Category: "Navigation", Summary: "带链接、省略项和当前位置的层级导航。",
	paint: (*State).paintBreadcrumb,
	Examples: []Example{
		{ID: "default", Title: "项目路径", Description: "链接动作由调用方持有，末项仅表示当前位置。", Snippet: `links := tree.Sub(page, tree.Name("link"))
breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
breadcrumb.AddLink(links.At(0), "Workspace")
breadcrumb.AddLink(links.At(1), "Viva")
breadcrumb.AddCurrent("Preview")
if index, ok := canvas.ClickedIn(links); ok {
    selected = int32(index + 1)
}`, PageSnippet: `ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
    first := ui.Text(c, "Workspace").Muted().Hit(ui.ID(c), "Workspace")
    if first.Clicked() {
        m.parent = 0
    }
    ui.Text(c, "›").Muted()
    parent := ui.Text(c, "Viva").Muted().Hit(ui.ID(c), "Viva")
    if parent.Clicked() {
        m.parent = 1
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "Preview")
})
caption := "Choose a parent path to go back."
if m.parent == 0 {
    caption = "Selected: Workspace"
}
if m.parent == 1 {
    caption = "Selected: Parent"
}
ui.Text(c, caption).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
    first := ui.Text(c, "工作区").Muted().Hit(ui.ID(c), "工作区")
    if first.Clicked() {
        m.parent = 0
    }
    ui.Text(c, "›").Muted()
    parent := ui.Text(c, "Viva").Muted().Hit(ui.ID(c), "Viva")
    if parent.Clicked() {
        m.parent = 1
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "预览")
})
caption := "选择上级路径以返回。"
if m.parent == 0 {
    caption = "已选择：工作区"
}
if m.parent == 1 {
    caption = "已选择：上一级"
}
ui.Text(c, caption).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "overflow", Title: "折叠路径", Description: "长层级使用省略项保持紧凑。", Snippet: `links := tree.Sub(page, tree.Name("link"))
breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
breadcrumb.AddLink(links.At(0), "Workspace")
breadcrumb.AddEllipsis(tree.ID(page, "more"))
breadcrumb.AddLink(links.At(1), "Components")
breadcrumb.AddCurrent("Breadcrumb")`, PageSnippet: `ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
    first := ui.Text(c, "Workspace").Muted().Hit(ui.ID(c), "Workspace")
    if first.Clicked() {
        m.parent = 0
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "…").Muted()
    ui.Text(c, "›").Muted()
    parent := ui.Text(c, "Components").Muted().Hit(ui.ID(c), "Components")
    if parent.Clicked() {
        m.parent = 1
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "Breadcrumb")
})
caption := "Choose a parent path to go back."
if m.parent == 0 {
    caption = "Selected: Workspace"
}
if m.parent == 1 {
    caption = "Selected: Parent"
}
ui.Text(c, caption).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
    first := ui.Text(c, "工作区").Muted().Hit(ui.ID(c), "工作区")
    if first.Clicked() {
        m.parent = 0
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "…").Muted()
    ui.Text(c, "›").Muted()
    parent := ui.Text(c, "组件").Muted().Hit(ui.ID(c), "组件")
    if parent.Clicked() {
        m.parent = 1
    }
    ui.Text(c, "›").Muted()
    ui.Text(c, "Breadcrumb")
})
caption := "选择上级路径以返回。"
if m.parent == 0 {
    caption = "已选择：工作区"
}
if m.parent == 1 {
    caption = "已选择：上一级"
}
ui.Text(c, caption).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
	},
}

var (
	breadcrumbUI = tree.Feature("breadcrumb")
	// breadcrumbLink's item i is the i-th link of the path.
	breadcrumbLink = tree.Sub(breadcrumbUI, tree.Name("link"))
)

func (state *State) paintBreadcrumb(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	caption := state.Text("选择上级路径以返回。", "Choose a parent path to go back.")
	switch state.values[0] {
	case 1:
		caption = state.Text("已选择：工作区", "Selected: Workspace")
	case 2:
		caption = state.Text("已选择：上一级", "Selected: Parent")
	}
	workspace := state.Text("工作区", "Workspace")
	components := state.Text("组件", "Components")
	viva := "Viva"
	preview := state.Text("预览", "Preview")
	overflow := state.exampleID == "overflow"
	state.drawPage(canvas, theme, size, 620, state.Text("Breadcrumb · 当前上下文", "Breadcrumb · Current context"), func(c ui.C) {
		ui.Row(c).Gap(6).Items(ui.Center).Children(func() {
			link := ui.Text(c, workspace).Muted().Hit(breadcrumbLink.At(0), workspace)
			if link.Clicked() {
				state.values[0] = 1
			}
			ui.Text(c, "›").Muted()
			if overflow {
				ui.Text(c, "…").Muted()
				ui.Text(c, "›").Muted()
				link = ui.Text(c, components).Muted().Hit(breadcrumbLink.At(1), components)
				if link.Clicked() {
					state.values[0] = 2
				}
				ui.Text(c, "›").Muted()
				ui.Text(c, "Breadcrumb")
			} else {
				link = ui.Text(c, viva).Muted().Hit(breadcrumbLink.At(1), viva)
				if link.Clicked() {
					state.values[0] = 2
				}
				ui.Text(c, "›").Muted()
				ui.Text(c, preview)
			}
		})
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
