package componentdemo

import (
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
control.End()`, PageSnippet: `var renderer string
ui.Select(c, &renderer, []string{"Metal", "Direct3D", "Vulkan"}, nil).
    Label("Renderer").Placeholder("Select a backend")`, PageSnippetZh: `var renderer string
ui.Select(c, &renderer, []string{"Metal", "Direct3D", "Vulkan"}, nil).
    Label("Renderer").Placeholder("Select a backend")`},
		{ID: "states", Title: "校验与禁用", Description: "并排展示无效和禁用触发器。", Snippet: `backends := []string{"Metal", "Direct3D", "Vulkan"}
var backend string
ui.Select(c, &backend, backends, nil).
    Label("Renderer").Placeholder("Select a backend").
    Invalid(backend == "")
var unavailable string
ui.Select(c, &unavailable, backends, nil).
    Label("Unavailable").Placeholder("Backend unavailable").
    Disabled(true).Small()`, PageSnippet: `backends := []string{"Metal", "Direct3D", "Vulkan"}
ui.Select(c, &m.backend, backends, nil).
    Label("Renderer").Placeholder("Select a backend").
    Invalid(m.backend == "")
ui.Select(c, &m.unavailable, backends, nil).
    Label("Unavailable").Placeholder("Backend unavailable").
    Disabled(true).Small()`, PageSnippetZh: `backends := []string{"Metal", "Direct3D", "Vulkan"}
ui.Select(c, &m.backend, backends, nil).
    Label("Renderer").Placeholder("Select a backend").
    Invalid(m.backend == "")
ui.Select(c, &m.unavailable, backends, nil).
    Label("Unavailable").Placeholder("Backend unavailable").
    Disabled(true).Small()`},
	},
}

// selectDemo is the caller-owned value of the page-first Select.
type selectDemo struct{ value string }

// selectStatesDemo is the caller-owned value of each Select in the states
// example: the backend that is still missing, and the one that is unavailable.
type selectStatesDemo struct {
	backend     string
	unavailable string
}

func (state *State) paintSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	backends := []string{"Metal", "Direct3D", "Vulkan"}
	if state.exampleID != "states" {
		state.drawPage(canvas, theme, size, 520, "Renderer", func(c ui.C) {
			demo := ui.State[selectDemo](c, "select-demo")
			ui.Select(c, &demo.value, backends, nil).
				Label("Renderer").Placeholder("Select a backend").
				FillWidth().MaxWidth(280)
			state.values[0] = selectValueIndex(demo.value)
			caption(c, state.Text("选中值与开合由应用持有。", "The application owns the selection and open state."))
		})
		return
	}

	state.drawPage(canvas, theme, size, 520, "Renderer", func(c ui.C) {
		demo := ui.State[selectStatesDemo](c, "select-states")
		ui.Select(c, &demo.backend, backends, nil).
			Label("Renderer").Placeholder("Select a backend").
			Invalid(demo.backend == "").FillWidth().MaxWidth(280)
		ui.Select(c, &demo.unavailable, backends, nil).
			Label("Unavailable").Placeholder("Backend unavailable").
			Disabled(true).Small().FillWidth().MaxWidth(280)
		state.values[0] = selectValueIndex(demo.backend)
		caption(c, state.Text("选中值与开合由应用持有。", "The application owns the selection and open state."))
	})
}

func selectValueIndex(value string) int32 {
	switch value {
	case "Metal":
		return 1
	case "Direct3D":
		return 2
	case "Vulkan":
		return 3
	default:
		return 0
	}
}
