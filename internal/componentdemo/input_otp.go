package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputotp"
)

var inputOTPComponent = Component{
	ID: "input-otp", Title: "Input OTP", Category: "Forms", Summary: "分组验证码槽、选择、组合文本与活动光标快照。",
	paint: (*State).paintInputOTP, pointer: (*State).pointerInputOTP, initialize: (*State).initializeInputOTP,
	editable: func(exampleID string) (editableField, bool) {
		// Only digits, at most one per slot, and a digit fills the slot the caret
		// is on rather than pushing the rest along. The states example is a fixed
		// snapshot of selection, composition and error styling.
		return editableField{ID: inputOTPField, Initial: inputOTPInitial(exampleID), Accept: inputOTPDigit, MaxRunes: inputOTPSlots, Overwrite: true}, exampleID == "default"
	},
	Examples: []Example{
		{ID: "default", Title: "六位验证码", Description: "输入数字自动填入并前进，退格删除，左右键或点击槽位定位。", Snippet: `components.PaintInputOTP(canvas, theme, bounds, components.InputOTPSnapshot{
    ID: tree.ID(page, "code"), Label: "Verification code", Value: code,
    Slots: 6, GroupSize: 3, HasActiveSlot: true,
    ActiveSlot: min(len(code), 5), ShowCaret: true,
})`, PageSnippet: `// m.codeEditor starts as "482"; m.codeField is bound to it.
code := inputotp.New().
    Bind(&m.codeField, m.codeEditor).
    Slots(6).
    GroupSize(3).
    Filter(inputotp.Digits)
code.View(c, "Verification code")`, PageSnippetZh: `// m.codeEditor 初始为 "482"；m.codeField 已绑定到它。
code := inputotp.New().
    Bind(&m.codeField, m.codeEditor).
    Slots(6).
    GroupSize(3).
    Filter(inputotp.Digits)
code.View(c, "Verification code")`},
		{ID: "states", Title: "验证码状态", Description: "选择、输入法组合与错误状态。", Snippet: `components.PaintInputOTP(canvas, theme, bounds, components.InputOTPSnapshot{
    ID: tree.ID(page, "code"), Label: "Verification code", Value: "482901",
    Slots: 6, GroupSize: 3, Invalid: true, Focused: true, ShowCaret: true,
    HasSelection: true, SelectionStart: 1, SelectionEnd: 3,
    HasComposition: true, CompositionStart: 3, CompositionEnd: 5,
    ActiveSlot: 0,
})`, PageSnippet: `// m.codeEditor starts as "482901" with slots 1–3 selected.
m.codeEditor.SetSelection(text.Range{Start: 1, End: 3})
code := inputotp.New().
    Bind(&m.codeField, m.codeEditor).
    Slots(6).
    GroupSize(3).
    Invalid()
code.View(c, "Verification code")`, PageSnippetZh: `// m.codeEditor 初始为 "482901"，槽位 1–3 选中。
m.codeEditor.SetSelection(text.Range{Start: 1, End: 3})
code := inputotp.New().
    Bind(&m.codeField, m.codeEditor).
    Slots(6).
    GroupSize(3).
    Invalid()
code.View(c, "Verification code")`},
	},
}

const inputOTPSlots = 6

var (
	inputOTPUI    = tree.Feature("input-otp")
	inputOTPField = tree.ID(inputOTPUI, "code")
)

func inputOTPDigit(r rune) bool { return r >= '0' && r <= '9' }

// inputOTPInitial is the code the example starts with. The default one is still
// being typed, so the caret has an empty slot to sit in; the states example is
// a complete code.
func inputOTPInitial(exampleID string) string {
	if exampleID == "default" {
		return "482"
	}
	return "482901"
}

// inputOTPDemo is one example's page control, field and stable state. It is
// kept beside the State rather than rebuilt in each paint, so the handlers and
// draw function are stable and a warm paint allocates nothing.
type inputOTPDemo struct {
	view  *inputotp.State
	field ui.Field
	bound bool
}

// inputOTPDemos is keyed by State because the constrained demo model has no
// field for this control; a State is painted on one thread at a time.
var inputOTPDemos = map[*State]*inputOTPDemo{}

func (state *State) initializeInputOTP() {
	if state.exampleID == "default" {
		state.values[0] = int32(len(inputOTPInitial("default")))
	}
	demo := &inputOTPDemo{
		view: inputotp.New().
			FieldID(inputOTPField).
			Slots(inputOTPSlots).
			GroupSize(3),
	}
	if state.exampleID == "states" {
		demo.view.Value("482901").Invalid().Focused().ActiveSlot(0).
			Select(1, 3).Composition(3, 5)
	}
	inputOTPDemos[state] = demo
}

func inputOTPDemoFor(state *State) *inputOTPDemo {
	if demo := inputOTPDemos[state]; demo != nil {
		return demo
	}
	demo := &inputOTPDemo{
		view: inputotp.New().
			FieldID(inputOTPField).
			Slots(inputOTPSlots).
			GroupSize(3),
	}
	inputOTPDemos[state] = demo
	return demo
}

// pointerInputOTP puts the caret on the slot that was clicked, or after the last
// digit when the click is past the end of the code: the next digit typed fills
// that slot. The hit is the example's identity, so the page control is not the
// pointer's target; the view's SelectAt applies the slot geometry it painted.
func (state *State) pointerInputOTP(event gfx.PointerEvent) gfx.EventResult {
	if !state.editing || event.Kind != gfx.PointerDown || event.Target != inputOTPField {
		return gfx.EventResult{}
	}
	if inputOTPDemoFor(state).view.SelectAt(event.X) {
		return gfx.EventResult{Repaint: true}
	}
	return gfx.EventResult{}
}

func (state *State) paintInputOTP(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := inputOTPDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		demo.field.Bind(state.editor)
		demo.view.Bind(&demo.field, state.editor)
		demo.bound = true
	}
	if state.editing {
		demo.view.Editable()
	} else {
		demo.view.ReadOnly()
	}
	if !state.editing {
		switch state.exampleID {
		case "default":
			demo.view.Value(inputOTPInitial("default")).Focused().ShowCaret().
				ActiveSlot(int(state.values[0]))
		case "states":
			demo.view.Value(inputOTPInitial("states")).Invalid().Focused().
				ActiveSlot(0).Select(1, 3).Composition(3, 5)
		}
	}
	state.drawPage(canvas, theme, size, 600, state.Text("Input OTP · 验证码", "Input OTP · Verification code"), func(c ui.C) {
		demo.view.View(c, "Verification code")
		caption := "Slots: 6 · Groups: 3 + 3"
		if state.editing {
			caption = state.Text("只接受数字，最多 6 位 · 输入后自动前进 · 退格删除 · ← → 移动", "Digits only, up to 6 · advance as you type · backspace deletes · ← → moves")
		}
		ui.Paragraph(c, caption, 3).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	// Without editing the code is a static snapshot: a click moves its active
	// slot along. With editing the editor owns the position (typing, arrows, a
	// click on a slot: see pointerInputOTP).
	if !state.editing && canvas.Clicked(inputOTPField) {
		state.values[0] = (state.values[0] + 1) % inputOTPSlots
	}
}
