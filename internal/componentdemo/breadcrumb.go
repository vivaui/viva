package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/breadcrumb"
)

var breadcrumbComponent = Component{
	ID: "breadcrumb", Title: "Breadcrumb", Category: "Navigation", Summary: "带链接、省略项和当前位置的层级导航。",
	paint:      (*State).paintBreadcrumb,
	PageSource: "ui/breadcrumb/breadcrumb.go",
	Examples: []Example{
		{ID: "default", Title: "项目路径", Description: "链接动作由调用方持有，末项仅表示当前位置。", Snippet: `links := tree.Sub(page, tree.Name("link"))
breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
breadcrumb.AddLink(links.At(0), "Workspace")
breadcrumb.AddLink(links.At(1), "Viva")
breadcrumb.AddCurrent("Preview")
if index, ok := canvas.ClickedIn(links); ok {
    selected = int32(index + 1)
}`, PageSnippet: `m.path = breadcrumb.New()
// m.places is a []breadcrumb.Item kept in the model.
m.path.View(c, "Location", breadcrumb.Spec{Items: m.places})
if i, ok := m.path.Clicked(); ok {
    m.goTo(i)
}`, PageSnippetZh: `m.path = breadcrumb.New()
// m.places 是保存在模型里的 []breadcrumb.Item。
m.path.View(c, "Location", breadcrumb.Spec{Items: m.places})
if i, ok := m.path.Clicked(); ok {
    m.goTo(i)
}`},
		{ID: "overflow", Title: "折叠路径", Description: "Max 把中间的层级收进省略号，点开菜单选择。", Snippet: `links := tree.Sub(page, tree.Name("link"))
breadcrumb := components.BeginBreadcrumb(canvas, theme, trail.Rect())
breadcrumb.AddLink(links.At(0), "Workspace")
breadcrumb.AddEllipsis(tree.ID(page, "more"))
breadcrumb.AddLink(links.At(1), "Components")
breadcrumb.AddCurrent("Breadcrumb")`, PageSnippet: `m.path.View(c, "Location", breadcrumb.Spec{
    Items: m.places,
    Max:   3,
})`, PageSnippetZh: `m.path.View(c, "Location", breadcrumb.Spec{
    Items: m.places,
    Max:   3,
})`},
	},
}

type breadcrumbDemo struct {
	path   *breadcrumb.Breadcrumb
	places []breadcrumb.Item
}

var breadcrumbDemos = map[*State]*breadcrumbDemo{}

func breadcrumbDemoFor(state *State) *breadcrumbDemo {
	if demo := breadcrumbDemos[state]; demo != nil {
		return demo
	}
	demo := &breadcrumbDemo{path: breadcrumb.New(), places: []breadcrumb.Item{
		{Label: state.Text("工作区", "Workspace")}, {Label: state.Text("项目", "Projects")}, {Label: state.Text("组件", "Components")},
		{Label: "Viva"}, {Label: state.Text("预览", "Preview")},
	}}
	breadcrumbDemos[state] = demo
	return demo
}

func (state *State) paintBreadcrumb(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := breadcrumbDemoFor(state)
	caption := state.Text("选择上级路径以返回。", "Choose a parent path to go back.")
	switch state.values[0] {
	case 1:
		caption = state.Text("已选择：", "Selected: ") + demo.places[0].Label
	case 2:
		caption = state.Text("已选择：", "Selected: ") + demo.places[len(demo.places)-2].Label
	}
	spec := breadcrumb.Spec{Items: demo.places, MoreLabel: state.Text("更多", "More")}
	if state.exampleID == "overflow" {
		spec.Max = 3
	}
	state.drawPage(canvas, theme, size, 620, state.Text("Breadcrumb · 当前上下文", "Breadcrumb · Current context"), func(c ui.C) {
		demo.path.View(c, state.Text("位置", "Location"), spec)
		if i, ok := demo.path.Clicked(); ok {
			state.values[0] = 2
			if i == 0 {
				state.values[0] = 1
			}
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
