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
})`, PageSnippet: `emailID := ui.ID(c)
ui.Text(c, "Work email").LabelFor(emailID)
ui.TextFieldID(c, emailID, &m.email, "Work email", "team@viva.dev")`, PageSnippetZh: `emailID := ui.ID(c)
ui.Text(c, "工作邮箱").LabelFor(emailID)
ui.TextFieldID(c, emailID, &m.email, "工作邮箱", "team@viva.dev")`},
		{ID: "states", Title: "标签状态", Description: "普通、必填和禁用状态并列。", Snippet: `components.BeginLabel(canvas, theme, normalBounds).Paint("Display name")
components.BeginLabel(canvas, theme, requiredBounds).Required().Paint("Organization name")
components.BeginLabel(canvas, theme, disabledBounds).Required().
    Disabled().Paint("Disabled field")`, PageSnippet: `displayID := ui.ID(c)
ui.Text(c, "Display name").LabelFor(displayID)
ui.TextFieldID(c, displayID, &m.displayName, "Display name", "")

organizationID := ui.ID(c)
ui.Text(c, "Organization name").LabelFor(organizationID)
ui.TextFieldID(c, organizationID, &m.organization, "Organization name", "")

disabledID := ui.ID(c)
ui.Text(c, "Disabled field").LabelFor(disabledID)
ui.TextFieldID(c, disabledID, &m.disabledField, "Disabled field", "").Disabled(true)`, PageSnippetZh: `displayID := ui.ID(c)
ui.Text(c, "显示名称").LabelFor(displayID)
ui.TextFieldID(c, displayID, &m.displayName, "显示名称", "")

organizationID := ui.ID(c)
ui.Text(c, "组织名称").LabelFor(organizationID)
ui.TextFieldID(c, organizationID, &m.organization, "组织名称", "")

disabledID := ui.ID(c)
ui.Text(c, "禁用字段").LabelFor(disabledID)
ui.TextFieldID(c, disabledID, &m.disabledField, "禁用字段", "").Disabled(true)`},
	},
}

var (
	labelUI    = tree.Feature("label")
	labelInput = tree.ID(labelUI, "input")
)

func (state *State) paintLabel(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	var field ui.Box
	state.drawPage(canvas, theme, size, 560, state.Text("Label · 表单语义", "Label · Form semantics"), func(c ui.C) {
		if state.exampleID == "default" {
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				rows[0] = ui.Slot(c).FillWidth().Height(24)
				field = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
			})
			return
		}
		for index := range rows {
			rows[index] = ui.Slot(c).FillWidth().Height(24)
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
