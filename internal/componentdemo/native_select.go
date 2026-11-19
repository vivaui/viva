package componentdemo

import (
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
		{ID: "states", Title: "选择器状态", Description: "紧凑、无效与禁用触发器。", Snippet: `regions := []string{"us-east", "eu-west", "ap-south"}
teams := []string{"No team"}
plans := []string{"Enterprise"}
ui.Select(c, &region, regions, nil).
    Label("Region").Placeholder("Select a region").Small()
ui.Select(c, &team, teams, nil).
    Label("Team").Placeholder("No team").Invalid(team == "")
plan := "Enterprise"
ui.Select(c, &plan, plans, nil).Label("Plan").Disabled(true)`, PageSnippet: `regions := []string{"us-east", "eu-west", "ap-south"}
teams := []string{"No team"}
plans := []string{"Enterprise"}
ui.Select(c, &m.region, regions, nil).
    Label("Region").Placeholder("Select a region").Small()
ui.Select(c, &m.team, teams, nil).
    Label("Team").Placeholder("No team").Invalid(m.team == "")
m.plan = "Enterprise"
ui.Select(c, &m.plan, plans, nil).Label("Plan").Disabled(true)`, PageSnippetZh: `regions := []string{"us-east", "eu-west", "ap-south"}
teams := []string{"No team"}
plans := []string{"Enterprise"}
ui.Select(c, &m.region, regions, nil).
    Label("Region").Placeholder("Select a region").Small()
ui.Select(c, &m.team, teams, nil).
    Label("Team").Placeholder("No team").Invalid(m.team == "")
m.plan = "Enterprise"
ui.Select(c, &m.plan, plans, nil).Label("Plan").Disabled(true)`},
	},
}

// nativeSelectDemo is the caller-owned value of the page-first Select.
type nativeSelectDemo struct {
	environment string
	ready       bool
}

// nativeSelectStatesDemo is the caller-owned value of each trigger in the states
// example. The team Select stays invalid until a value is chosen.
type nativeSelectStatesDemo struct {
	region, team, plan string
	ready              bool
}

func (state *State) paintNativeSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	environments := []string{"Development", "Staging", "Production"}
	if state.exampleID == "default" {
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

	regions := []string{"us-east", "eu-west", "ap-south"}
	teams := []string{"No team"}
	plans := []string{"Enterprise"}
	state.drawPage(canvas, theme, size, 560, state.Text("Native Select · 系统弹窗", "Native Select · System popup"), func(c ui.C) {
		demo := ui.State[nativeSelectStatesDemo](c, "native-select-states")
		if !demo.ready {
			demo.ready, demo.plan = true, "Enterprise"
		}
		ui.Select(c, &demo.region, regions, nil).
			Label("Region").Placeholder("Select a region").Small().FillWidth()
		ui.Select(c, &demo.team, teams, nil).
			Label("Team").Placeholder("No team").Invalid(demo.team == "").FillWidth()
		ui.Select(c, &demo.plan, plans, nil).Label("Plan").Disabled(true).FillWidth()
	})
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
