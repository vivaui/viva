package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var singleSelectComponent = Component{
	ID: "single-select", Title: "Single Select", Category: "Forms", Summary: "应用持有选中值的页面 Select 单选。",
	paint: (*State).paintSingleSelect,
	Examples: []Example{
		{ID: "default", Title: "单选与创建", Description: "选择已有条目或输入新名称后回车创建；选中后收起，支持清空。", Height: 400, Snippet: `selection := components.SingleSelectState{AllowCreate: true}
selection.SetOptions([]components.SingleSelectOption{
    {Value: "development", Label: "Development"},
    {Value: "testing", Label: "Testing"},
})
// Bind native input to &selection.Query and keys to selection.Key(event).
// Route clicks to selection.Choose(index) and selection.Clear().
components.PaintSingleSelect(canvas, theme, bounds, &selection, options)`, PageSnippet: `group := "Development"
ui.Select(c, &group, []string{"Development", "Testing", "Production"}, nil).
    Label("Group").Placeholder("Select or create a group")`, PageSnippetZh: `group := "开发环境"
ui.Select(c, &group, []string{"开发环境", "测试环境", "生产环境"}, nil).
    Label("分组").Placeholder("选择或创建分组")`},
		{ID: "disabled", Title: "禁用", Description: "保留当前值，禁止输入和选择。", Height: 280, Snippet: `options.Disabled = true
components.PaintSingleSelect(canvas, theme, bounds, &selection, options)`, PageSnippet: `group := "Development"
ui.Select(c, &group, []string{"Development", "Testing", "Production"}, nil).
    Label("Group").Disabled(true)`, PageSnippetZh: `group := "开发环境"
ui.Select(c, &group, []string{"开发环境", "测试环境", "生产环境"}, nil).
    Label("分组").Disabled(true)`},
	},
}

// singleSelectDemo is the caller-owned value of the page-first Select.
type singleSelectDemo struct {
	group string
	ready bool
}

func (state *State) paintSingleSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	groups := []string{state.Text("开发环境", "Development"), state.Text("测试环境", "Testing"), state.Text("生产环境", "Production")}
	state.drawPage(canvas, theme, size, 420, state.Text("Single Select · 单选分组", "Single Select · Groups"), func(c ui.C) {
		ui.Text(c, state.Text("选择一个分组；应用持有当前值。", "Choose a group; the application owns the value.")).Muted().FontSize(theme.Metrics.FontSizeS)
		demo := ui.State[singleSelectDemo](c, "single-select-demo")
		if !demo.ready {
			demo.ready, demo.group = true, groups[0]
		}
		field := ui.Select(c, &demo.group, groups, nil).
			Label(state.Text("分组", "Group")).Placeholder(state.Text("选择或创建分组", "Select or create a group")).
			FillWidth()
		if state.exampleID == "disabled" {
			field.Disabled(true)
		}
	})
}
