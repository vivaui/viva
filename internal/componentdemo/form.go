package componentdemo

import (
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var formComponent = Component{
	ID: "form", Title: "Form", Category: "Forms", Summary: "无注册表的表单字段布局与显式控件语义。",
	paint: (*State).paintForm,
	editable: func(exampleID string) (editableField, bool) {
		return editableField{ID: formInput, Initial: formInitial(exampleID)}, true
	},
	Examples: []Example{
		{ID: "default", Title: "账户字段", Description: "生产 FormFieldLayout 与生产 Input 组合。", Snippet: `username := tree.ID(page, "username")
field := components.PaintFormField(canvas, theme, bounds, components.FormFieldSnapshot{
    Label:       "Username",
    Description: "This name is visible to collaborators.",
    Required:    true,
    ControlID:   username,
})
components.PaintInput(canvas, theme, field.Control, components.InputSnapshot{
    ID: username, Label: field.Semantics.Label,
    Value: value, Hint: field.Semantics.AssistiveText(),
})`, PageSnippet: `m.field.Description = "This name is visible to collaborators."
m.field.Required = true
// m.username starts as "gopher".
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`, PageSnippetZh: `m.field.Description = "This name is visible to collaborators."
m.field.Required = true
// m.username starts as "gopher".
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`},
		{ID: "message", Title: "帮助消息", Description: "字段说明和服务端消息由调用方显式传入。", Snippet: `username := tree.ID(page, "username")
field := components.PaintFormField(canvas, theme, bounds, components.FormFieldSnapshot{
    Label:       "Username",
    Description: "This name is visible to collaborators.",
    Message:     "Username is available.",
    Required:    true,
    ControlID:   username,
})
components.PaintInput(canvas, theme, field.Control, components.InputSnapshot{
    ID: username, Label: field.Semantics.Label,
    Value: value, Hint: field.Semantics.AssistiveText(),
})`, PageSnippet: `m.field.Description = "This name is visible to collaborators."
m.field.Message = "Username is available."
m.field.Required = true
// m.username starts as "gopher".
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`, PageSnippetZh: `m.field.Description = "This name is visible to collaborators."
m.field.Message = "Username is available."
m.field.Required = true
// m.username starts as "gopher".
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`},
		{ID: "invalid", Title: "验证错误", Description: "错误覆盖普通消息并同步到控件语义。", Snippet: `username := tree.ID(page, "username")
snapshot := components.FormFieldSnapshot{
    Label: "Username", Required: true, ControlID: username,
    Description: "This name is visible to collaborators.",
}
if utf8.RuneCountInString(value) < 3 {
    snapshot.Invalid, snapshot.Error = true, "Use at least three characters."
}
layout := components.PaintFormField(canvas, theme, bounds, snapshot)
components.PaintInput(canvas, theme, layout.Control, components.InputSnapshot{
    ID: username, Label: layout.Semantics.Label, Value: value,
    Hint: layout.Semantics.AssistiveText(), Invalid: layout.Semantics.Invalid,
})`, PageSnippet: `// m.username starts as "go".
m.field.Description = "This name is visible to collaborators."
m.field.Required = true
m.field.Invalid = utf8.RuneCountInString(m.username) < 3
if m.field.Invalid {
    m.field.Error = "Use at least three characters."
}
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`, PageSnippetZh: `// m.username starts as "go".
m.field.Description = "This name is visible to collaborators."
m.field.Required = true
m.field.Invalid = utf8.RuneCountInString(m.username) < 3
if m.field.Invalid {
    m.field.Error = "Use at least three characters."
}
ui.FormField(c, &m.field, "Username").Children(func() {
    ui.TextField(c, &m.username, "Username", "")
})`},
	},
}

var (
	formUI    = tree.Feature("form")
	formInput = tree.ID(formUI, "input")
)

// formInitial is the username the form starts with. The error example starts
// one character short of valid, so typing one more clears its error.
func formInitial(exampleID string) string {
	if exampleID == "invalid" {
		return "go"
	}
	return "gopher"
}

func (state *State) paintForm(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	initial := formInitial(state.exampleID)
	state.drawPage(canvas, theme, size, 560, state.Text("Form · 显式字段状态", "Form · Explicit field state"), func(c ui.C) {
		field := ui.State[ui.Field](c, "username")
		form := ui.State[ui.FormFieldState](c, "field")
		state.bindPageField(field, initial)
		form.Description = "This name is visible to collaborators."
		form.Required = true
		form.For = formInput
		if state.exampleID == "message" {
			form.Message = "Username is available."
		}
		if state.exampleID == "invalid" {
			field.Invalid = utf8.RuneCountInString(field.Text()) < 3
			form.Invalid = field.Invalid
			form.Error = ""
			if field.Invalid {
				form.Error = "Use at least three characters."
			}
		}
		ui.FormField(c, form, "Username").Children(func() {
			ui.TextField(c, field, "Username", "", ui.WithID(formInput)).ReadOnly(!state.editing)
		})
	})
}
