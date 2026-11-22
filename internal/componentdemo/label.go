package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var labelComponent = Component{
	ID: "label", Title: "Label", Category: "Forms", Summary: "字段标签、必填标记与禁用状态；点击标签聚焦关联控件。",
	paint: (*State).paintLabel,
	Examples: []Example{
		{ID: "default", Title: "字段标签", Description: "标准生产 Label 与只读字段；点击标签聚焦关联控件。", Snippet: `email := tree.ID(page, "email")
label := components.BeginLabel(canvas, theme, labelBounds).For(email).Required()
label.Paint("Work email")
components.PaintInput(canvas, theme, fieldBounds, components.InputSnapshot{
    ID: email, Label: "Work email", Placeholder: "team@viva.dev",
    ReadOnly: true,
})`, PageSnippet: `emailID := ui.ID(c)
ui.Text(c, "Work email").LabelFor(emailID)
// m.email starts as "team@viva.dev".
ui.TextFieldID(c, emailID, &m.email, "Work email", "team@viva.dev").ReadOnly(true)`, PageSnippetZh: `emailID := ui.ID(c)
ui.Text(c, "工作邮箱").LabelFor(emailID)
// m.email 初始为 "team@viva.dev"。
ui.TextFieldID(c, emailID, &m.email, "工作邮箱", "team@viva.dev").ReadOnly(true)`},
		{ID: "states", Title: "标签状态", Description: "普通、必填和禁用状态并列。", Snippet: `components.BeginLabel(canvas, theme, normalBounds).Paint("Display name")
components.BeginLabel(canvas, theme, requiredBounds).Required().Paint("Organization name")
components.BeginLabel(canvas, theme, disabledBounds).Required().
    Disabled().Paint("Disabled field")`, PageSnippet: `ui.Column(c).Gap(8).Children(func() {
    ui.Label(c, "Display name")
    ui.Label(c, "Organization name").Required()
    ui.Label(c, "Disabled field").Required().Disabled(true)
})`, PageSnippetZh: `ui.Column(c).Gap(8).Children(func() {
    ui.Label(c, "显示名称")
    ui.Label(c, "组织名称").Required()
    ui.Label(c, "禁用字段").Required().Disabled(true)
})`},
	},
}

var (
	labelUI    = tree.Feature("label")
	labelInput = tree.ID(labelUI, "input")
)

func (state *State) paintLabel(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	if state.exampleID == "default" {
		state.drawPage(canvas, theme, size, 560, state.Text("Label · 表单语义", "Label · Form semantics"), func(c ui.C) {
			email := state.pageFieldText(c, "email", "team@viva.dev")
			label := state.Text("工作邮箱", "Work email")
			ui.Column(c).FillWidth().Gap(8).Children(func() {
				ui.Text(c, label).LabelFor(labelInput)
				ui.TextFieldID(c, labelInput, email, label, "team@viva.dev").ReadOnly(true)
			})
		})
		return
	}
	state.drawPage(canvas, theme, size, 560, state.Text("Label · 表单语义", "Label · Form semantics"), func(c ui.C) {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.Label(c, state.Text("显示名称", "Display name"))
			ui.Label(c, state.Text("组织名称", "Organization name")).Required()
			ui.Label(c, state.Text("禁用字段", "Disabled field")).Required().Disabled(true)
		})
	})
}
