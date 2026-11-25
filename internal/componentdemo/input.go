package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var inputComponent = Component{
	ID: "input", Title: "Input", Category: "Forms", Summary: "Web 与原生预览连接同一个应用持有的 Go Editor。",
	paint: (*State).paintInput,
	editable: func(exampleID string) (editableField, bool) {
		inputState := &State{exampleID: exampleID}
		return editableField{ID: inputEditable, Initial: inputState.initialInputValue()}, exampleID != "states"
	},
	Examples: []Example{
		{ID: "default", Title: "真实编辑", Description: "Web 与原生 provider 连接同一个 Go Editor，支持中英文输入、选区与 IME。", Snippet: `field := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{
    ID: name, Label: "Name", Value: value,
    Placeholder: "Type in Chinese or English…",
})`, PageSnippet: `// m.name starts as "Viva text input".
ui.TextField(c, &m.name, "Name", "Type in Chinese or English…")`, PageSnippetZh: `// m.name starts as "Viva 文本输入".
ui.TextField(c, &m.name, "Name", "输入中文或 English…")`},
		{ID: "placeholder", Title: "空值提示", Description: "空值显示 placeholder；聚焦后由当前平台文本服务显示光标。", Snippet: `field := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{
    ID: name, Label: "Name", Value: "",
    Placeholder: "Type in Chinese or English…",
})`, PageSnippet: `// m.name is empty.
ui.TextField(c, &m.name, "Name", "Type in Chinese or English…")`, PageSnippetZh: `// m.name is empty.
ui.TextField(c, &m.name, "Name", "输入中文或 English…")`},
		{ID: "invalid", Title: "校验状态", Description: "演示规则：非空文本含 @ 即解除错误，不作为完整邮箱校验。", Snippet: `invalid := value == "" || !strings.Contains(value, "@")
field := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{
    ID: email, Label: "Email", Value: value,
    Placeholder: "name@example.com", Invalid: invalid,
})`, PageSnippet: `// m.email starts as "not-an-email".
m.email.Invalid = !strings.Contains(m.email.Text(), "@")
ui.TextField(c, &m.email, "Email", "name@example.com", ui.WithID(ui.ID(c)))`, PageSnippetZh: `// m.email 初始为 "not-an-email"。
m.email.Invalid = !strings.Contains(m.email.Text(), "@")
ui.TextField(c, &m.email, "Email", "name@example.com", ui.WithID(ui.ID(c)))`},
		{ID: "states", Title: "禁用与只读", Description: "两个固定文本样本均不向平台 provider 暴露 Editor。", Snippet: `disabled := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, disabled.Rect(), components.InputSnapshot{
    ID: disabledID, Label: "Disabled",
    Value: "Not editable", Disabled: true,
})
readOnly := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, readOnly.Rect(), components.InputSnapshot{
    ID: readOnlyID, Label: "Read-only",
    Value: "Viva · read-only text", ReadOnly: true,
})`, PageSnippet: `// m.disabled starts as "Not editable"; m.readOnly as "Viva · read-only text".
m.disabled.Disabled = true
ui.TextField(c, &m.disabled, "Disabled", "")
ui.TextField(c, &m.readOnly, "Read-only", "").ReadOnly(true)`, PageSnippetZh: `// m.disabled 初始为 "不可编辑"；m.readOnly 初始为 "Viva · 只读文本"。
m.disabled.Disabled = true
ui.TextField(c, &m.disabled, "Disabled", "")
ui.TextField(c, &m.readOnly, "Read-only", "").ReadOnly(true)`},
		{ID: "custom", Title: "圆角与字号", Description: "单个输入框用 Radius 和 FontSize 拥有自己的圆角与字号，光标、选区随字号缩放，旁边的输入框不受影响。", Snippet: `// A field's theme and text size belong to the call: the others keep theirs.
field := ui.Slot(c).FillWidth().Height(40)
components.PaintInput(canvas, theme.WithRadius(gfx.RadiusFull), field.Rect(), components.InputSnapshot{
    ID: search, Label: "Search", Value: value, Text: components.InputTextStyle{Size: 16},
})`, PageSnippet: `// m.pill starts as "Radius and size".
ui.TextField(c, &m.pill, "Pill", "").FontSize(16).Radius(gfx.RadiusFull)
// m.square starts as "Smaller text"; m.plain as "Theme default".
ui.TextField(c, &m.square, "Square", "").FontSize(12).Radius(2).ReadOnly(true)
ui.TextField(c, &m.plain, "Theme", "").ReadOnly(true)`, PageSnippetZh: `// m.pill starts as "圆角与字号".
ui.TextField(c, &m.pill, "Pill", "").FontSize(16).Radius(gfx.RadiusFull)
// m.square starts as "较小的字号"; m.plain as "主题默认"。
ui.TextField(c, &m.square, "Square", "").FontSize(12).Radius(2).ReadOnly(true)
ui.TextField(c, &m.plain, "Theme", "").ReadOnly(true)`},
	},
}

var (
	inputUI       = tree.Feature("input")
	inputEditable = tree.ID(inputUI, "editable")
	inputDisabled = tree.ID(inputUI, "disabled")
	inputReadOnly = tree.ID(inputUI, "read-only")
	inputSquare   = tree.ID(inputUI, "square")
	inputPlain    = tree.ID(inputUI, "plain")
)

// The input example's caller-owned sample text, in both preview languages.
const (
	inputSampleChinese = "Viva 文本输入"
	inputSampleEnglish = "Viva text input"
)

func (state *State) initialInputValue() string {
	switch state.exampleID {
	case "default":
		return state.Text(inputSampleChinese, inputSampleEnglish)
	case "invalid":
		return "not-an-email"
	}
	return ""
}

// editableInitial returns a caller-owned starting text in the preview's
// language. Component.editable only receives the example ID, so the input
// example's sample always comes back Chinese; the editor is created with the
// localized one.
func (state *State) editableInitial(initial string) string {
	if initial == inputSampleChinese {
		return state.Text(inputSampleChinese, inputSampleEnglish)
	}
	return initial
}

// EnableTextEditing connects the example's editable field, if it has one, to a
// Go-owned text.Editor. Runners call it when their platform provides a text
// service, then resolve the focused hit through Editor. Calling it again keeps
// the editor, and with it any text typed so far.
func (state *State) EnableTextEditing() {
	if state.editing || state.descriptor == nil || state.descriptor.editable == nil {
		return
	}
	field, ok := state.descriptor.editable(state.exampleID)
	if !ok {
		return
	}
	state.editing = true
	state.fieldID = field.ID
	state.editor = text.NewEditor(state.editableInitial(field.Initial))
	if state.descriptor.editor != nil {
		state.editor = state.descriptor.editor(state)
	}
	if field.Accept != nil || field.MaxRunes > 0 {
		state.editor.Restrict(field.Accept, field.MaxRunes)
	}
	state.editor.SetOverwrite(field.Overwrite)
}

// Editor returns the editor behind a text hit, or false for every hit that is
// not the example's editable field (and for all hits before EnableTextEditing).
func (state *State) Editor(id gfx.ID) (*text.Editor, bool) {
	if !state.editing || id != state.fieldID || state.editor == nil {
		return nil, false
	}
	return state.editor, true
}

// fieldValue is the text a field shows: what the user has typed once editing
// is enabled, the example's initial text before that.
func (state *State) fieldValue(initial string) string {
	if !state.editing {
		return initial
	}
	if state.editor.RuneCount() == 0 {
		return ""
	}
	return state.editor.String()
}

func (state *State) paintInput(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	mode := state.Text("Web / 原生文本服务 · 连接同一个 Go Editor", "Web / native text service · one Go Editor")
	if state.editing {
		mode = state.Text("平台文本服务 · 支持输入、选区与 IME", "Platform text service · typing, selection and IME")
	}
	switch state.exampleID {
	case "states":
		mode = state.Text("禁用与只读 · 不连接编辑器", "Disabled and read-only · no editor")
		state.paintInputStates(canvas, theme, size, mode)
	case "custom":
		state.paintCustomInputs(canvas, theme, size, mode)
	default:
		state.paintInputPage(canvas, theme, size, mode)
	}
}

// paintInputPage is the page TextField of the single-field examples: the value
// lives in the Field the page paints, so the preview and the snippet connect the
// same editor. Without a platform text service the field is read-only: it keeps
// its value and ordinary look, and takes no edits.
func (state *State) paintInputPage(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, mode string) {
	initial := state.initialInputValue()
	label, placeholder := "Name", state.Text("输入中文或 English…", "Type in Chinese or English…")
	invalidExample := state.exampleID == "invalid"
	if invalidExample {
		label, placeholder = "Email", "name@example.com"
	}
	captionText := state.Text("Web 与原生预览均连接同一个 Go Editor。", "The Web and native previews connect the same Go Editor.")
	state.drawPage(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(c ui.C) {
		field := ui.State[ui.Field](c, "input")
		state.bindPageField(field, initial)
		field.Hint = mode
		if invalidExample {
			field.Invalid = field.Text() == "" || !strings.Contains(field.Text(), "@")
			captionText = state.Text("演示校验通过。", "The demo validation passes.")
			if field.Invalid {
				captionText = state.Text("请输入含 @ 的非空文本。", "Enter non-empty text containing @.")
			}
		}
		ui.Text(c, mode).FontSize(theme.Metrics.FontSizeS).Muted()
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, label).FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, field, label, placeholder, ui.WithID(inputEditable)).ReadOnly(!state.editing)
		})
		ui.Text(c, captionText).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}

// paintInputStates shows a disabled field and a read-only one as page
// TextFields: the disabled field is dimmed and unavailable, the read-only field
// keeps its ordinary look and value but takes no edits, and neither reaches a
// platform editor.
func (state *State) paintInputStates(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, mode string) {
	state.drawPage(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(c ui.C) {
		disabled := state.pageFieldText(c, "disabled", state.Text("不可编辑", "Not editable"))
		disabled.Disabled = true
		readOnly := state.pageFieldText(c, "read-only", state.Text("Viva · 只读文本", "Viva · read-only text"))
		ui.Text(c, mode).FontSize(theme.Metrics.FontSizeS).Muted()
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Disabled").FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, disabled, "Disabled", "", ui.WithID(inputDisabled))
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Read-only").FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, readOnly, "Read-only", "", ui.WithID(inputReadOnly)).ReadOnly(true)
		})
		ui.Text(c, state.Text("禁用与只读字段均不连接编辑器。", "Disabled and read-only fields do not connect an editor.")).
			FontSize(theme.Metrics.FontSizeS).Muted()
	})
}

// pageFieldText is the page Field kept under name with initial as its text. A
// field the example does not edit is seeded once and keeps what the model gives
// it, so its visible value survives repaints.
func (state *State) pageFieldText(c ui.C, name, initial string) *ui.Field {
	field := ui.State[ui.Field](c, name)
	if field.GetEditor() == nil {
		field.SetText(initial)
	}
	return field
}

// bindPageField binds the demo's platform editor to a page Field, or seeds the
// Field's own editor with the example's initial text when no platform text
// service is attached.
func (state *State) bindPageField(field *ui.Field, initial string) {
	if state.editing && state.editor != nil {
		if field.GetEditor() != state.editor {
			field.Bind(state.editor)
		}
		return
	}
	if field.GetEditor() == nil {
		field.SetText(initial)
	}
}

// paintCustomInputs shows three page TextFields in one column: a pill with
// larger text, a squarer one with smaller text, and one with the theme's own
// radius and size. Only the first is connected to the editor; the others are
// read-only.
func (state *State) paintCustomInputs(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, mode string) {
	state.drawPage(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(c ui.C) {
		pill := ui.State[ui.Field](c, "pill")
		state.bindPageField(pill, state.Text("圆角与字号", "Radius and size"))
		square := state.pageFieldText(c, "square", state.Text("较小的字号", "Smaller text"))
		plain := state.pageFieldText(c, "plain", state.Text("主题默认", "Theme default"))
		ui.Text(c, mode).FontSize(theme.Metrics.FontSizeS).Muted()
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "RadiusFull · 16").FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, pill, "Pill", state.Text("输入中文或 English…", "Type in Chinese or English…"), ui.WithID(inputEditable)).
				FontSize(16).Radius(gfx.RadiusFull).ReadOnly(!state.editing)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Radius 2 · 12").FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, square, "Square", "", ui.WithID(inputSquare)).FontSize(12).Radius(2).ReadOnly(true)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Theme").FontSize(theme.Metrics.FontSizeM)
			ui.TextField(c, plain, "Theme", "", ui.WithID(inputPlain)).ReadOnly(true)
		})
	})
}
