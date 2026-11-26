package componentdemo

import (
	"slices"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputtag"
)

var inputTagComponent = Component{
	ID: "input-tag", Title: "Input Tag", Category: "Forms", Summary: "标签输入：回车或逗号添加标签，空输入框按退格删除最后一个。",
	Source: "ui/inputtag/inputtag.go", paint: (*State).paintInputTag, initialize: (*State).initializeInputTag,
	editable: func(string) (editableField, bool) {
		return editableField{ID: inputTagField, Initial: ""}, true
	},
	Examples: []Example{
		{ID: "default", Title: "输入多个值", Description: "控件只报告添加与删除，标签列表由页面持有；重复、空白与超出上限的值被拒绝并让输入框显示无效外观。", Snippet: `m.hosts = inputtag.New()
m.hosts.View(c, "Allowed hosts", m.allowed, inputtag.Spec{Placeholder: "Add a host"}).FillWidth()
if value, ok := m.hosts.Added(); ok {
    m.allowed = append(m.allowed, value)
}
if i, ok := m.hosts.Removed(); ok {
    m.allowed = slices.Delete(m.allowed, i, i+1)
}`, PageSnippet: `m.hosts = inputtag.New()
m.hosts.View(c, "Allowed hosts", m.allowed, inputtag.Spec{Placeholder: "Add a host"}).FillWidth()
if value, ok := m.hosts.Added(); ok {
    m.allowed = append(m.allowed, value)
}
if i, ok := m.hosts.Removed(); ok {
    m.allowed = slices.Delete(m.allowed, i, i+1)
}`, PageSnippetZh: `m.hosts = inputtag.New()
m.hosts.View(c, "Allowed hosts", m.allowed, inputtag.Spec{Placeholder: "Add a host"}).FillWidth()
if value, ok := m.hosts.Added(); ok {
    m.allowed = append(m.allowed, value)
}
if i, ok := m.hosts.Removed(); ok {
    m.allowed = slices.Delete(m.allowed, i, i+1)
}`},
	},
}

var (
	inputTagUI    = tree.Feature("input-tag")
	inputTagField = tree.ID(inputTagUI, "field")
)

type inputTagDemo struct {
	t     *inputtag.InputTag
	tags  []string
	bound bool
}

var inputTagDemos = map[*State]*inputTagDemo{}

func inputTagDemoFor(state *State) *inputTagDemo {
	if demo := inputTagDemos[state]; demo != nil {
		return demo
	}
	demo := &inputTagDemo{t: inputtag.New().FieldID(inputTagField), tags: []string{"build-01", "db-primary"}}
	inputTagDemos[state] = demo
	return demo
}

func (state *State) initializeInputTag() { inputTagDemoFor(state) }

func (state *State) paintInputTag(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := inputTagDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		demo.bound = true
		demo.t.Field().Bind(state.editor)
	}
	state.drawPage(canvas, theme, size, 440, state.Text("Input Tag · 标签输入", "Input Tag"), func(c ui.C) {
		demo.t.View(c, state.Text("允许的主机", "Allowed hosts"), demo.tags, inputtag.Spec{
			Placeholder: state.Text("添加主机", "Add a host"), MaxTags: 6, RemoveLabel: state.Text("移除", "Remove"),
		}).FillWidth()
		if value, ok := demo.t.Added(); ok {
			demo.tags = append(demo.tags, value)
		}
		if i, ok := demo.t.Removed(); ok {
			demo.tags = slices.Delete(demo.tags, i, i+1)
		}
	})
}
