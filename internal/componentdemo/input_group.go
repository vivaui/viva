package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

var inputGroupComponent = Component{
	ID: "input-group", Title: "Input Group", Category: "Forms", Summary: "输入快照与文本、按钮、键帽附加槽的组合。",
	paint: (*State).paintInputGroup, initialize: (*State).initializeInputGroup,
	editable: func(exampleID string) (editableField, bool) {
		return editableField{ID: inputGroupField, Initial: inputGroupInitial(exampleID)}, exampleID != "states"
	},
	Examples: []Example{
		{ID: "default", Title: "域名输入", Description: "前缀文本和清除按钮由生产 InputGroup 绘制。", Snippet: `components.PaintInputGroup(canvas, theme, bounds, components.InputGroupSnapshot{
    ID: tree.ID(page, "domain"), Label: "Project domain", Value: value,
    Placeholder: "project-name",
    InlineStart: components.InputGroupSlot{
        Kind: components.InputGroupSlotText, Text: "https://", Separated: true,
    },
    InlineEnd: components.InputGroupSlot{
        Kind: components.InputGroupSlotButton, ID: tree.ID(page, "clear"),
        Icon: clearIcon, ButtonSize: components.InputGroupButtonIconXS,
        Label: "Clear domain",
    },
})`},
		{ID: "addons", Title: "块级附加项", Description: "块级说明、键帽和多行控制槽。", Snippet: `components.PaintInputGroup(canvas, theme, bounds, components.InputGroupSnapshot{
    ID: tree.ID(page, "description"), Label: "Release description",
    Value: value, Multiline: true,
    BlockStart: components.InputGroupSlot{
        Kind: components.InputGroupSlotText, Text: "Release description",
        Separated: true,
    },
    BlockEnd: components.InputGroupSlot{
        Kind: components.InputGroupSlotKbd, Text: "⌘↵",
    },
})`},
		{ID: "buttons", Title: "图标按钮与加载", Description: "icon-xs / icon-sm 按钮切换密码可见，以及 Spinner 附加槽。", Snippet: `reveal := tree.ID(page, "reveal")
components.PaintInputGroup(canvas, theme, bounds, components.InputGroupSnapshot{
    ID: tree.ID(page, "password"), Label: "Password", Value: value,
    Secure: !revealed,
    InlineEnd: components.InputGroupSlot{
        Kind: components.InputGroupSlotButton, ID: reveal, Icon: eyeIcon,
        ButtonSize: components.InputGroupButtonIconSM, Label: "Show password",
    },
})
if canvas.Clicked(reveal) {
    revealed = !revealed
}`},
		{ID: "states", Title: "输入状态", Description: "无效与禁用快照保留完整语义。", Snippet: `invalid := components.InputGroupSnapshot{
    ID: tree.ID(page, "email"), Label: "Project domain", Value: value,
    Invalid: true,
    InlineStart: components.InputGroupSlot{
        Kind: components.InputGroupSlotText, Text: "@", Separated: true,
    },
    InlineEnd: components.InputGroupSlot{
        Kind: components.InputGroupSlotButton, ID: tree.ID(page, "invite"),
        Text: "Invite", Disabled: true,
    },
}
components.PaintInputGroup(canvas, theme, invalidBounds, invalid)
locked := components.InputGroupSnapshot{
    ID: tree.ID(page, "locked"), Label: "Project domain",
    Value: "locked", Disabled: true,
}
components.PaintInputGroup(canvas, theme, lockedBounds, locked)`},
	},
}

var (
	inputGroupUI    = tree.Feature("input-group")
	inputGroupField = tree.ID(inputGroupUI, "field")
	inputGroupClear = tree.ID(inputGroupUI, "clear")
	// The states example's inert specimens.
	inputGroupInvite = tree.ID(inputGroupUI, "invite")
	inputGroupLocked = tree.ID(inputGroupUI, "locked")
	// The buttons example's reveal button and its second, searching group.
	inputGroupReveal   = tree.ID(inputGroupUI, "reveal")
	inputGroupSearchID = tree.ID(inputGroupUI, "search-field")
)

// inputGroupCrossIcon is Lucide's x, the clear button's icon.
var inputGroupCrossIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(18, 6), gfx.LineTo(6, 18), gfx.MoveTo(6, 6), gfx.LineTo(18, 18),
}}

// inputGroupEyeIcon is a stand-in for a reveal button's icon: a lens shape.
var inputGroupEyeIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(2, 12), gfx.LineTo(7, 7), gfx.LineTo(17, 7), gfx.LineTo(22, 12), gfx.LineTo(17, 17), gfx.LineTo(7, 17), gfx.LineTo(2, 12),
}}

func inputGroupInitial(exampleID string) string {
	if exampleID == "addons" {
		return "Ship the component gallery.\nAdd the keyboard shortcuts."
	}
	return "viva.dev"
}

func (state *State) initializeInputGroup() { state.flags |= 1 }

// clearInputGroup is the clear button's action. Without editing it only toggles
// the snapshot's text; with it, it empties the editor and fills it back in when
// it is already empty.
func (state *State) clearInputGroup() {
	if !state.editing {
		state.flags ^= 1
		return
	}
	value := ""
	if state.editor.RuneCount() == 0 {
		value = inputGroupInitial(state.exampleID)
	}
	state.editor.SetSelection(text.Range{Start: 0, End: state.editor.RuneCount()})
	state.editor.Insert(value)
}

func (state *State) paintInputGroup(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	height := theme.Metrics.ControlHeight
	if state.exampleID == "addons" {
		// A 32px block add-on above and below, and a 64px multiline control.
		height = theme.Metrics.ControlHeight * 4
	}
	var group, locked, searching ui.Box
	state.page(canvas, theme, size, 560, state.Text("Input Group · 附加槽", "Input Group · Add-on slots"), func(f *ui.Frame) {
		group = f.Slot().FillWidth().Height(height)
		if state.exampleID == "states" {
			locked = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
		}
		if state.exampleID == "buttons" {
			searching = f.Slot().FillWidth().Height(theme.Metrics.ControlHeight)
		}
	})
	value := state.fieldValue("")
	if !state.editing && state.flags&1 != 0 {
		value = inputGroupInitial(state.exampleID)
	}
	snapshot := components.InputGroupSnapshot{ID: inputGroupField, Label: "Project domain", Value: value, Placeholder: "project-name",
		ReadOnly: !state.editing, HideEditorText: true, InlineStart: components.InputGroupSlot{Kind: components.InputGroupSlotText, Text: "https://", Separated: true}, InlineEnd: components.InputGroupSlot{Kind: components.InputGroupSlotButton, ID: inputGroupClear, Icon: inputGroupCrossIcon, ButtonSize: components.InputGroupButtonIconXS, Label: "Clear domain"}}
	if state.editing && state.editor != nil && canvas.Focused() == inputGroupField {
		selection := state.editor.Selection()
		snapshot.SelectionStart, snapshot.SelectionEnd = byteOffsetOfRune(value, selection.Start), byteOffsetOfRune(value, selection.End)
		snapshot.Caret = byteOffsetOfRune(value, state.editor.Caret())
		snapshot.ShowCaret = selection.Start == selection.End
		snapshot.Blink = &state.blink
		if marked, composing := state.editor.Composition(); composing {
			snapshot.CompositionStart, snapshot.CompositionEnd = byteOffsetOfRune(value, marked.Start), byteOffsetOfRune(value, marked.End)
		}
	}
	if state.exampleID == "addons" {
		snapshot.Multiline = true
		// The description is not a domain: no "https://" in front of it.
		snapshot.InlineStart = components.InputGroupSlot{}
		snapshot.BlockStart = components.InputGroupSlot{Kind: components.InputGroupSlotText, Text: "Release description", Separated: true}
		snapshot.BlockEnd = components.InputGroupSlot{Kind: components.InputGroupSlotKbd, Text: "⌘↵"}
	}
	if state.exampleID == "buttons" {
		// A secret with a reveal button, and a field that is searching.
		snapshot.Secure = state.flags&2 == 0
		snapshot.InlineStart = components.InputGroupSlot{}
		snapshot.InlineEnd = components.InputGroupSlot{Kind: components.InputGroupSlotButton, ID: inputGroupReveal, Icon: inputGroupEyeIcon, ButtonSize: components.InputGroupButtonIconSM, Label: "Show password"}
	}
	if state.exampleID == "states" {
		snapshot.Invalid = true
		snapshot.InlineStart = components.InputGroupSlot{Kind: components.InputGroupSlotText, Text: "@", Separated: true}
		snapshot.InlineEnd = components.InputGroupSlot{Kind: components.InputGroupSlotButton, ID: inputGroupInvite, Text: "Invite", Disabled: true}
	}
	components.PaintInputGroup(canvas, theme, group.Rect(), snapshot)
	if canvas.Clicked(inputGroupClear) {
		state.clearInputGroup()
	}
	if canvas.Clicked(inputGroupReveal) {
		state.flags ^= 2
	}
	if state.exampleID == "buttons" {
		components.PaintInputGroup(canvas, theme, searching.Rect(), components.InputGroupSnapshot{
			ID: inputGroupSearchID, Label: "Search", Value: "viva", ReadOnly: true,
			InlineStart: components.InputGroupSlot{Kind: components.InputGroupSlotText, Text: "Search"},
			InlineEnd:   components.InputGroupSlot{Kind: components.InputGroupSlotSpinner, Phase: components.SpinnerPhase(state.values[1] & 7)},
		})
	}
	if state.exampleID == "states" {
		disabled := snapshot
		disabled.ID = inputGroupLocked
		disabled.Invalid = false
		disabled.Disabled = true
		disabled.Value = "locked"
		disabled.InlineEnd.ID = gfx.ID{}
		components.PaintInputGroup(canvas, theme, locked.Rect(), disabled)
	}
}
