package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/multiselect"
)

var comboboxComponent = Component{
	ID: "combobox", Title: "Combobox", Category: "Forms", Summary: "外部持有查询、展开和选择状态的可编辑选择器。",
	paint: (*State).paintCombobox, initialize: initializeCombobox,
	Examples: []Example{
		{ID: "default", Title: "选择框架", Description: "输入区域、弹层和选项均使用生产 Combobox。", Height: 180, Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
    InputID: inputID, ClearID: clearID, Label: "Framework", Value: values[selected],
    Query: query, Open: open, Content: content, Scope: scope, DismissID: dismissID,
    ShowTrigger: true, ShowClear: selected > 0, Layer: 6,
})
combo.AddLabel("Available targets")
// Highlighted is the row the pointer or the arrow keys are on; Selected keeps
// its check. Enter chooses the highlighted row.
combo.AddItem(components.ComboboxItem{ID: opt.At(0), Label: "Viva Native",
    Selected: selected == 1, Highlighted: highlighted == 0})
combo.AddItem(components.ComboboxItem{ID: opt.At(1), Label: "WebGPU",
    Selected: selected == 2, Highlighted: highlighted == 1})
combo.AddSeparator()
combo.AddItem(components.ComboboxItem{
    ID: opt.At(2), Label: "Headless tests", Selected: selected == 3,
})
combo.End()`, PageSnippet: `targets := []string{"Viva Native", "WebGPU", "Headless tests"}
hints := []string{"Native desktop", "Optional web target", "Windowless testing"}
ui.Combobox(c, &m.target, targets, nil, "Framework").Clearable().
    Describe(func(index int) ui.ComboboxOption {
        return ui.ComboboxOption{Detail: hints[index]}
    })`, PageSnippetZh: `targets := []string{"Viva Native", "WebGPU", "Headless tests"}
hints := []string{"原生桌面", "可选 Web 目标", "无窗口测试"}
ui.Combobox(c, &m.target, targets, nil, "框架").Clearable().
    Describe(func(index int) ui.ComboboxOption {
        return ui.ComboboxOption{Detail: hints[index]}
    })`},
		{ID: "empty", Title: "空结果", Description: "查询无匹配项时显示生产空状态。", Height: 160, Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
    InputID: inputID, Label: "Framework", Value: values[selected], Query: "mobile",
    Open: open, Content: content, Scope: scope, DismissID: dismissID, Layer: 6,
})
if open {
    combo.AddEmpty("No matching frameworks")
}
combo.End()`, PageSnippet: `frameworks := []string{"Viva Native", "WebGPU", "Headless tests"}
ui.Combobox(c, &m.framework, frameworks, nil, "Framework").
    Empty("No matching frameworks").InitiallyOpen(true).InitialQuery("mobile")`, PageSnippetZh: `frameworks := []string{"Viva Native", "WebGPU", "Headless tests"}
ui.Combobox(c, &m.framework, frameworks, nil, "框架").
    Empty("没有匹配的框架").InitiallyOpen(true).InitialQuery("mobile")`},
		{ID: "invalid", Title: "无效状态", Description: "保留选项弹层并突出输入边框。", Height: 180, Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
    InputID: inputID, ClearID: clearID, Label: "Framework", Value: values[selected],
    Query: query, Open: open, Invalid: true, Content: content, Scope: scope,
    DismissID: dismissID, ShowTrigger: true, ShowClear: selected > 0, Layer: 6,
})
combo.AddLabel("Available targets")
combo.AddItem(components.ComboboxItem{ID: opt.At(0), Label: "Viva Native", Selected: selected == 1})
combo.AddItem(components.ComboboxItem{ID: opt.At(1), Label: "WebGPU", Selected: selected == 2})
combo.AddItem(components.ComboboxItem{
    ID: opt.At(2), Label: "Headless tests", Selected: selected == 3,
})
combo.End()`, PageSnippet: `frameworks := []string{"Viva Native", "WebGPU", "Headless tests"}
ui.Combobox(c, &m.framework, frameworks, nil, "Framework").Clearable().
    Invalid(true).InitiallyOpen(true)`, PageSnippetZh: `frameworks := []string{"Viva Native", "WebGPU", "Headless tests"}
ui.Combobox(c, &m.framework, frameworks, nil, "框架").Clearable().
    Invalid(true).InitiallyOpen(true)`},
		{ID: "chips", Title: "多选标签", Description: "标签与查询输入按宽度换行，容器随内容增高。", Height: 240, Snippet: `// chips is the caller-owned []components.ComboboxChip, one RemoveID each.
var rects [5]gfx.Rect
components.PaintComboboxChips(canvas, theme, bounds, chips, components.ComboboxChipsOptions{
    InputID: inputID, Label: "Framework", Query: query, Placeholder: "Add a framework…",
    Scope: scope, Focused: canvas.Focused() == inputID,
}, rects[:0])
if index, ok := canvas.ClickedIn(remove); ok {
    chips = append(chips[:index], chips[index+1:]...)
}`, PageSnippet: `// The multi-select page control paints the chips, the native query and the
// filtered popup; removal is one press on a chip's remove button.
tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "Viva Native", Label: "Viva Native"},
    {Value: "WebGPU", Label: "WebGPU"},
    {Value: "Headless tests", Label: "Headless tests"},
})
tags.SetValues([]string{"Viva Native", "WebGPU"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "Framework", Placeholder: "Add a framework…",
    Options: m.frameworkOptions, Removes: m.frameworkRemoves,
}).FillWidth()`, PageSnippetZh: `// 多选页面控件绘制标签、原生查询输入和筛选弹层；点击标签的删除按钮即移除。
tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
tags.SetOptions([]multiselect.Option{
    {Value: "Viva Native", Label: "Viva Native"},
    {Value: "WebGPU", Label: "WebGPU"},
    {Value: "Headless tests", Label: "Headless tests"},
})
tags.SetValues([]string{"Viva Native", "WebGPU"})
picker := multiselect.New(tags)
picker.View(c, multiselect.Spec{
    Label: "框架", Placeholder: "添加框架…",
    Options: m.frameworkOptions, Removes: m.frameworkRemoves,
}).FillWidth()`},
	},
}

var (
	comboboxUI         = tree.Feature("combobox")
	comboboxChipOption = tree.Sub(comboboxUI, tree.Name("chip-option"))
	comboboxChipRemove = tree.Sub(comboboxUI, tree.Name("chip-remove"))
)

func initializeCombobox(state *State) {
	if state.exampleID != "chips" {
		return
	}
	names := [...]string{"Viva Native", "WebGPU", "Headless tests", "Windows", "Linux"}
	options := make([]multiselect.Option, len(names))
	for index, name := range names {
		options[index] = multiselect.Option{Value: name, Label: name}
	}
	tags := &multiselect.State{AllowCreate: true, MatchCaseInsensitive: true, Open: true}
	tags.SetOptions(options)
	tags.SetValues(names[:])
	state.multiSelect = tags
}

// comboboxChipsDemo is the page-owned multi-select picker of the chips example,
// made once so a warm paint reuses it instead of building a new control.
type comboboxChipsDemo struct {
	picker *multiselect.Picker
}

// comboboxDemo is the caller-owned value of the page-first Combobox.
type comboboxDemo struct {
	target string
}

func comboboxValueIndex(target string) int32 {
	switch target {
	case "Viva Native":
		return 1
	case "WebGPU":
		return 2
	case "Headless tests":
		return 3
	default:
		return 0
	}
}

func (state *State) paintCombobox(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "chips" {
		state.paintComboboxChips(canvas, theme, size)
		return
	}
	targets := []string{"Viva Native", "WebGPU", "Headless tests"}
	hints := []string{state.Text("原生桌面", "Native desktop"), state.Text("可选 Web 目标", "Optional web target"), state.Text("无窗口测试", "Windowless testing")}
	empty, invalid := state.exampleID == "empty", state.exampleID == "invalid"
	caption := state.Text("点击输入区域打开选项。", "Click the input to open the options.")
	if empty {
		caption = state.Text("查询 mobile 没有匹配项。", "The query mobile matches nothing.")
	}
	if invalid {
		caption = state.Text("字段被标记为无效，选项弹层仍然可用。", "The field is marked invalid; the options popup still works.")
	}
	state.drawPage(canvas, theme, size, 420, state.Text("Combobox · 框架", "Combobox · Framework"), func(c ui.C) {
		demo := ui.State[comboboxDemo](c, "combobox-demo")
		combo := ui.Combobox(c, &demo.target, targets, nil, state.Text("框架", "Framework")).Clearable()
		switch {
		case empty:
			combo = combo.Empty(state.Text("没有匹配的框架", "No matching frameworks")).InitiallyOpen(true).InitialQuery("mobile")
		case invalid:
			combo = combo.Invalid(true).InitiallyOpen(true)
		}
		combo.Describe(func(index int) ui.ComboboxOption {
			return ui.ComboboxOption{Detail: hints[index]}
		})
		state.values[0] = comboboxValueIndex(demo.target)
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}

func (state *State) paintComboboxChips(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.multiSelect == nil {
		initializeCombobox(state)
	}
	state.drawPage(canvas, theme, size, 360, state.Text("Combobox · 多选标签", "Combobox · Multi-select chips"), func(c ui.C) {
		demo := ui.State[comboboxChipsDemo](c, "combobox-chips")
		if demo.picker == nil {
			demo.picker = multiselect.New(state.multiSelect)
		}
		demo.picker.View(c, multiselect.Spec{
			Label: state.Text("框架", "Framework"), Placeholder: state.Text("添加框架…", "Add a framework…"),
			Options: comboboxChipOption, Removes: comboboxChipRemove,
		}).FillWidth()
	})
}
