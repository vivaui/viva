package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var nativeSelectComponent = Component{
	ID: "native-select", Title: "Native Select", Category: "Forms", Summary: "由操作系统拥有弹窗生命周期的选择触发器。",
	paint: (*State).paintNativeSelect,
	Examples: []Example{
		{ID: "default", Title: "系统选择器", Description: "点击触发器切换受控 open 快照。", Snippet: `components.PaintNativeSelect(canvas, theme, bounds, components.NativeSelectSnapshot{
    ID: tree.ID(page, "environment"), Label: "Environment",
    Value: "production", Display: "Production", Hint: "Choose environment",
    Open: open,
})
if canvas.Clicked(tree.ID(page, "environment")) {
    open = !open
}`, PageSnippet: `environment := "Production"
ui.Select(c, &environment, []string{"Development", "Staging", "Production"}, nil).
    Label("Environment").Hint("Choose environment")`, PageSnippetZh: `environment := "Production"
ui.Select(c, &environment, []string{"Development", "Staging", "Production"}, nil).
    Label("Environment").Hint("Choose environment")`},
		{ID: "states", Title: "选择器状态", Description: "紧凑、无效与禁用触发器。", Snippet: `components.PaintNativeSelect(canvas, theme, smallBounds, components.NativeSelectSnapshot{
    ID: tree.ID(page, "region"), Label: "Region", Placeholder: "Select a region",
    Size: components.NativeSelectSmall,
})
components.PaintNativeSelect(canvas, theme, invalidBounds, components.NativeSelectSnapshot{
    ID: tree.ID(page, "team"), Label: "Team", Value: "none",
    Display: "No team", Invalid: true,
})
components.PaintNativeSelect(canvas, theme, disabledBounds, components.NativeSelectSnapshot{
    ID: tree.ID(page, "plan"), Label: "Plan", Value: "enterprise",
    Display: "Enterprise", Disabled: true,
})`, PageSnippet: `regions := []string{"us-east", "eu-west", "ap-south"}
teams := []string{"No team"}
plans := []string{"Enterprise"}
ui.Select(c, &m.region, regions, nil).Placeholder("Select a region").Small()
ui.FormField(c, &m.teamField, "Team").Children(func() {
    ui.Select(c, &m.team, teams, nil).Placeholder("No team")
})
ui.Select(c, &m.plan, plans, nil).Disabled(true)`, PageSnippetZh: `regions := []string{"us-east", "eu-west", "ap-south"}
teams := []string{"No team"}
plans := []string{"Enterprise"}
ui.Select(c, &m.region, regions, nil).Placeholder("Select a region").Small()
ui.FormField(c, &m.teamField, "Team").Children(func() {
    ui.Select(c, &m.team, teams, nil).Placeholder("No team")
})
ui.Select(c, &m.plan, plans, nil).Disabled(true)`},
	},
}

var (
	nativeSelectUI = tree.Feature("native-select")
	// nativeSelectTrigger's item i is the i-th trigger of the example.
	nativeSelectTrigger = tree.Sub(nativeSelectUI, tree.Name("trigger"))
)

// nativeSelectDemo is the caller-owned value of the page-first Select.
type nativeSelectDemo struct {
	environment string
	ready       bool
}

func (state *State) paintNativeSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "default" {
		environments := []string{"Development", "Staging", "Production"}
		state.drawPage(canvas, theme, size, 560, state.Text("Native Select · 系统弹窗", "Native Select · System popup"), func(c ui.C) {
			demo := ui.State[nativeSelectDemo](c, "native-select-demo")
			if !demo.ready {
				demo.ready, demo.environment = true, "Production"
			}
			ui.Select(c, &demo.environment, environments, nil).
				Label("Environment").Hint("Choose environment").FillWidth()
			state.values[0] = nativeSelectValueIndex(demo.environment)
			ui.Text(c, state.Text("平台适配器负责系统弹窗。", "The platform adapter owns the system popup.")).FontSize(theme.Metrics.FontSizeS).Muted()
		})
		return
	}

	// The states example stays on the canvas API: the page Select has no
	// invalid-trigger state for the "No team" row.
	var rows [3]ui.Box
	state.drawPage(canvas, theme, size, 560, state.Text("Native Select · 系统弹窗", "Native Select · System popup"), func(c ui.C) {
		rows[0] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight - 4)
		rows[1] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
		rows[2] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
	})
	components.PaintNativeSelect(canvas, theme, rows[0].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(0), Label: "Region", Placeholder: "Select a region", Size: components.NativeSelectSmall})
	components.PaintNativeSelect(canvas, theme, rows[1].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(1), Label: "Team", Value: "none", Display: "No team", Invalid: true})
	components.PaintNativeSelect(canvas, theme, rows[2].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(2), Label: "Plan", Value: "enterprise", Display: "Enterprise", Disabled: true})
}

func nativeSelectValueIndex(value string) int32 {
	switch value {
	case "Development":
		return 1
	case "Staging":
		return 2
	case "Production":
		return 3
	default:
		return 0
	}
}
