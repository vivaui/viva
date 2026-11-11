package componentdemo

import (
	"strings"

	"github.com/vivaui/viva/components"
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
})`},
		{ID: "placeholder", Title: "空值提示", Description: "空值显示 placeholder；聚焦后由当前平台文本服务显示光标。", Snippet: `field := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{
    ID: name, Label: "Name", Value: "",
    Placeholder: "Type in Chinese or English…",
})`},
		{ID: "invalid", Title: "校验状态", Description: "演示规则：非空文本含 @ 即解除错误，不作为完整邮箱校验。", Snippet: `invalid := value == "" || !strings.Contains(value, "@")
field := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, field.Rect(), components.InputSnapshot{
    ID: email, Label: "Email", Value: value,
    Placeholder: "name@example.com", Invalid: invalid,
})`},
		{ID: "states", Title: "禁用与只读", Description: "两个固定文本样本均不向平台 provider 暴露 Editor。", Snippet: `disabled := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, disabled.Rect(), components.InputSnapshot{
    ID: disabledID, Label: "Disabled",
    Value: "Not editable", Disabled: true,
})
readOnly := ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
components.PaintInput(canvas, theme, readOnly.Rect(), components.InputSnapshot{
    ID: readOnlyID, Label: "Read-only",
    Value: "Viva · read-only text", ReadOnly: true,
})`},
		{ID: "custom", Title: "圆角与字号", Description: "单个输入框用 Radius 和 FontSize 拥有自己的圆角与字号，光标、选区随字号缩放，旁边的输入框不受影响。", Snippet: `// A field's theme and text size belong to the call: the others keep theirs.
field := ui.Slot(c).FillWidth().Height(40)
components.PaintInput(canvas, theme.WithRadius(gfx.RadiusFull), field.Rect(), components.InputSnapshot{
    ID: search, Label: "Search", Value: value, Text: components.InputTextStyle{Size: 16},
})`},
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

// platformFocused reports whether the platform text service currently owns the
// field: it is editable, has the focus and has text to show. An empty field
// keeps painting its placeholder and caret.
func (state *State) platformFocused(canvas *gfx.Canvas, value string) bool {
	return state.editing && canvas.Focused() == state.fieldID && value != ""
}

// paintEditable fills what the component paints itself for the example's
// editable single-line field: HideEditorText keeps the platform's text, caret
// and selection out of the picture, and the caret, selection and composition
// come from the same Go editor the platform edits. The value is then drawn by
// one renderer, so it cannot move when the platform's editor appears.
func (state *State) paintEditable(canvas *gfx.Canvas, id gfx.ID, value string, snapshot *components.InputSnapshot) {
	snapshot.HideEditorText = true
	if !state.editing || state.editor == nil || canvas.Focused() != id {
		return
	}
	selection := state.editor.Selection()
	snapshot.SelectionStart, snapshot.SelectionEnd = byteOffsetOfRune(value, selection.Start), byteOffsetOfRune(value, selection.End)
	snapshot.Caret = byteOffsetOfRune(value, state.editor.Caret())
	snapshot.ShowCaret = selection.Start == selection.End
	if marked, composing := state.editor.Composition(); composing {
		snapshot.CompositionStart, snapshot.CompositionEnd = byteOffsetOfRune(value, marked.Start), byteOffsetOfRune(value, marked.End)
	}
	snapshot.Blink = &state.blink
}

// inputSample is one field a demo declares: the Slot that gives it its place,
// and the snapshot the component paints into the laid-out rectangle. A Page
// has no raw InputSnapshot control yet, so the demo lays the field out as a
// Slot and the component, which owns the look, hit and caret, paints it.
type inputSample struct {
	box         ui.Box
	id          gfx.ID
	label       string
	value       string
	placeholder string
	hint        string
	invalid     bool
	disabled    bool
	readOnly    bool
	radius      float32
	themeRadius bool
	fontSize    float32
}

func (state *State) paintInputSample(canvas *gfx.Canvas, theme gfx.Theme, sample *inputSample) {
	snapshot := components.InputSnapshot{
		ID: sample.id, Label: sample.label, Value: sample.value,
		Placeholder: sample.placeholder, Hint: sample.hint,
		Invalid: sample.invalid, Disabled: sample.disabled, ReadOnly: sample.readOnly,
	}
	snapshot.Text.Size = sample.fontSize
	state.paintEditable(canvas, sample.id, sample.value, &snapshot)
	if !sample.themeRadius {
		theme = theme.WithRadius(sample.radius)
	}
	components.PaintInput(canvas, theme, sample.box.Rect(), snapshot)
}

func (state *State) paintInput(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	mode := state.Text("Web / 原生文本服务 · 连接同一个 Go Editor", "Web / native text service · one Go Editor")
	if state.editing {
		mode = state.Text("平台文本服务 · 支持输入、选区与 IME", "Platform text service · typing, selection and IME")
	}
	if state.exampleID == "states" {
		mode = state.Text("禁用与只读 · 不连接编辑器", "Disabled and read-only · no editor")
	}
	if state.exampleID == "custom" {
		state.paintCustomInputs(canvas, theme, size, mode)
		return
	}
	var samples [2]inputSample
	caption := state.Text("Web 与原生预览均连接同一个 Go Editor。", "The Web and native previews connect the same Go Editor.")
	if state.exampleID == "states" {
		samples[0] = inputSample{
			id: inputDisabled, label: "Disabled", value: state.Text("不可编辑", "Not editable"),
			disabled: true, themeRadius: true,
		}
		samples[1] = inputSample{
			id: inputReadOnly, label: "Read-only", value: state.Text("Viva · 只读文本", "Viva · read-only text"),
			readOnly: true, themeRadius: true,
		}
		caption = state.Text("禁用与只读字段均不连接编辑器。", "Disabled and read-only fields do not connect an editor.")
	} else {
		value := state.fieldValue(state.initialInputValue())
		label, placeholder := "Name", state.Text("输入中文或 English…", "Type in Chinese or English…")
		invalid := false
		if state.exampleID == "invalid" {
			label, placeholder = "Email", "name@example.com"
			invalid = value == "" || !strings.Contains(value, "@")
		}
		samples[0] = inputSample{
			id: inputEditable, label: label, value: value, placeholder: placeholder, hint: mode,
			invalid: invalid, readOnly: !state.editing, themeRadius: true,
		}
		if state.exampleID == "invalid" {
			caption = state.Text("演示校验通过。", "The demo validation passes.")
			if invalid {
				caption = state.Text("请输入含 @ 的非空文本。", "Enter non-empty text containing @.")
			}
		}
	}
	state.drawPage(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(c ui.C) {
		ui.Text(c, mode).FontSize(theme.Metrics.FontSizeS).Muted()
		if state.exampleID == "states" {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Disabled").FontSize(theme.Metrics.FontSizeM)
				samples[0].box = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
			})
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, "Read-only").FontSize(theme.Metrics.FontSizeM)
				samples[1].box = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
			})
		} else {
			ui.Column(c).Gap(6).Children(func() {
				ui.Text(c, samples[0].label).FontSize(theme.Metrics.FontSizeM)
				samples[0].box = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
			})
		}
		ui.Text(c, caption).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	for index := range samples {
		if samples[index].id.IsZero() {
			continue
		}
		state.paintInputSample(canvas, theme, &samples[index])
	}
}

// paintCustomInputs shows three fields in one column: a pill with larger text, a
// squarer one with smaller text, and one with the theme's own radius and size.
// Only the first is connected to the editor; the others are read-only samples.
func (state *State) paintCustomInputs(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, mode string) {
	value := state.fieldValue(state.Text("圆角与字号", "Radius and size"))
	samples := [3]inputSample{
		{
			id: inputEditable, label: "Pill", value: value,
			placeholder: state.Text("输入中文或 English…", "Type in Chinese or English…"), hint: mode,
			readOnly: !state.editing, radius: gfx.RadiusFull, fontSize: 16,
		},
		{
			id: inputSquare, label: "Square", value: state.Text("较小的字号", "Smaller text"),
			readOnly: true, radius: 2, fontSize: 12,
		},
		{
			id: inputPlain, label: "Theme", value: state.Text("主题默认", "Theme default"),
			readOnly: true, themeRadius: true,
		},
	}
	state.drawPage(canvas, theme, size, 440, state.Text("Input · 文本输入", "Input · Text input"), func(c ui.C) {
		ui.Text(c, mode).FontSize(theme.Metrics.FontSizeS).Muted()
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "RadiusFull · 16").FontSize(theme.Metrics.FontSizeM)
			samples[0].box = ui.Slot(c).FillWidth().Height(40)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Radius 2 · 12").FontSize(theme.Metrics.FontSizeM)
			samples[1].box = ui.Slot(c).FillWidth().Height(28)
		})
		ui.Column(c).Gap(6).Children(func() {
			ui.Text(c, "Theme").FontSize(theme.Metrics.FontSizeM)
			samples[2].box = ui.Slot(c).FillWidth().Height(theme.Metrics.ControlHeight)
		})
	})
	for index := range samples {
		state.paintInputSample(canvas, theme, &samples[index])
	}
}
