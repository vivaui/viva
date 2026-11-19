package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var toggleComponent = Component{
	ID: "toggle", Title: "Toggle", Category: "Actions", Summary: "调用方保存激活状态的紧凑双态操作。",
	paint: (*State).paintToggle,
	Examples: []Example{
		{ID: "default", Title: "格式开关", Description: "点击切换粗体、斜体与下划线。", Snippet: `format := tree.Sub(page, tree.Name("format"))
for index, label := range [...]string{"B", "I", "U"} {
    active := flags&(1<<index) != 0
    toggle := components.BeginToggle(canvas, theme, format.At(index), bounds, active)
    if toggle.Outline(label).Clicked() {
        flags ^= 1 << index
    }
}`, PageSnippet: `ui.Row(c).Gap(12).Children(func() {
    ui.Toggle(c, &m.bold, "B").Outline()
    ui.Toggle(c, &m.italic, "I").Outline()
    ui.Toggle(c, &m.underline, "U").Outline()
})
ui.Text(c, "Each button keeps its own active bit.").Muted()`, PageSnippetZh: `ui.Row(c).Gap(12).Children(func() {
    ui.Toggle(c, &m.bold, "B").Outline()
    ui.Toggle(c, &m.italic, "I").Outline()
    ui.Toggle(c, &m.underline, "U").Outline()
})
ui.Text(c, "每个按钮独立保存激活位。").Muted()`},
		{ID: "variants", Title: "外观与尺寸", Description: "默认、描边、小号与大号变体。", Snippet: `small := components.BeginToggle(canvas, theme, tree.ID(page, "small"), bounds, active)
if small.Small().Default("Small").Clicked() {
    active = !active
}
large := components.BeginToggle(canvas, theme, tree.ID(page, "large"), bounds, true)
large.Large().Outline("Large")`, PageSnippet: `ui.Row(c).Gap(16).Items(ui.Center).Children(func() {
    ui.Toggle(c, &m.small, "Small").Small()
    ui.Toggle(c, &m.large, "Large").Large().Outline()
})
ui.Text(c, "Size comes from explicit chained options.").Muted()`, PageSnippetZh: `ui.Row(c).Gap(16).Items(ui.Center).Children(func() {
    ui.Toggle(c, &m.small, "Small").Small()
    ui.Toggle(c, &m.large, "Large").Large().Outline()
})
ui.Text(c, "尺寸由显式链式选项决定。").Muted()`},
		{ID: "states", Title: "校验与禁用", Description: "激活、错误和禁用状态。", Snippet: `invalid, disabled := true, false
ui.Row(c).Gap(16).Children(func() {
    ui.Toggle(c, &invalid, "Invalid").Outline().Invalid(true)
    ui.Toggle(c, &disabled, "Disabled").Outline().Disabled(true)
})`, PageSnippet: `m.invalid = true
ui.Row(c).Gap(16).Children(func() {
    ui.Toggle(c, &m.invalid, "Invalid").Outline().Invalid(true)
    ui.Toggle(c, &m.disabled, "Disabled").Outline().Disabled(true)
})`, PageSnippetZh: `m.invalid = true
ui.Row(c).Gap(16).Children(func() {
    ui.Toggle(c, &m.invalid, "Invalid").Outline().Invalid(true)
    ui.Toggle(c, &m.disabled, "Disabled").Outline().Disabled(true)
})`},
	},
}

var (
	toggleUI = tree.Feature("toggle")
	// toggleItem's item i is the i-th toggle whose state the caller keeps (bit i of flags).
	toggleItem  = tree.Sub(toggleUI, tree.Name("item"))
	toggleLarge = tree.ID(toggleUI, "large")
)

// togglePageState is the page Toggle's model: the active bits of the toggles a
// caller keeps (the fixed specimens are seeded each paint).
type togglePageState struct {
	values [4]bool
}

func (state *State) paintToggle(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "states" {
		state.paintToggleStates(canvas, theme, size)
		return
	}
	state.drawPage(canvas, theme, size, 500, state.Text("Toggle · 双态操作", "Toggle · Two-state action"), func(c ui.C) {
		page := ui.State[togglePageState](c, "toggles")
		if state.exampleID == "variants" {
			ui.Row(c).Gap(16).Items(ui.Center).Children(func() {
				page.values[3] = state.flags&(1<<3) != 0
				if ui.Toggle(c, &page.values[3], "Small").Small().Hit(toggleItem.At(3), "Small").Changed() {
					if page.values[3] {
						state.flags |= 1 << 3
					} else {
						state.flags &^= 1 << 3
					}
				}
				page.values[1] = true
				ui.Toggle(c, &page.values[1], "Large").Large().Outline().Hit(toggleLarge, "Large")
			})
			caption(c, state.Text("尺寸由显式链式选项决定。", "Size comes from explicit chained options."))
			return
		}
		ui.Row(c).Gap(12).Children(func() {
			for index, label := range [...]string{"B", "I", "U"} {
				page.values[index] = state.flags&(1<<index) != 0
				if ui.Toggle(c, &page.values[index], label).Outline().Hit(toggleItem.At(index), label).Changed() {
					if page.values[index] {
						state.flags |= 1 << index
					} else {
						state.flags &^= 1 << index
					}
				}
			}
		})
		ui.Text(c, state.Text("每个按钮独立保存激活位。", "Each button keeps its own active bit.")).Muted()
	})
}

// paintToggleStates declares the validation example from the page API: an active
// invalid toggle and a disabled one, each with its own caller-owned value.
func (state *State) paintToggleStates(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	state.drawPage(canvas, theme, size, 500, state.Text("Toggle · 双态操作", "Toggle · Two-state action"), func(c ui.C) {
		page := ui.State[togglePageState](c, "toggles")
		page.values[0], page.values[1] = true, false
		ui.Row(c).Gap(16).Children(func() {
			ui.Toggle(c, &page.values[0], "Invalid").Outline().Invalid(true)
			ui.Toggle(c, &page.values[1], "Disabled").Outline().Disabled(true)
		})
		caption(c, state.Text("错误与禁用状态由链式选项声明。", "The invalid and disabled states are chained options."))
	})
}
