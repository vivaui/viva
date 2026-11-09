package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var labelComponent = Component{
	ID: "label", Title: "Label", Category: "Forms", Summary: "字段标签、必填标记与禁用状态；点击标签聚焦关联控件。",
	paint: (*State).paintLabel,
	Examples: []Example{
		{ID: "default", Title: "字段标签", Description: "标准生产 Label 与必填标记。", Snippet: `email := tree.ID(page, "email")
label := components.BeginLabel(canvas, theme, labelBounds).For(email).Required()
label.Paint("Work email")
components.PaintInput(canvas, theme, fieldBounds, components.InputSnapshot{
    ID: email, Label: "Work email", Placeholder: "team@viva.dev",
    ReadOnly: true,
})`},
		{ID: "states", Title: "标签状态", Description: "普通、必填和禁用状态并列。", Snippet: `components.BeginLabel(canvas, theme, normalBounds).Paint("Display name")
components.BeginLabel(canvas, theme, requiredBounds).Required().Paint("Organization name")
components.BeginLabel(canvas, theme, disabledBounds).Required().
    Disabled().Paint("Disabled field")`},
	},
}

var (
	labelUI    = tree.Feature("label")
	labelInput = tree.ID(labelUI, "input")
)

func (state *State) paintLabel(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	var field ui.Box
	state.page(canvas, theme, size, 560, state.Text("Label · 表单语义", "Label · Form semantics"), func(f *ui.Frame) {
		if state.exampleID == "default" {
			f.Column().FillWidth().Gap(8).Children(func() {
				rows[0] = f.Slot().FillWidth().Height(24)
				field = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
			})
			return
		}
		for index := range rows {
			rows[index] = f.Slot().FillWidth().Height(24)
		}
	})
	if state.exampleID == "default" {
		// For makes the words the label of the input: pressing them focuses it.
		email := state.Text("工作邮箱", "Work email")
		components.BeginLabel(canvas, theme, rows[0].Rect()).For(labelInput).Required().Paint(email)
		components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{ID: labelInput, Label: email, Placeholder: "team@viva.dev", ReadOnly: true})
		return
	}
	components.BeginLabel(canvas, theme, rows[0].Rect()).Paint(state.Text("显示名称", "Display name"))
	components.BeginLabel(canvas, theme, rows[1].Rect()).Required().Paint(state.Text("组织名称", "Organization name"))
	components.BeginLabel(canvas, theme, rows[2].Rect()).Required().Disabled().Paint(state.Text("禁用字段", "Disabled field"))
}
