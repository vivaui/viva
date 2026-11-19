package componentdemo

import (
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
}`, Height: 170, PageSnippet: `ui.Row(c).Children(func() {
    if ui.Button(c, "Left").Outline().Clicked() {
        m.align = "Left"
    }
    if ui.Button(c, "Center").Outline().Clicked() {
        m.align = "Center"
    }
    if ui.Button(c, "Right").Outline().Clicked() {
        m.align = "Right"
    }
})
ui.Textf(c, "Current: %s", m.align).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Row(c).Children(func() {
    if ui.Button(c, "左对齐").Outline().Clicked() {
        m.align = "左对齐"
    }
    if ui.Button(c, "居中").Outline().Clicked() {
        m.align = "居中"
    }
    if ui.Button(c, "右对齐").Outline().Clicked() {
        m.align = "右对齐"
    }
})
ui.Textf(c, "当前：%s", m.align).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "vertical", Title: "垂直操作", Description: "窄区域中使用相同选项垂直排列。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
group := components.BeginButtonGroup(canvas, theme, bounds, options).Vertical()
if id, ok := group.Paint().Clicked(); ok {
    align = id
}`, Height: 230, PageSnippet: `ui.Column(c).Children(func() {
    if ui.Button(c, "Left").Outline().Clicked() {
        m.align = "Left"
    }
    if ui.Button(c, "Center").Outline().Clicked() {
        m.align = "Center"
    }
    if ui.Button(c, "Right").Outline().Clicked() {
        m.align = "Right"
    }
})
ui.Textf(c, "Current: %s", m.align).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Column(c).Children(func() {
    if ui.Button(c, "左对齐").Outline().Clicked() {
        m.align = "左对齐"
    }
    if ui.Button(c, "居中").Outline().Clicked() {
        m.align = "居中"
    }
    if ui.Button(c, "右对齐").Outline().Clicked() {
        m.align = "右对齐"
    }
})
ui.Textf(c, "当前：%s", m.align).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
		{ID: "disabled", Title: "禁用组合", Description: "整体禁用仍保留每个动作的语义。", Snippet: `options := []components.ButtonGroupOption{
    {ID: tree.ID(page, "left"), Label: "Left", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "center"), Label: "Center", Variant: components.ButtonOutline},
    {ID: tree.ID(page, "right"), Label: "Right", Variant: components.ButtonOutline},
}
components.BeginButtonGroup(canvas, theme, bounds, options).Disabled().Paint()`, Height: 170, PageSnippet: `ui.Row(c).Children(func() {
    ui.Button(c, "Left").Outline().Disabled(true)
    ui.Button(c, "Center").Outline().Disabled(true)
    ui.Button(c, "Right").Outline().Disabled(true)
})
ui.Text(c, "The whole group is disabled.").
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Row(c).Children(func() {
    ui.Button(c, "左对齐").Outline().Disabled(true)
    ui.Button(c, "居中").Outline().Disabled(true)
    ui.Button(c, "右对齐").Outline().Disabled(true)
})
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
}`, Height: 170, PageSnippet: `ui.Row(c).Children(func() {
    if ui.Button(c, "Save").Outline().Clicked() {
        m.action = "Save"
    }
    if ui.Button(c, "Delete").DestructiveOutline().Clicked() {
        m.action = "Delete"
    }
    if ui.Button(c, "Help").Link().Clicked() {
        m.action = "Help"
    }
})
ui.Textf(c, "Current: %s", m.action).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`, PageSnippetZh: `ui.Row(c).Children(func() {
    if ui.Button(c, "保存").Outline().Clicked() {
        m.action = "保存"
    }
    if ui.Button(c, "删除").DestructiveOutline().Clicked() {
        m.action = "删除"
    }
    if ui.Button(c, "帮助").Link().Clicked() {
        m.action = "帮助"
    }
})
ui.Textf(c, "当前：%s", m.action).
    FontSize(c.Theme().Metrics.FontSizeS).Muted()`},
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
		if vertical {
			ui.Column(c).Children(func() {
				for index := range labels {
					button := ui.Button(c, labels[index]).Outline().Hit(buttonGroupItem.At(index), labels[index])
					if button.Clicked() {
						state.values[0] = int32(index)
					}
				}
			})
		} else {
			ui.Row(c).Children(func() {
				for index := range labels {
					var button ui.Box
					switch {
					case disabled:
						button = ui.Button(c, labels[index]).Outline().Disabled(true).Hit(buttonGroupItem.At(index), labels[index])
					case variants && index == 1:
						button = ui.Button(c, labels[index]).DestructiveOutline().Hit(buttonGroupItem.At(index), labels[index])
					case variants && index == 2:
						button = ui.Button(c, labels[index]).Link().Hit(buttonGroupItem.At(index), labels[index])
					default:
						button = ui.Button(c, labels[index]).Outline().Hit(buttonGroupItem.At(index), labels[index])
					}
					if button.Clicked() {
						state.values[0] = int32(index)
					}
				}
			})
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}
