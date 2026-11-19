package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/multiselect"
)

var (
	multiSelectUI      = tree.Feature("multi-select")
	multiSelectOptions = tree.Sub(multiSelectUI, tree.Name("option"))
	multiSelectRemove  = tree.Sub(multiSelectUI, tree.Name("remove"))
)

// multiSelectDemo is the page-owned picker of one example, made once so a warm
// paint reuses it instead of building a new control every frame.
type multiSelectDemo struct {
	picker *multiselect.Picker
}

var multiSelectComponent = Component{
	ID: "multi-select", Title: "Multi Select", Category: "Forms", Summary: "可搜索、多选并创建条目，已选项可逐个删除。",
	initialize: func(state *State) {
		state.multiSelect = &components.MultiSelectState{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
		state.multiSelect.SetOptions([]components.MultiSelectOption{{Value: state.Text("开发", "Development")}, {Value: state.Text("测试", "Testing")}, {Value: state.Text("生产", "Production")}})
		state.multiSelect.SetValues([]string{state.Text("开发", "Development")})
	},
	paint: (*State).paintMultiSelect,
	Examples: []Example{
		{ID: "default", Title: "多选与创建", Description: "选择多个已有选项，或输入新条目后回车创建；点击空白处收起。", Height: 400, Snippet: `selection := components.MultiSelectState{AllowCreate: true}
selection.SetOptions([]components.MultiSelectOption{
    {Value: "development", Label: "Development"},
    {Value: "testing", Label: "Testing"},
    {Value: "production", Label: "Production"},
})
// Connect the native input provider to &selection.Query.
// Route input keys to selection.Key(event), clicks to Choose/Remove.
components.PaintMultiSelect(canvas, theme, bounds, &selection, options)`, PageSnippet: `tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "development", Label: "Development"},
    {Value: "testing", Label: "Testing"},
    {Value: "production", Label: "Production"},
})
tags.SetValues([]string{"development"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "Tags", Placeholder: "Select or create tags",
    EmptyLabel: "No matching options", CreatePrefix: "Create: ",
}).FillWidth()`, PageSnippetZh: `tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "开发", Label: "开发"},
    {Value: "测试", Label: "测试"},
    {Value: "生产", Label: "生产"},
})
tags.SetValues([]string{"开发"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "标签", Placeholder: "选择或输入标签",
    EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
}).FillWidth()`},
		{ID: "disabled", Title: "禁用", Description: "保留已选项，禁止编辑、展开或删除。", Height: 280, Snippet: `options.Disabled = true
components.PaintMultiSelect(canvas, theme, bounds, &selection, options)`, PageSnippet: `tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "development", Label: "Development"},
    {Value: "testing", Label: "Testing"},
    {Value: "production", Label: "Production"},
})
tags.SetValues([]string{"development"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "Tags", Placeholder: "Select or create tags",
    EmptyLabel: "No matching options", CreatePrefix: "Create: ",
}).FillWidth().Disabled(true)`, PageSnippetZh: `tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "开发", Label: "开发"},
    {Value: "测试", Label: "测试"},
    {Value: "生产", Label: "生产"},
})
tags.SetValues([]string{"开发"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "标签", Placeholder: "选择或输入标签",
    EmptyLabel: "没有匹配项", CreatePrefix: "创建：",
}).FillWidth().Disabled(true)`},
	},
}

func (state *State) paintMultiSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 420, state.Text("Multi Select · 多选标签", "Multi Select · Tags"), func(c ui.C) {
		ui.Text(c, state.Text("输入筛选，回车创建，支持中文输入法。", "Type to filter, press Enter to create. Native IME supported.")).Muted().FontSize(theme.Metrics.FontSizeS)
		demo := ui.State[multiSelectDemo](c, "multi-select-picker")
		if demo.picker == nil {
			demo.picker = multiselect.New(state.multiSelect)
		}
		demo.picker.View(c, multiselect.Spec{
			Label: state.Text("标签", "Tags"), Placeholder: state.Text("选择或输入标签", "Select or create tags"),
			EmptyLabel: state.Text("没有匹配项", "No matching options"), CreatePrefix: state.Text("创建：", "Create: "),
			Options: multiSelectOptions, Removes: multiSelectRemove,
		}).FillWidth().Disabled(state.exampleID == "disabled")
	})
}
