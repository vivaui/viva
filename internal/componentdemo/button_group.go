package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var buttonGroupComponent = Component{
	ID: "button-group", Title: "Button Group", Category: "Actions", Summary: "共享边框的水平或垂直相关操作。",
	paint: (*State).paintButtonGroup,
	Examples: []Example{
		{ID: "horizontal", Title: "文本对齐", Description: "三个相关动作组成一个水平控件。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options)
if id, ok := group.Paint().Clicked(); ok {
    align = id
}`, Height: 170},
		{ID: "vertical", Title: "垂直操作", Description: "窄区域中使用相同选项垂直排列。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options).Vertical()
if id, ok := group.Paint().Clicked(); ok {
    align = id
}`, Height: 230},
		{ID: "disabled", Title: "禁用组合", Description: "整体禁用仍保留每个动作的语义。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
components.BeginButtonGroup(canvas, theme, bounds, options).Disabled().Paint()`, Height: 170},
		{ID: "variants", Title: "混合变体", Description: "描边、危险与链接动作沿用各自 Button 变体的配色。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "save"), Label: "Save", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "delete"), Label: "Delete", Variant: components.ButtonDestructiveOutline},
    {ID: tree.ID(page, "help"), Label: "Help", Variant: components.ButtonLink},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options)
if id, ok := group.Paint().Clicked(); ok {
    action = id
}`, Height: 170},
	},
}

var (
	buttonGroupUI = tree.Feature("button-group")
	// buttonGroupItem's item i is the i-th action of the group.
	buttonGroupItem = tree.Sub(buttonGroupUI, tree.Name("item"))
)

func (state *State) paintButtonGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	vertical := state.exampleID == "vertical"
	disabled := state.exampleID == "disabled"
	var slot ui.Box
	state.drawPage(canvas, theme, size, 360, state.Text("Button Group · 对齐方式", "Button Group · Alignment"), func(c ui.C) {
		if vertical {
			slot = ui.Slot(c).Size(220, theme.Metrics.ControlHeight*3).Self(ui.Start)
		} else {
			slot = ui.Slot(c).FillWidth().MaxWidth(360).Height(theme.Metrics.ControlHeight).Self(ui.Start)
		}
		labels := [...]string{
			state.Text("当前：左对齐", "Current: Left"),
			state.Text("当前：居中", "Current: Center"),
			state.Text("当前：右对齐", "Current: Right"),
		}
		caption := labels[state.values[0]]
		if disabled {
			caption = state.Text("整个操作组已禁用。", "The whole group is disabled.")
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	var options [3]components.ButtonGroupOption
	if state.exampleID == "variants" {
		options = [3]components.ButtonGroupOption{
			{ID: buttonGroupItem.At(0), Label: state.Text("保存", "Save"), Variant: components.ButtonOutline},
			{ID: buttonGroupItem.At(1), Label: state.Text("删除", "Delete"), Variant: components.ButtonDestructiveOutline},
			{ID: buttonGroupItem.At(2), Label: state.Text("帮助", "Help"), Variant: components.ButtonLink},
		}
	} else {
		options = [3]components.ButtonGroupOption{
			{ID: buttonGroupItem.At(0), Label: state.Text("左对齐", "Left"), Variant: components.ButtonOutline},
			{ID: buttonGroupItem.At(1), Label: state.Text("居中", "Center"), Variant: components.ButtonOutline},
			{ID: buttonGroupItem.At(2), Label: state.Text("右对齐", "Right"), Variant: components.ButtonOutline},
		}
	}
	group := components.BeginButtonGroup(canvas, theme, slot.Rect(), options[:])
	if vertical {
		group = group.Vertical()
	} else if disabled {
		group = group.Disabled()
	}
	if id, ok := group.Paint().Clicked(); ok {
		state.values[0] = int32(listIndex(id, buttonGroupItem))
	}
}
