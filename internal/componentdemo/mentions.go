package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/mentions"
)

var mentionsComponent = Component{
	ID: "mentions", Title: "Mentions", Category: "Forms", Summary: "提及：输入 @ 后弹出候选列表，选中后写入输入框。",
	Source: "ui/mentions/mentions.go", paint: (*State).paintMentions, initialize: (*State).initializeMentions,
	editable: func(string) (editableField, bool) {
		return editableField{ID: mentionsField, Initial: ""}, true
	},
	Examples: []Example{
		{ID: "default", Title: "输入 @ 提及", Description: "在词首输入 @ 与几个字母，列表按包含关系过滤；上下键移动，回车或 Tab 选中，Esc 关闭，点选项不会夺走输入框的键盘。", Snippet: `m.mentions = mentions.New()
m.mentions.View(c, &m.message, "Message", "Write a message", mentions.Spec{Options: m.people})
if option, ok := m.mentions.Chosen(); ok {
    // the field already holds "@Ada "
}`, PageSnippet: `m.mentions = mentions.New()
m.mentions.View(c, &m.message, "Message", "Write a message", mentions.Spec{Options: m.people})
if option, ok := m.mentions.Chosen(); ok {
    // the field already holds "@Ada "
}`, PageSnippetZh: `m.mentions = mentions.New()
m.mentions.View(c, &m.message, "Message", "Write a message", mentions.Spec{Options: m.people})
if option, ok := m.mentions.Chosen(); ok {
    // 输入框里已经是 "@Ada "
}`},
	},
}

var (
	mentionsUI    = tree.Feature("mentions")
	mentionsField = tree.ID(mentionsUI, "field")
)

type mentionsDemo struct {
	bound   bool
	m       *mentions.Mentions
	field   ui.Field
	options []mentions.Option
}

var mentionsDemos = map[*State]*mentionsDemo{}

func mentionsDemoFor(state *State) *mentionsDemo {
	if demo := mentionsDemos[state]; demo != nil {
		return demo
	}
	demo := &mentionsDemo{m: mentions.New().FieldID(mentionsField), options: []mentions.Option{
		{Label: "Ada", Detail: "Engineer"}, {Label: "Grace", Detail: "Admiral"}, {Label: "Alan", Detail: "Mathematician"},
		{Label: "Linus", Detail: "Maintainer"}, {Label: "Margaret", Detail: "Director"},
	}}
	mentionsDemos[state] = demo
	return demo
}

func (state *State) initializeMentions() { mentionsDemoFor(state) }

func (state *State) paintMentions(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := mentionsDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		// The platform edits the example's one editable field; the mentions read what
		// is typed from the same editor.
		demo.bound = true
		demo.field.Bind(state.editor)
	}
	state.drawPage(canvas, theme, size, 420, state.Text("Mentions · 提及", "Mentions"), func(c ui.C) {
		demo.m.View(c, &demo.field, state.Text("消息", "Message"), state.Text("输入 @ 提及某人", "Type @ to mention someone"), mentions.Spec{
			Options: demo.options, Empty: state.Text("没有匹配的人", "No one matches"),
		}).FillWidth()
	})
}
