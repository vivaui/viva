package componentdemo

import (
	"github.com/vivaui/viva/components"
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
}`},
		{ID: "variants", Title: "外观与尺寸", Description: "默认、描边、小号与大号变体。", Snippet: `small := components.BeginToggle(canvas, theme, tree.ID(page, "small"), bounds, active)
if small.Small().Default("Small").Clicked() {
    active = !active
}
large := components.BeginToggle(canvas, theme, tree.ID(page, "large"), bounds, true)
large.Large().Outline("Large")`},
		{ID: "states", Title: "校验与禁用", Description: "激活、错误和禁用状态。", Snippet: `invalid := tree.ID(page, "invalid")
components.BeginToggle(canvas, theme, invalid, bounds, true).Invalid().Outline("Invalid")
disabled := tree.ID(page, "disabled")
components.BeginToggle(canvas, theme, disabled, bounds, false).Disabled().Outline("Disabled")`},
	},
}

var (
	toggleUI = tree.Feature("toggle")
	// toggleItem's item i is the i-th toggle whose state the caller keeps (bit i of flags).
	toggleItem     = tree.Sub(toggleUI, tree.Name("item"))
	toggleLarge    = tree.ID(toggleUI, "large")
	toggleInvalid  = tree.ID(toggleUI, "invalid")
	toggleDisabled = tree.ID(toggleUI, "disabled")
)

// toggleSpecimen is one toggle: the slot ui gave it and how the component paints it.
type toggleSpecimen struct {
	slot                          ui.Box
	id                            gfx.ID
	label                         string
	active, small, large, outline bool
	invalid, disabled             bool
}

func (state *State) paintToggle(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var specimens [6]toggleSpecimen
	count := 0
	height := theme.Metrics.ControlHeight // default size; small and large adjust below
	add := func(c ui.C, specimen toggleSpecimen, width, extra float32) {
		specimen.slot = ui.Slot(c).Size(width, height+extra)
		specimens[count] = specimen
		count++
	}
	state.drawPage(canvas, theme, size, 500, state.Text("Toggle · 双态操作", "Toggle · Two-state action"), func(c ui.C) {
		ui.Row(c).Gap(12).Children(func() {
			for index, label := range [...]string{"B", "I", "U"} {
				add(c, toggleSpecimen{id: toggleItem.At(index), label: label, outline: true, active: state.flags&(1<<index) != 0}, 44, 0)
			}
		})
		switch state.exampleID {
		case "variants":
			ui.Row(c).Gap(16).Items(ui.Center).Children(func() {
				add(c, toggleSpecimen{id: toggleItem.At(3), label: "Small", small: true, active: state.flags&(1<<3) != 0}, 80, -4)
				add(c, toggleSpecimen{id: toggleLarge, label: "Large", large: true, outline: true, active: true}, 92, 4)
			})
			caption(c, state.Text("尺寸由显式链式选项决定。", "Size comes from explicit chained options."))
		case "states":
			ui.Row(c).Gap(16).Children(func() {
				add(c, toggleSpecimen{id: toggleInvalid, label: "Invalid", outline: true, active: true, invalid: true}, 96, 0)
				add(c, toggleSpecimen{id: toggleDisabled, label: "Disabled", outline: true, disabled: true}, 96, 0)
			})
		default:
			caption(c, state.Text("每个按钮独立保存激活位。", "Each button keeps its own active bit."))
		}
	})

	for _, specimen := range specimens[:count] {
		toggle := components.BeginToggle(canvas, theme, specimen.id, specimen.slot.Rect(), specimen.active)
		if specimen.small {
			toggle = toggle.Small()
		}
		if specimen.large {
			toggle = toggle.Large()
		}
		if specimen.invalid {
			toggle = toggle.Invalid()
		}
		if specimen.disabled {
			toggle = toggle.Disabled()
		}
		if specimen.outline {
			toggle.Outline(specimen.label)
		} else {
			toggle.Default(specimen.label)
		}
	}
	if index, ok := canvas.ClickedIn(toggleItem); ok {
		state.flags ^= 1 << index
	}
}
