package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var fieldComponent = Component{
	ID: "field", Title: "Field", Category: "Forms", Summary: "标签、说明、错误与调用方控件槽的结构化字段。",
	paint: (*State).paintField,
	editable: func(exampleID string) (editableField, bool) {
		return editableField{ID: fieldInput, Initial: fieldInitial(exampleID)}, true
	},
	Examples: []Example{
		{ID: "default", Title: "垂直字段", Description: "生产 Field 返回控件槽，预览填入生产 Input。", Snippet: `email := tree.ID(page, "email")
field := components.BeginField(canvas, theme, bounds).For(email)
control := field.Paint("Work email", "Used only for release notifications.", "")
components.PaintInput(canvas, theme, control, components.InputSnapshot{
    ID: email, Label: "Work email", Value: value,
})`, PageSnippet: `m.field.Description = "Used only for release notifications."
// m.email starts as "team@viva.dev".
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`, PageSnippetZh: `m.field.Description = "Used only for release notifications."
// m.email starts as "team@viva.dev".
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`},
		{ID: "orientations", Title: "响应式方向", Description: "窄窗口垂直，宽窗口切换为水平布局。", Snippet: `email := tree.ID(page, "email")
field := components.BeginField(canvas, theme, bounds).For(email).
    Orientation(components.FieldResponsive)
control := field.Paint("Work email", "Used only for release notifications.", "")
components.PaintInput(canvas, theme, control, components.InputSnapshot{
    ID: email, Label: "Work email", Value: value,
})`, PageSnippet: `m.field.Description = "Used only for release notifications."
m.field.Orientation = ui.FieldResponsive
// m.email starts as "team@viva.dev".
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`, PageSnippetZh: `m.field.Description = "Used only for release notifications."
m.field.Orientation = ui.FieldResponsive
// m.email starts as "team@viva.dev".
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`},
		{ID: "invalid", Title: "错误状态", Description: "必填、无效元数据与无效输入保持一致。", Snippet: `email := tree.ID(page, "email")
invalid := !strings.Contains(value, "@")
field := components.BeginField(canvas, theme, bounds).For(email).Required()
errorText := ""
if invalid {
    field = field.Invalid()
    errorText = "Enter a valid work email."
}
control := field.Paint("Work email", "Used only for release notifications.", errorText)
components.PaintInput(canvas, theme, control, components.InputSnapshot{
    ID: email, Label: "Work email", Value: value, Invalid: invalid,
})`, PageSnippet: `// m.email starts as "team".
m.field.Description = "Used only for release notifications."
m.field.Required = true
m.field.Invalid = !strings.Contains(m.email, "@")
if m.field.Invalid {
    m.field.Error = "Enter a valid work email."
}
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`, PageSnippetZh: `// m.email starts as "team".
m.field.Description = "Used only for release notifications."
m.field.Required = true
m.field.Invalid = !strings.Contains(m.email, "@")
if m.field.Invalid {
    m.field.Error = "Enter a valid work email."
}
ui.FormField(c, &m.field, "Work email").Children(func() {
    ui.TextField(c, &m.email, "Work email", "")
})`},
	},
}

var (
	fieldUI    = tree.Feature("field")
	fieldInput = tree.ID(fieldUI, "input")
)

// fieldInitial is the field's starting text. The error example starts without
// an "@", so its error is real and clears once the address is complete.
func fieldInitial(exampleID string) string {
	if exampleID == "invalid" {
		return "team"
	}
	return "team@viva.dev"
}

func (state *State) paintField(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	initial := fieldInitial(state.exampleID)
	state.drawPage(canvas, theme, size, 600, state.Text("Field · 结构化字段", "Field · Structured field"), func(c ui.C) {
		field := ui.State[ui.Field](c, "email")
		form := ui.State[ui.FormFieldState](c, "field")
		state.bindPageField(field, initial)
		form.Description = "Used only for release notifications."
		form.For = fieldInput
		if state.exampleID == "orientations" {
			form.Orientation = ui.FieldResponsive
		}
		if state.exampleID == "invalid" {
			field.Invalid = !strings.Contains(field.Text(), "@")
			form.Invalid = field.Invalid
			form.Required = true
			form.Error = ""
			if field.Invalid {
				form.Error = "Enter a valid work email."
			}
		}
		ui.FormField(c, form, "Work email").Children(func() {
			ui.TextFieldID(c, fieldInput, field, "Work email", "").ReadOnly(!state.editing)
		})
	})
}
