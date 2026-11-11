package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/components"
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
})`},
		{ID: "orientations", Title: "响应式方向", Description: "窄窗口垂直，宽窗口切换为水平布局。", Snippet: `email := tree.ID(page, "email")
field := components.BeginField(canvas, theme, bounds).For(email).
    Orientation(components.FieldResponsive)
control := field.Paint("Work email", "Used only for release notifications.", "")
components.PaintInput(canvas, theme, control, components.InputSnapshot{
    ID: email, Label: "Work email", Value: value,
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
	var slot ui.Box
	state.drawPage(canvas, theme, size, 600, state.Text("Field · 结构化字段", "Field · Structured field"), func(c ui.C) {
		slot = ui.Slot(c).FillWidth().Height(136)
	})
	field := components.BeginField(canvas, theme, slot.Rect()).For(fieldInput)
	if state.exampleID == "orientations" {
		field = field.Orientation(components.FieldResponsive)
	}
	value := state.fieldValue(fieldInitial(state.exampleID))
	required := state.exampleID == "invalid"
	invalid := required && !strings.Contains(value, "@")
	if required {
		field = field.Required()
	}
	if invalid {
		field = field.Invalid()
	}
	errorText := ""
	if invalid {
		errorText = "Enter a valid work email."
	}
	control := field.Paint("Work email", "Used only for release notifications.", errorText)
	input := components.InputSnapshot{
		ID: fieldInput, Label: "Work email", Value: value, Hint: "Used only for release notifications", Invalid: invalid,
		ReadOnly: !state.editing,
	}
	state.paintEditable(canvas, fieldInput, value, &input)
	components.PaintInput(canvas, theme, control, input)
}
