package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var comboboxComponent = Component{
	ID: "combobox", Title: "Combobox", Category: "Forms", Summary: "外部持有查询、展开和选择状态的可编辑选择器。",
	paint: (*State).paintCombobox, initialize: initializeCombobox,
	Examples: []Example{
		{ID: "default", Title: "选择框架", Description: "输入区域、弹层和选项均使用生产 Combobox。", Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
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
combo.End()`},
		{ID: "empty", Title: "空结果", Description: "查询无匹配项时显示生产空状态。", Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
    InputID: inputID, Label: "Framework", Value: values[selected], Query: "mobile",
    Open: open, Content: content, Scope: scope, DismissID: dismissID, Layer: 6,
})
if open {
    combo.AddEmpty("No matching frameworks")
}
combo.End()`},
		{ID: "invalid", Title: "无效状态", Description: "保留选项弹层并突出输入边框。", Snippet: `combo := components.BeginCombobox(canvas, theme, inputBounds, components.ComboboxOptions{
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
combo.End()`},
		{ID: "chips", Title: "多选标签", Description: "标签与查询输入按宽度换行，容器随内容增高。", Snippet: `// chips is the caller-owned []components.ComboboxChip, one RemoveID each.
var rects [5]gfx.Rect
components.PaintComboboxChips(canvas, theme, bounds, chips, components.ComboboxChipsOptions{
    InputID: inputID, Label: "Framework", Query: query, Placeholder: "Add a framework…",
    Scope: scope, Focused: canvas.Focused() == inputID,
}, rects[:0])
if index, ok := canvas.ClickedIn(remove); ok {
    chips = append(chips[:index], chips[index+1:]...)
}`},
	},
}

var (
	comboboxUI      = tree.Feature("combobox")
	comboboxInput   = tree.ID(comboboxUI, "input")
	comboboxClear   = tree.ID(comboboxUI, "clear")
	comboboxDismiss = tree.ID(comboboxUI, "dismiss")
	comboboxScope   = tree.ID(comboboxUI, "scope")
	// comboboxOption's item i is the i-th option of the list.
	comboboxOption = tree.Sub(comboboxUI, tree.Name("option"))
)

const comboboxOpenFlag uint64 = 1

func initializeCombobox(state *State) { state.flags |= comboboxOpenFlag }

// comboboxChipRemove's item i removes the i-th chip of the chips example.
var comboboxChipRemove = tree.Sub(comboboxUI, tree.Name("chip-remove"))

func (state *State) paintComboboxChips(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var field ui.Box
	state.drawPage(canvas, theme, size, 360, state.Text("Combobox · 多选标签", "Combobox · Multi-select chips"), func(c ui.C) {
		field = ui.Slot(c).FillWidth().Height(120)
	})
	names := [...]string{"Viva Native", "WebGPU", "Headless tests", "Windows", "Linux"}
	// Bit i of the flags above the open bit says chip i has been removed.
	var chips [len(names)]components.ComboboxChip
	count := 0
	for index, name := range names {
		if state.flags&(2<<index) != 0 {
			continue
		}
		chips[count] = components.ComboboxChip{RemoveID: comboboxChipRemove.At(index), Label: name}
		count++
	}
	var rectangles [len(names)]gfx.Rect
	components.PaintComboboxChips(canvas, theme, field.Rect(), chips[:count], components.ComboboxChipsOptions{
		InputID: comboboxInput, Label: state.Text("框架", "Framework"), Placeholder: state.Text("添加框架…", "Add a framework…"),
		Scope:   comboboxScope,
		Focused: canvas.Focused() == comboboxInput,
	}, rectangles[:0])
	if index, ok := canvas.ClickedIn(comboboxChipRemove); ok {
		state.flags |= 2 << index
	}
}

func (state *State) paintCombobox(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "chips" {
		state.paintComboboxChips(canvas, theme, size)
		return
	}
	values := [...]string{"", "Viva Native", "WebGPU", "Headless tests"}
	selected := int(state.values[0])
	query := ""
	if state.exampleID == "empty" {
		query = "mobile"
	}
	open := state.flags&comboboxOpenFlag != 0
	var field ui.Box
	state.drawPage(canvas, theme, size, 420, state.Text("Combobox · 框架", "Combobox · Framework"), func(c ui.C) {
		field = ui.Slot(c).FillWidth().MaxWidth(280).Height(theme.Metrics.ControlHeight)
		if !open {
			ui.Text(c, state.Text("点击输入区域重新打开选项。", "Click the input to reopen the options.")).FontSize(theme.Metrics.FontSizeS).Muted()
		}
	})
	// The list is a popup under the input, at least 300 wide and centered on it.
	input := field.Rect()
	contentWidth := max(input.Width, float32(300))
	contentHeight := float32(28 + 3*32 + 9 + 2*4) // group label, three rows, separator, padding
	if state.exampleID == "empty" {
		contentHeight = 36 + 2*4 // one empty-result row and padding
	}
	combo := components.BeginCombobox(canvas, theme, input, components.ComboboxOptions{
		InputID: comboboxInput, ClearID: comboboxClear, Label: state.Text("框架", "Framework"), Value: values[selected], Query: query,
		Placeholder: state.Text("选择框架…", "Select a framework…"), Hint: state.Text("输入以筛选", "Type to filter"),
		Open: open, Invalid: state.exampleID == "invalid",
		ShowTrigger: true, ShowClear: selected > 0,
		Content: gfx.Rect{X: input.X + (input.Width-contentWidth)/2, Y: input.Y + input.Height + 8, Width: contentWidth, Height: contentHeight},
		Layer:   6, Scope: comboboxScope, DismissID: comboboxDismiss,
	})
	if canvas.Clicked(comboboxInput) {
		state.flags ^= comboboxOpenFlag
	} else if canvas.Clicked(comboboxClear) {
		state.values[0] = 0
	} else if canvas.Clicked(comboboxDismiss) {
		state.flags &^= comboboxOpenFlag
	} else if index, ok := canvas.ClickedIn(comboboxOption); ok {
		state.values[0] = int32(index) + 1
		state.flags &^= comboboxOpenFlag
	}
	defer combo.End()
	if !open {
		return
	}
	if state.exampleID == "empty" {
		combo.AddEmpty(state.Text("没有匹配的框架", "No matching frameworks"))
		return
	}
	// Highlighted is the row the pointer or the arrow keys are on (Radix's
	// data-highlighted): it is the row Enter chooses. A selected row keeps only
	// its check, so the two never tint two rows at once.
	highlighted := selected - 1
	for i := 0; i < 3; i++ {
		if canvas.Hovered() == comboboxOption.At(i) {
			highlighted = i
		}
	}
	combo.AddLabel(state.Text("可用目标", "Available targets"))
	combo.AddItem(components.ComboboxItem{ID: comboboxOption.At(0), Label: "Viva Native", Value: "native", Hint: state.Text("原生桌面", "Native desktop"), Selected: selected == 1, Highlighted: highlighted == 0})
	combo.AddItem(components.ComboboxItem{ID: comboboxOption.At(1), Label: "WebGPU", Value: "webgpu", Hint: state.Text("可选 Web 目标", "Optional web target"), Selected: selected == 2, Highlighted: highlighted == 1})
	combo.AddSeparator()
	combo.AddItem(components.ComboboxItem{ID: comboboxOption.At(2), Label: "Headless tests", Value: "headless", Hint: state.Text("无窗口测试", "Windowless testing"), Selected: selected == 3, Highlighted: highlighted == 2})
}
