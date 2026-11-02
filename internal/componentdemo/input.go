package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/flex"
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
		{ID: "default", Title: "真实编辑", Description: "Web 与原生 provider 连接同一个 Go Editor，支持中英文输入、选区与 IME。", Snippet: `name := tree.ID(page, "name")
f.Input(components.InputSnapshot{
    ID: name, Label: "Name", Value: value,
    Placeholder: "Type in Chinese or English…",
})`},
		{ID: "placeholder", Title: "空值提示", Description: "空值显示 placeholder；聚焦后由当前平台文本服务显示光标。", Snippet: `f.Input(components.InputSnapshot{
    ID: tree.ID(page, "name"), Label: "Name", Value: "",
    Placeholder: "Type in Chinese or English…",
})`},
		{ID: "invalid", Title: "校验状态", Description: "演示规则：非空文本含 @ 即解除错误，不作为完整邮箱校验。", Snippet: `invalid := value == "" || !strings.Contains(value, "@")
f.Input(components.InputSnapshot{
    ID: tree.ID(page, "email"), Label: "Email", Value: value,
    Placeholder: "name@example.com", Invalid: invalid,
})`},
		{ID: "states", Title: "禁用与只读", Description: "两个固定文本样本均不向平台 provider 暴露 Editor。", Snippet: `f.Input(components.InputSnapshot{
    ID: tree.ID(page, "disabled"), Label: "Disabled",
    Value: "Not editable", Disabled: true,
})
f.Input(components.InputSnapshot{
    ID: tree.ID(page, "read-only"), Label: "Read-only",
    Value: "Viva · read-only text", ReadOnly: true,
})`},
		{ID: "custom", Title: "圆角与字号", Description: "单个输入框用 Radius 和 FontSize 拥有自己的圆角与字号，光标、选区随字号缩放，旁边的输入框不受影响。", Snippet: `// In a flex page: this control only
f.Input(components.InputSnapshot{ID: search, Label: "Search", Value: value}).
    Radius(ui.RadiusFull).FontSize(16).Height(40)

// Painted by hand: this call gets a theme of its own
components.PaintInput(canvas, theme.WithRadius(2).WithFontSize(12), bounds, snapshot)`},
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
func (state *State) Editor(id ui.ID) (*text.Editor, bool) {
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

// platformFocused reports whether the platform text service currently owns the
// field: it is editable, has the focus and has text to show. An empty field
// keeps painting its placeholder and caret.
func (state *State) platformFocused(canvas *ui.Canvas, value string) bool {
	return state.editing && canvas.Focused() == state.fieldID && value != ""
}

func (state *State) paintInput(canvas *ui.Canvas, theme ui.Theme, size ui.Size) {
	mode := state.Text("Web / 原生文本服务 · 连接同一个 Go Editor", "Web / native text service · one Go Editor")
	if state.editing {
		mode = state.Text("平台文本服务 · 支持输入、选区与 IME", "Platform text service · typing, selection and IME")
	}
	if state.exampleID == "states" {
		mode = state.Text("禁用与只读 · 不连接编辑器", "Disabled and read-only · no editor")
	}
	state.page(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(f *flex.Frame) {
		f.Text(mode).FontSize(theme.Metrics.FontSizeS).Muted()
		if state.exampleID == "states" {
			f.Column().Gap(6).Children(func() {
				f.Text("Disabled").FontSize(theme.Metrics.FontSizeM)
				f.Input(components.InputSnapshot{ID: inputDisabled, Label: "Disabled", Value: state.Text("不可编辑", "Not editable"), Disabled: true})
			})
			f.Column().Gap(6).Children(func() {
				f.Text("Read-only").FontSize(theme.Metrics.FontSizeM)
				f.Input(components.InputSnapshot{ID: inputReadOnly, Label: "Read-only", Value: state.Text("Viva · 只读文本", "Viva · read-only text"), ReadOnly: true})
			})
			f.Text(state.Text("禁用与只读字段均不连接编辑器。", "Disabled and read-only fields do not connect an editor.")).FontSize(theme.Metrics.FontSizeS).Muted()
			return
		}
		if state.exampleID == "custom" {
			state.paintCustomInputs(canvas, theme, f, mode)
			return
		}
		value := state.fieldValue(state.initialInputValue())
		label, placeholder := "Name", state.Text("输入中文或 English…", "Type in Chinese or English…")
		invalid := false
		if state.exampleID == "invalid" {
			label, placeholder = "Email", "name@example.com"
			invalid = value == "" || !strings.Contains(value, "@")
		}
		f.Column().Gap(6).Children(func() {
			f.Text(label).FontSize(theme.Metrics.FontSizeM)
			f.Input(components.InputSnapshot{
				ID: inputEditable, Label: label, Value: value, Placeholder: placeholder,
				Hint: mode, Invalid: invalid, ReadOnly: !state.editing,
				PlatformEditing: state.platformFocused(canvas, value),
			})
		})
		caption := state.Text("Web 与原生预览均连接同一个 Go Editor。", "The Web and native previews connect the same Go Editor.")
		if state.exampleID == "invalid" {
			caption = state.Text("演示校验通过。", "The demo validation passes.")
			if invalid {
				caption = state.Text("请输入含 @ 的非空文本。", "Enter non-empty text containing @.")
			}
		}
		f.Text(caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
}

// paintCustomInputs shows three fields in one column: a pill with larger text, a
// squarer one with smaller text, and one with the theme's own radius and size.
// Only the first is connected to the editor; the others are read-only samples.
func (state *State) paintCustomInputs(canvas *ui.Canvas, theme ui.Theme, f *flex.Frame, mode string) {
	value := state.fieldValue(state.Text("圆角与字号", "Radius and size"))
	f.Column().Gap(6).Children(func() {
		f.Text("RadiusFull · 16").FontSize(theme.Metrics.FontSizeM)
		f.Input(components.InputSnapshot{
			ID: inputEditable, Label: "Pill", Value: value, Placeholder: state.Text("输入中文或 English…", "Type in Chinese or English…"),
			Hint: mode, ReadOnly: !state.editing, PlatformEditing: state.platformFocused(canvas, value),
		}).Radius(ui.RadiusFull).FontSize(16).Height(40)
	})
	f.Column().Gap(6).Children(func() {
		f.Text("Radius 2 · 12").FontSize(theme.Metrics.FontSizeM)
		f.Input(components.InputSnapshot{
			ID: inputSquare, Label: "Square", Value: state.Text("较小的字号", "Smaller text"), ReadOnly: true,
		}).Radius(2).FontSize(12).Height(28)
	})
	f.Column().Gap(6).Children(func() {
		f.Text("Theme").FontSize(theme.Metrics.FontSizeM)
		f.Input(components.InputSnapshot{
			ID: inputPlain, Label: "Theme", Value: state.Text("主题默认", "Theme default"), ReadOnly: true,
		})
	})
}
