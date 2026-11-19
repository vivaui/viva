package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/buttongroupview"
)

var buttonGroupComponent = Component{
	ID: "button-group", Title: "Button Group", Category: "Actions", Summary: "共享边框的水平或垂直相关操作。",
	paint: (*State).paintButtonGroup, initialize: (*State).initializeButtonGroup,
	Examples: []Example{
		{ID: "horizontal", Title: "文本对齐", Description: "三个相关动作组成一个水平控件。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options)
if id, ok := group.Paint().Clicked(); ok {
    align = id
}`, Height: 170, PageSnippet: `// m.buttonGroup is a buttongroupview.Group; m.align is the chosen index.
group := m.buttonGroup
group.Reset().
    Item(ui.ID(c), "Left").Outline().
    Item(ui.ID(c), "Center").Outline().
    Item(ui.ID(c), "Right").Outline().
    Selected(m.align).
    OnClick(m.selectAlignment)
group.View(c, "Text alignment").FillWidth()
ui.Textf(c, "Current: %s", m.alignName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `// m.buttonGroup 是 buttongroupview.Group；m.align 是选中项下标。
group := m.buttonGroup
group.Reset().
    Item(ui.ID(c), "左对齐").Outline().
    Item(ui.ID(c), "居中").Outline().
    Item(ui.ID(c), "右对齐").Outline().
    Selected(m.align).
    OnClick(m.selectAlignment)
group.View(c, "文本对齐").FillWidth()
ui.Textf(c, "当前：%s", m.alignName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "vertical", Title: "垂直操作", Description: "窄区域中使用相同选项垂直排列。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options).Vertical()
if id, ok := group.Paint().Clicked(); ok {
    align = id
}`, Height: 230, PageSnippet: `// m.buttonGroup is a buttongroupview.Group; m.align is the chosen index.
group := m.buttonGroup
group.Reset().
    Vertical().
    Item(ui.ID(c), "Left").Outline().
    Item(ui.ID(c), "Center").Outline().
    Item(ui.ID(c), "Right").Outline().
    Selected(m.align).
    OnClick(m.selectAlignment)
group.View(c, "Text alignment").Width(160)
ui.Textf(c, "Current: %s", m.alignName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `// m.buttonGroup 是 buttongroupview.Group；m.align 是选中项下标。
group := m.buttonGroup
group.Reset().
    Vertical().
    Item(ui.ID(c), "左对齐").Outline().
    Item(ui.ID(c), "居中").Outline().
    Item(ui.ID(c), "右对齐").Outline().
    Selected(m.align).
    OnClick(m.selectAlignment)
group.View(c, "文本对齐").Width(160)
ui.Textf(c, "当前：%s", m.alignName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "disabled", Title: "禁用组合", Description: "整体禁用仍保留每个动作的语义。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
components.BeginButtonGroup(canvas, theme, bounds, options).Disabled().Paint()`, Height: 170, PageSnippet: `// m.buttonGroup is a buttongroupview.Group.
group := m.buttonGroup
group.Reset().
    Disabled().
    Item(ui.ID(c), "Left").Outline().
    Item(ui.ID(c), "Center").Outline().
    Item(ui.ID(c), "Right").Outline()
group.View(c, "Text alignment").FillWidth()
ui.Text(c, "The whole group is disabled.").
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `// m.buttonGroup 是 buttongroupview.Group。
group := m.buttonGroup
group.Reset().
    Disabled().
    Item(ui.ID(c), "左对齐").Outline().
    Item(ui.ID(c), "居中").Outline().
    Item(ui.ID(c), "右对齐").Outline()
group.View(c, "文本对齐").FillWidth()
ui.Text(c, "整个操作组已禁用。").
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "variants", Title: "混合变体", Description: "描边、危险与链接动作沿用各自 Button 变体的配色。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "save"), Label: "Save", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "delete"), Label: "Delete", Variant: components.ButtonDestructiveOutline},
    {ID: tree.ID(page, "help"), Label: "Help", Variant: components.ButtonLink},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options)
if id, ok := group.Paint().Clicked(); ok {
    action = id
}`, Height: 170, PageSnippet: `// m.buttonGroup is a buttongroupview.Group; m.selectAction reports the click.
group := m.buttonGroup
group.Reset().
    Item(ui.ID(c), "Save").Outline().
    Item(ui.ID(c), "Delete").DestructiveOutline().
    Item(ui.ID(c), "Help").Link().
    OnClick(m.selectAction)
group.View(c, "Actions").FillWidth()
ui.Textf(c, "Current: %s", m.actionName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `// m.buttonGroup 是 buttongroupview.Group；m.selectAction 处理点击。
group := m.buttonGroup
group.Reset().
    Item(ui.ID(c), "保存").Outline().
    Item(ui.ID(c), "删除").DestructiveOutline().
    Item(ui.ID(c), "帮助").Link().
    OnClick(m.selectAction)
group.View(c, "操作").FillWidth()
ui.Textf(c, "当前：%s", m.actionName).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
	},
}

var (
	buttonGroupUI = tree.Feature("button-group")
	// buttonGroupItem's item i is the i-th action of the group.
	buttonGroupItem = tree.Sub(buttonGroupUI, tree.Name("item"))
)

func (state *State) initializeButtonGroup() {
	state.buttonGroup = buttongroupview.New()
	state.buttonGroup.OnClick(state.selectButtonGroup)
}

func (state *State) selectButtonGroup(index int, _ gfx.ID) {
	state.values[0] = int32(index)
}

func (state *State) paintButtonGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	vertical := state.exampleID == "vertical"
	disabled := state.exampleID == "disabled"
	variants := state.exampleID == "variants"
	labels := [3]string{
		state.Text("左对齐", "Left"),
		state.Text("居中", "Center"),
		state.Text("右对齐", "Right"),
	}
	captions := [3]string{
		state.Text("当前：左对齐", "Current: Left"),
		state.Text("当前：居中", "Current: Center"),
		state.Text("当前：右对齐", "Current: Right"),
	}
	if variants {
		labels = [3]string{state.Text("保存", "Save"), state.Text("删除", "Delete"), state.Text("帮助", "Help")}
		captions = [3]string{
			state.Text("当前：保存", "Current: Save"),
			state.Text("当前：删除", "Current: Delete"),
			state.Text("当前：帮助", "Current: Help"),
		}
	}
	caption := captions[state.values[0]]
	if disabled {
		caption = state.Text("整个操作组已禁用。", "The whole group is disabled.")
	}
	state.drawPage(canvas, theme, size, 360, state.Text("Button Group · 对齐方式", "Button Group · Alignment"), func(c ui.C) {
		group := state.buttonGroup
		if group == nil {
			state.initializeButtonGroup()
			group = state.buttonGroup
		}
		group.Reset()
		if vertical {
			group.Vertical()
		}
		if disabled {
			group.Disabled()
		}
		for index := range labels {
			group.Item(buttonGroupItem.At(index), labels[index])
			switch {
			case variants && index == 1:
				group.DestructiveOutline()
			case variants && index == 2:
				group.Link()
			default:
				group.Outline()
			}
		}
		if !variants && !disabled {
			group.Selected(int(state.values[0]))
		}
		view := group.View(c, state.Text("文本对齐", "Text alignment")).FillWidth()
		if vertical {
			view.Width(180)
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
