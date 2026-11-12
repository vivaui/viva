package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var (
	singleSelectUI      = tree.Feature("single-select")
	singleSelectInput   = tree.ID(singleSelectUI, "input")
	singleSelectTrigger = tree.ID(singleSelectUI, "trigger")
	singleSelectClear   = tree.ID(singleSelectUI, "clear")
	singleSelectDismiss = tree.ID(singleSelectUI, "dismiss")
	singleSelectScope   = tree.ID(singleSelectUI, "scope")
	singleSelectScroll  = tree.ID(singleSelectUI, "scroll")
	singleSelectOptions = tree.Sub(singleSelectUI, tree.Name("option"))
)

var singleSelectComponent = Component{
	ID: "single-select", Title: "Single Select", Category: "Forms", Summary: "可搜索、单选并创建条目，选择后收起列表。",
	initialize: func(state *State) {
		state.singleSelect = &components.SingleSelectState{AllowCreate: true, Open: true}
		state.singleSelect.SetOptions([]components.SingleSelectOption{{Value: state.Text("开发环境", "Development")}, {Value: state.Text("测试环境", "Testing")}, {Value: state.Text("生产环境", "Production")}})
		state.singleSelect.SetValue(state.Text("开发环境", "Development"))
	},
	editable: func(example string) (editableField, bool) {
		return editableField{ID: singleSelectInput}, example != "disabled"
	},
	editor: func(state *State) *text.Editor { return &state.singleSelect.Query },
	key: func(state *State, event gfx.KeyEvent) gfx.EventResult {
		if state.exampleID == "disabled" || event.Target != singleSelectInput && (!state.singleSelect.Open || event.Key != gfx.KeyEscape && event.Key != gfx.KeyTab) {
			return gfx.EventResult{}
		}
		return state.singleSelect.Key(event)
	},
	scroll: func(state *State, event gfx.ScrollEvent) gfx.EventResult {
		return gfx.EventResult{Handled: state.singleSelect.Open, Repaint: state.singleSelect.Open && state.singleSelect.Wheel(event)}
	},
	paint: (*State).paintSingleSelect,
	Examples: []Example{
		{ID: "default", Title: "单选与创建", Description: "选择已有条目或输入新名称后回车创建；选中后收起，支持清空。", Height: 400, Snippet: `selection := components.SingleSelectState{AllowCreate: true}
selection.SetOptions([]components.SingleSelectOption{
    {Value: "development", Label: "Development"},
    {Value: "testing", Label: "Testing"},
})
// Bind native input to &selection.Query and keys to selection.Key(event).
// Route clicks to selection.Choose(index) and selection.Clear().
components.PaintSingleSelect(canvas, theme, bounds, &selection, options)`},
		{ID: "disabled", Title: "禁用", Description: "保留当前值，禁止输入和选择。", Height: 280, Snippet: `options.Disabled = true
components.PaintSingleSelect(canvas, theme, bounds, &selection, options)`},
	},
}

func (state *State) paintSingleSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var field ui.Box
	state.drawPage(canvas, theme, size, 420, state.Text("Single Select · 单选分组", "Single Select · Groups"), func(c ui.C) {
		ui.Text(c, state.Text("输入筛选，回车创建；每次保留一个选项。", "Type to filter or create; keep one selected value.")).Muted().FontSize(theme.Metrics.FontSizeS)
		field = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
	})
	components.PaintSingleSelect(canvas, theme, field.Rect(), state.singleSelect, components.SingleSelectOptions{
		InputID: singleSelectInput, TriggerID: singleSelectTrigger, ClearID: singleSelectClear, DismissID: singleSelectDismiss, Scope: singleSelectScope, ScrollID: singleSelectScroll, OptionIDs: singleSelectOptions,
		Label: state.Text("分组", "Group"), Placeholder: state.Text("选择或创建分组", "Select or create a group"), EmptyLabel: state.Text("输入即可创建分组", "Type to create a group"), CreatePrefix: state.Text("创建分组：", "Create group: "), ClearLabel: state.Text("清空分组", "Clear group"),
		Disabled: state.exampleID == "disabled", Viewport: gfx.Rect{Width: size.Width, Height: size.Height}, Layer: 6,
	})
	if state.exampleID == "disabled" {
		return
	}
	switch {
	case canvas.Clicked(singleSelectInput):
		state.singleSelect.Open = true
	case canvas.Clicked(singleSelectTrigger):
		state.singleSelect.Open = !state.singleSelect.Open
	case canvas.Clicked(singleSelectClear):
		state.singleSelect.Clear()
	case canvas.Clicked(singleSelectDismiss):
		state.singleSelect.Dismiss()
	default:
		if i, ok := canvas.ClickedIn(singleSelectOptions); ok {
			state.singleSelect.Choose(i)
		}
	}
}
