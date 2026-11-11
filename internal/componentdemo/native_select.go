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
}`},
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
})`},
	},
}

var (
	nativeSelectUI = tree.Feature("native-select")
	// nativeSelectTrigger's item i is the i-th trigger of the example.
	nativeSelectTrigger = tree.Sub(nativeSelectUI, tree.Name("trigger"))
)

func (state *State) paintNativeSelect(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	state.drawPage(canvas, theme, size, 560, state.Text("Native Select · 系统弹窗", "Native Select · System popup"), func(c ui.C) {
		if state.exampleID == "default" {
			rows[0] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
			ui.Text(c, state.Text("平台适配器负责系统弹窗。", "The platform adapter owns the system popup.")).FontSize(theme.Metrics.FontSizeS).Muted()
			return
		}
		rows[0] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight - 4)
		rows[1] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
		rows[2] = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
	})
	if state.exampleID == "default" {
		components.PaintNativeSelect(canvas, theme, rows[0].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(0), Label: "Environment", Value: "production", Display: "Production", Hint: "Choose environment", Open: state.flags&1 != 0})
		if canvas.Clicked(nativeSelectTrigger.At(0)) {
			state.flags ^= 1
		}
		return
	}
	components.PaintNativeSelect(canvas, theme, rows[0].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(0), Label: "Region", Placeholder: "Select a region", Size: components.NativeSelectSmall})
	components.PaintNativeSelect(canvas, theme, rows[1].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(1), Label: "Team", Value: "none", Display: "No team", Invalid: true})
	components.PaintNativeSelect(canvas, theme, rows[2].Rect(), components.NativeSelectSnapshot{ID: nativeSelectTrigger.At(2), Label: "Plan", Value: "enterprise", Display: "Enterprise", Disabled: true})
}
