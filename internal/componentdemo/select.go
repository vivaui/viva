package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var selectComponent = Component{
	ID: "select", Title: "Select", Category: "Forms", Summary: "应用持有选中值和开合状态的原生选择菜单。",
	paint: (*State).paintSelect,
	Examples: []Example{
		{ID: "default", Title: "选择后端", Description: "打开菜单并选择一个桌面渲染后端。", Snippet: `item := tree.Sub(page, tree.Name("item"))
values := []string{"metal", "d3d", "vulkan"}
control := components.BeginSelect(canvas, theme, bounds, components.SelectOptions{
    TriggerID: tree.ID(page, "trigger"), Label: "Renderer", Value: value,
    Placeholder: "Select a backend", Open: open, Content: content, Layer: 4,
    Scope: tree.ID(page, "content"), DismissID: tree.ID(page, "dismiss"),
})
control.AddLabel("Native backends")
for index, label := range []string{"Metal", "Direct3D", "Vulkan"} {
    control.AddItem(components.SelectItem{
        ID: item.At(index), Label: label, Value: values[index],
        Selected: value == values[index],
    })
}
control.End()`},
		{ID: "states", Title: "校验与禁用", Description: "并排展示无效和禁用触发器。", Snippet: `components.BeginSelect(canvas, theme, invalidBounds, components.SelectOptions{
    TriggerID: tree.ID(page, "trigger"), Label: "Renderer",
    Value: "", Placeholder: "Select a backend", Invalid: true,
})
components.BeginSelect(canvas, theme, disabledBounds, components.SelectOptions{
    TriggerID: tree.ID(page, "disabled"), Label: "Unavailable",
    Placeholder: "Backend unavailable", Disabled: true,
    Size: components.SelectSizeSmall,
})`},
	},
}

var (
	selectUI       = tree.Feature("select")
	selectTrigger  = tree.ID(selectUI, "trigger")
	selectDismiss  = tree.ID(selectUI, "dismiss")
	selectScope    = tree.ID(selectUI, "content")
	selectDisabled = tree.ID(selectUI, "disabled")
	// selectItem's item i is the i-th backend.
	selectItem = tree.Sub(selectUI, tree.Name("item"))
)

func (state *State) paintSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var trigger, unavailable ui.Box
	open := state.flags&1 != 0
	state.drawPage(canvas, theme, size, 520, "Renderer", func(c ui.C) {
		trigger = ui.Slot(c).FillWidth().MaxWidth(280).Height(theme.Metrics.ControlHeight)
		if state.exampleID == "states" {
			unavailable = ui.Slot(c).FillWidth().MaxWidth(280).Height(theme.Metrics.ControlHeight)
		}
		caption(c, state.Text("选中值与开合由应用持有。", "The application owns the selection and open state."))
	})

	bounds := trigger.Rect()
	// One 28px group label, three 32px rows, and the popup's 4px top and bottom
	// padding. A taller box would leave an empty strip below the last option.
	contentHeight := float32(28 + 3*32 + 2*4)
	contentY := bounds.Y + bounds.Height + 6
	if contentY+contentHeight > size.Height-8 {
		contentY = max(8, bounds.Y-contentHeight-6)
	}
	selectControl := components.BeginSelect(canvas, theme, bounds, components.SelectOptions{
		TriggerID: selectTrigger, Label: "Renderer", Value: selectValueLabel(state.values[0]), Placeholder: "Select a backend",
		Open: open, Invalid: state.exampleID == "states" && state.values[0] == 0,
		Content: gfx.Rect{X: bounds.X, Y: contentY, Width: bounds.Width, Height: contentHeight},
		Layer:   4, Scope: selectScope, DismissID: selectDismiss,
	})
	selectControl.AddLabel("Native backends")
	selectControl.AddItem(components.SelectItem{ID: selectItem.At(0), Label: "Metal", Value: "metal", Selected: state.values[0] == 1})
	selectControl.AddItem(components.SelectItem{ID: selectItem.At(1), Label: "Direct3D", Value: "d3d", Selected: state.values[0] == 2})
	selectControl.AddItem(components.SelectItem{ID: selectItem.At(2), Label: "Vulkan", Value: "vulkan", Selected: state.values[0] == 3})
	selectControl.End()

	if state.exampleID == "states" && !open {
		components.BeginSelect(canvas, theme, unavailable.Rect(), components.SelectOptions{
			TriggerID: selectDisabled, Label: "Unavailable", Placeholder: "Backend unavailable", Disabled: true, Size: components.SelectSizeSmall,
		}).TriggerBounds()
	}

	switch index, picked := canvas.ClickedIn(selectItem); {
	case picked:
		state.values[0] = int32(index) + 1
		state.flags &^= 1
	case canvas.Clicked(selectTrigger):
		state.flags ^= 1
	case canvas.Clicked(selectDismiss):
		state.flags &^= 1
	}
}

func selectValueLabel(value int32) string {
	switch value {
	case 1:
		return "Metal"
	case 2:
		return "Direct3D"
	case 3:
		return "Vulkan"
	default:
		return ""
	}
}
