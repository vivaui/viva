package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputgroup"
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
})`, PageSnippet: `// m.domain starts as "viva.dev".
group := inputgroup.New(&m.domain).
    Placeholder("project-name").
    Prefix("https://").
    Action(m.clearID, m.crossIcon, "Clear domain", m.clearDomain)
group.View(c, "Project domain").FillWidth()`, PageSnippetZh: `// m.domain 初始为 "viva.dev"。
group := inputgroup.New(&m.domain).
    Placeholder("project-name").
    Prefix("https://").
    Action(m.clearID, m.crossIcon, "清除域名", m.clearDomain)
group.View(c, "Project domain").FillWidth()`},
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
})`, PageSnippet: `// m.description starts as "Ship the component gallery.\nAdd the keyboard shortcuts."
group := inputgroup.New(&m.description).
    Multiline().
    BlockStart("Release description").
    BlockKbd("⌘↵")
group.View(c, "Release description").FillWidth()`, PageSnippetZh: `// m.description 初始为 "发布组件画廊。\n补充键盘快捷键。"。
group := inputgroup.New(&m.description).
    Multiline().
    BlockStart("发布说明").
    BlockKbd("⌘↵")
group.View(c, "Release description").FillWidth()`},
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
}`, PageSnippet: `// m.password starts as "viva.dev"; it is secure until m.reveal runs.
m.password.Secure = m.passwordHidden
password := inputgroup.New(&m.password).
    Action(m.revealID, m.eyeIcon, "Show password", m.reveal).
    Small()
password.View(c, "Password").FillWidth()
// m.search starts as "viva".
search := inputgroup.New(&m.search).
    Prefix("Search").
    Spinner(m.spinnerPhase)
search.View(c, "Search").FillWidth()`, PageSnippetZh: `// m.password 初始为 "viva.dev"；m.reveal 运行前保持密文。
m.password.Secure = m.passwordHidden
password := inputgroup.New(&m.password).
    Action(m.revealID, m.eyeIcon, "显示密码", m.reveal).
    Small()
password.View(c, "Password").FillWidth()
// m.search 初始为 "viva"。
search := inputgroup.New(&m.search).
    Prefix("搜索").
    Spinner(m.spinnerPhase)
search.View(c, "Search").FillWidth()`},
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
components.PaintInputGroup(canvas, theme, lockedBounds, locked)`, PageSnippet: `// m.email starts as "viva.dev"; m.locked starts as "locked".
m.email.Invalid = true
email := inputgroup.New(&m.email).
    Prefix("@").
    Button(m.inviteID, "Invite", m.invite).
    DisableAction()
email.View(c, "Project domain").FillWidth()
m.locked.Disabled = true
locked := inputgroup.New(&m.locked)
locked.View(c, "Project domain").FillWidth()`, PageSnippetZh: `// m.email 初始为 "viva.dev"；m.locked 初始为 "locked"。
m.email.Invalid = true
email := inputgroup.New(&m.email).
    Prefix("@").
    Button(m.inviteID, "Invite", m.invite).
    DisableAction()
email.View(c, "Project domain").FillWidth()
m.locked.Disabled = true
locked := inputgroup.New(&m.locked)
locked.View(c, "Project domain").FillWidth()`},
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

// inputGroupDemo is one example's page controls, fields and stable callbacks.
// They are kept beside the State rather than rebuilt in each paint, so the
// handlers and draw functions are stable and a warm paint allocates nothing.
type inputGroupDemo struct {
	group  *inputgroup.Group
	search *inputgroup.Group
	locked *inputgroup.Group

	field       ui.Field
	searchField ui.Field
	lockedField ui.Field

	clear  func()
	reveal func()
	bound  bool
}

// inputGroupDemos is keyed by State because the constrained demo model has no
// field for these controls; a State is painted on one thread at a time.
var inputGroupDemos = map[*State]*inputGroupDemo{}

func (state *State) initializeInputGroup() {
	state.flags |= 1
	demo := &inputGroupDemo{}
	demo.clear = func() { state.clearInputGroup() }
	demo.reveal = func() { state.flags ^= 2 }
	demo.field.SetText(inputGroupInitial(state.exampleID))
	switch state.exampleID {
	case "addons":
		demo.group = inputgroup.New(&demo.field).
			FieldID(inputGroupField).
			Multiline().
			BlockStart("Release description").
			BlockKbd("⌘↵")
	case "buttons":
		demo.group = inputgroup.New(&demo.field).
			FieldID(inputGroupField).
			Action(inputGroupReveal, inputGroupEyeIcon, "Show password", demo.reveal).
			Small()
		demo.searchField.SetText("viva")
		demo.search = inputgroup.New(&demo.searchField).
			FieldID(inputGroupSearchID).
			Prefix("Search").
			Spinner(0)
	case "states":
		demo.group = inputgroup.New(&demo.field).
			FieldID(inputGroupField).
			Prefix("@").
			Button(inputGroupInvite, "Invite", nil).
			DisableAction()
		demo.lockedField.SetText("locked")
		demo.lockedField.Disabled = true
		demo.locked = inputgroup.New(&demo.lockedField).
			FieldID(inputGroupLocked)
	default:
		demo.group = inputgroup.New(&demo.field).
			FieldID(inputGroupField).
			Placeholder("project-name").
			Prefix("https://").
			Action(inputGroupClear, inputGroupCrossIcon, "Clear domain", demo.clear)
	}
	inputGroupDemos[state] = demo
}

func inputGroupDemoFor(state *State) *inputGroupDemo {
	if demo := inputGroupDemos[state]; demo != nil {
		return demo
	}
	demo := &inputGroupDemo{}
	demo.clear = func() { state.clearInputGroup() }
	demo.reveal = func() { state.flags ^= 2 }
	demo.field.SetText(inputGroupInitial(state.exampleID))
	inputGroupDemos[state] = demo
	return demo
}

// clearInputGroup is the clear button's action. Without editing it toggles the
// field's text; with editing it empties the editor and fills it back in when it
// is already empty.
func (state *State) clearInputGroup() {
	if !state.editing {
		state.flags ^= 1
		demo := inputGroupDemoFor(state)
		if state.flags&1 != 0 {
			demo.field.SetText(inputGroupInitial(state.exampleID))
		} else {
			demo.field.SetText("")
		}
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
	demo := inputGroupDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		demo.field.Bind(state.editor)
		demo.bound = true
	}
	if state.editing {
		demo.group.Editable()
	} else {
		demo.group.ReadOnly()
	}
	if state.exampleID == "buttons" {
		demo.field.Secure = state.flags&2 == 0
		demo.search.ReadOnly()
		demo.search.Spinner(components.SpinnerPhase(state.values[1] & 7))
	}
	if state.exampleID == "states" {
		demo.field.Invalid = true
	}
	state.drawPage(canvas, theme, size, 560, state.Text("Input Group · 附加槽", "Input Group · Add-on slots"), func(c ui.C) {
		switch state.exampleID {
		case "addons":
			demo.group.View(c, "Release description").FillWidth()
		case "buttons":
			demo.group.View(c, "Password").FillWidth()
			demo.search.View(c, "Search").FillWidth()
		case "states":
			ui.Column(c).Gap(theme.Metrics.GapItem).Children(func() {
				demo.group.View(c, "Project domain").FillWidth()
				demo.locked.View(c, "Project domain").FillWidth()
			})
		default:
			demo.group.View(c, "Project domain").FillWidth()
		}
	})
}
