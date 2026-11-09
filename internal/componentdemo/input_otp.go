package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
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
})`},
		{ID: "states", Title: "验证码状态", Description: "选择、输入法组合与错误状态。", Snippet: `components.PaintInputOTP(canvas, theme, bounds, components.InputOTPSnapshot{
    ID: tree.ID(page, "code"), Label: "Verification code", Value: "482901",
    Slots: 6, GroupSize: 3, Invalid: true, Focused: true, ShowCaret: true,
    HasSelection: true, SelectionStart: 1, SelectionEnd: 3,
    HasComposition: true, CompositionStart: 3, CompositionEnd: 5,
    ActiveSlot: 0,
})`},
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

// initializeInputOTP places the static (not editable) default example's active
// slot at the first empty one.
func (state *State) initializeInputOTP() {
	if state.exampleID == "default" {
		state.values[0] = int32(len(inputOTPInitial("default")))
	}
}

// pointerInputOTP puts the caret on the slot that was clicked, or after the last
// digit when the click is past the end of the code: the next digit typed fills
// that slot.
func (state *State) pointerInputOTP(event gfx.PointerEvent) gfx.EventResult {
	if !state.editing || event.Kind != gfx.PointerDown || event.Target != inputOTPField {
		return gfx.EventResult{}
	}
	layout := state.inputOTPLayout(inputOTPLeft(pointerViewport(event).Width))
	slot := inputOTPSlots - 1
	for index := 0; index < inputOTPSlots; index++ {
		bounds := layout.SlotBounds(index)
		if event.X < bounds.X+bounds.Width {
			slot = index
			break
		}
	}
	caret := min(slot, state.editor.RuneCount())
	state.editor.SetSelection(textRange(caret, caret))
	return gfx.EventResult{Repaint: true}
}

// inputOTPWidth is the widest the demo page gets; inputOTPLeft is where its
// content starts in a viewport of the given width (the page pads 24 and centers
// the column). The pointer callback needs the slot geometry without having
// painted, so it derives it the way the page lays it out.
const inputOTPWidth = 600

func inputOTPLeft(viewport float32) float32 {
	return (viewport - min(viewport-48, inputOTPWidth)) / 2
}

// inputOTPLayout is the slot geometry for a code whose left edge is at x, shared
// by painting and by hit-testing a click to a slot.
func (state *State) inputOTPLayout(x float32) components.InputOTPLayout {
	return components.PaintInputOTP(nil, state.theme(), gfx.Rect{X: x}, components.InputOTPSnapshot{Slots: inputOTPSlots, GroupSize: 3})
}

func (state *State) paintInputOTP(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	measure := state.inputOTPLayout(0).Bounds
	var code ui.Box
	state.page(canvas, theme, size, inputOTPWidth, state.Text("Input OTP · 验证码", "Input OTP · Verification code"), func(f *ui.Frame) {
		code = f.Slot().Size(measure.Width, measure.Height).Self(ui.Start)
		caption := "Slots: 6 · Groups: 3 + 3"
		if state.editing {
			caption = state.Text("只接受数字，最多 6 位 · 输入后自动前进 · 退格删除 · ← → 移动", "Digits only, up to 6 · advance as you type · backspace deletes · ← → moves")
		}
		f.Paragraph(caption, 3).FontSize(theme.Metrics.FontSizeS).Muted()
	})
	bounds := code.Rect()
	value := state.fieldValue(inputOTPInitial(state.exampleID))
	snapshot := components.InputOTPSnapshot{
		ID: inputOTPField, Label: "Verification code", Value: value, Slots: inputOTPSlots, GroupSize: 3,
		ReadOnly: !state.editing, PlatformInput: state.editing, HasActiveSlot: true, ShowCaret: true,
	}
	if state.editing {
		// The editor is the source of truth: the active slot is where the caret
		// is, clamped to the last slot once the code is complete, and the
		// selection and any composition come straight from it.
		selection := state.editor.Selection()
		snapshot.ActiveSlot = min(selection.Start, inputOTPSlots-1)
		if selection.Start != selection.End {
			snapshot.HasSelection, snapshot.SelectionStart, snapshot.SelectionEnd = true, selection.Start, selection.End
		}
		if composition, composing := state.editor.Composition(); composing {
			snapshot.HasComposition, snapshot.CompositionStart, snapshot.CompositionEnd = true, composition.Start, composition.End
		}
	} else {
		// A snapshot is shown focused so its ring and caret are visible.
		snapshot.Focused = true
		snapshot.ActiveSlot = int(state.values[0])
	}
	if state.exampleID == "states" {
		snapshot.Invalid = true
		snapshot.HasSelection = true
		snapshot.SelectionStart = 1
		snapshot.SelectionEnd = 3
		snapshot.HasComposition = true
		snapshot.CompositionStart = 3
		snapshot.CompositionEnd = 5
		snapshot.ActiveSlot = 0
	}
	components.PaintInputOTP(canvas, theme, gfx.Rect{X: bounds.X, Y: bounds.Y}, snapshot)
	// Without editing the code is a static snapshot: a click moves its active slot
	// along. With editing the editor owns the position (typing, arrows, a click on
	// a slot: see pointerInputOTP).
	if canvas.Clicked(inputOTPField) && !state.editing {
		state.values[0] = (state.values[0] + 1) % inputOTPSlots
	}
}
