package componentdemo

import (
	"github.com/vivaui/viva/components"
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
}`},
		{ID: "overflow", Title: "折叠路径", Description: "长层级使用省略项保持紧凑。", Snippet: `links := tree.Sub(page, tree.Name("link"))
breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
breadcrumb.AddLink(links.At(0), "Workspace")
breadcrumb.AddEllipsis(tree.ID(page, "more"))
breadcrumb.AddLink(links.At(1), "Components")
breadcrumb.AddCurrent("Breadcrumb")`},
	},
}

var (
	breadcrumbUI = tree.Feature("breadcrumb")
	// breadcrumbLink's item i is the i-th link of the path.
	breadcrumbLink = tree.Sub(breadcrumbUI, tree.Name("link"))
	breadcrumbMore = tree.ID(breadcrumbUI, "more")
)

func (state *State) paintBreadcrumb(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var trail ui.Box
	state.page(canvas, theme, size, 620, state.Text("Breadcrumb · 当前上下文", "Breadcrumb · Current context"), func(f *ui.Frame) {
		trail = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
		caption := state.Text("选择上级路径以返回。", "Choose a parent path to go back.")
		switch state.values[0] {
		case 1:
			caption = state.Text("已选择：工作区", "Selected: Workspace")
		case 2:
			caption = state.Text("已选择：上一级", "Selected: Parent")
		}
		f.Text(caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
	breadcrumb.AddLink(breadcrumbLink.At(0), state.Text("工作区", "Workspace"))
	if state.exampleID == "overflow" {
		breadcrumb.AddEllipsis(breadcrumbMore)
		breadcrumb.AddLink(breadcrumbLink.At(1), state.Text("组件", "Components"))
		breadcrumb.AddCurrent("Breadcrumb")
	} else {
		breadcrumb.AddLink(breadcrumbLink.At(1), "Viva")
		breadcrumb.AddCurrent(state.Text("预览", "Preview"))
	}
	if index, ok := canvas.ClickedIn(breadcrumbLink); ok {
		state.values[0] = int32(index + 1)
	}
}
