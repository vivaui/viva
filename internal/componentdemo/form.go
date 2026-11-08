package componentdemo

import (
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
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

func (state *State) paintForm(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	var slot flex.Box
	state.page(canvas, theme, size, 560, state.Text("Form · 显式字段状态", "Form · Explicit field state"), func(f *flex.Frame) {
		slot = f.Slot().FillWidth().Height(150)
	})
	value := state.fieldValue(formInitial(state.exampleID))
	snapshot := components.FormFieldSnapshot{Label: "Username", Description: "This name is visible to collaborators.", Required: true, ControlID: formInput}
	if state.exampleID == "message" {
		snapshot.Message = "Username is available."
	}
	if state.exampleID == "invalid" && utf8.RuneCountInString(value) < 3 {
		snapshot.Invalid = true
		snapshot.Error = "Use at least three characters."
	}
	layout := components.PaintFormField(canvas, theme, slot.Rect(), snapshot)
	input := components.InputSnapshot{
		ID: formInput, Label: layout.Semantics.Label, Value: value, Hint: layout.Semantics.AssistiveText(), Invalid: layout.Semantics.Invalid,
		ReadOnly: !state.editing,
	}
	state.paintEditable(canvas, formInput, value, &input)
	components.PaintInput(canvas, theme, layout.Control, input)
}
