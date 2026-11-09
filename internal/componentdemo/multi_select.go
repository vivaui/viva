package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var (
	multiSelectUI      = tree.Feature("multi-select")
	multiSelectInput   = tree.ID(multiSelectUI, "input")
	multiSelectTrigger = tree.ID(multiSelectUI, "trigger")
	multiSelectDismiss = tree.ID(multiSelectUI, "dismiss")
	multiSelectScope   = tree.ID(multiSelectUI, "scope")
	multiSelectOptions = tree.Sub(multiSelectUI, tree.Name("option"))
	multiSelectRemove  = tree.Sub(multiSelectUI, tree.Name("remove"))
)

var multiSelectComponent = Component{
	ID: "multi-select", Title: "Multi Select", Category: "Forms", Summary: "可搜索、多选并创建条目，已选项可逐个删除。",
	initialize: func(state *State) {
		state.multiSelect = &components.MultiSelectState{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
		state.multiSelect.SetOptions([]components.MultiSelectOption{{Value: state.Text("开发", "Development")}, {Value: state.Text("测试", "Testing")}, {Value: state.Text("生产", "Production")}})
		state.multiSelect.SetValues([]string{state.Text("开发", "Development")})
	},
	editable: func(example string) (editableField, bool) {
		return editableField{ID: multiSelectInput}, example != "disabled"
	},
	editor: func(state *State) *text.Editor { return &state.multiSelect.Query },
	key: func(state *State, event gfx.KeyEvent) gfx.EventResult {
		if state.exampleID == "disabled" {
			return gfx.EventResult{}
		}
		if event.Target != multiSelectInput && (!state.multiSelect.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
			return gfx.EventResult{}
		}
		return state.multiSelect.Key(event)
	},
	scroll: func(state *State, event gfx.ScrollEvent) gfx.EventResult {
		return gfx.EventResult{Handled: state.multiSelect.Open, Repaint: state.multiSelect.Open && state.multiSelect.Wheel(event)}
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
components.PaintMultiSelect(canvas, theme, bounds, &selection, options)`},
		{ID: "disabled", Title: "禁用", Description: "保留已选项，禁止编辑、展开或删除。", Height: 280, Snippet: `options.Disabled = true
components.PaintMultiSelect(canvas, theme, bounds, &selection, options)`},
	},
}

func (state *State) paintMultiSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var field ui.Box
	state.page(canvas, theme, size, 420, state.Text("Multi Select · 多选标签", "Multi Select · Tags"), func(f *ui.Frame) {
		f.Text(state.Text("输入筛选，回车创建，支持中文输入法。", "Type to filter, press Enter to create. Native IME supported.")).Muted().FontSize(theme.Metrics.FontSizeS)
		field = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
	})
	components.PaintMultiSelect(canvas, theme, field.Rect(), state.multiSelect, components.MultiSelectOptions{
		InputID: multiSelectInput, TriggerID: multiSelectTrigger, DismissID: multiSelectDismiss, Scope: multiSelectScope,
		OptionIDs: multiSelectOptions, RemoveIDs: multiSelectRemove, Label: state.Text("标签", "Tags"), Placeholder: state.Text("选择或输入标签", "Select or create tags"),
		EmptyLabel: state.Text("没有匹配项", "No matching options"), CreatePrefix: state.Text("创建：", "Create: "), Disabled: state.exampleID == "disabled",
		Viewport: gfx.Rect{Width: size.Width, Height: size.Height}, Layer: 6,
	})
	if state.exampleID == "disabled" {
		return
	}
	switch {
	case canvas.Clicked(multiSelectInput):
		state.multiSelect.Open = true
	case canvas.Clicked(multiSelectTrigger):
		state.multiSelect.Open = !state.multiSelect.Open
	case canvas.Clicked(multiSelectDismiss):
		state.multiSelect.Open = false
	default:
		if i, ok := canvas.ClickedIn(multiSelectOptions); ok {
			state.multiSelect.Choose(i)
		}
		if i, ok := canvas.ClickedIn(multiSelectRemove); ok {
			state.multiSelect.Remove(i)
		}
	}
}
