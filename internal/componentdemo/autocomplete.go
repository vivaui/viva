package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/autocomplete"
)

var autocompleteComponent = Component{
	ID: "autocomplete", Title: "Autocomplete", Category: "Forms", Summary: "自动补全：输入时提示包含所输文字的候选，值仍是自由文本。",
	Source: "ui/autocomplete/autocomplete.go", paint: (*State).paintAutocomplete, initialize: (*State).initializeAutocomplete,
	editable: func(string) (editableField, bool) {
		return editableField{ID: autocompleteField, Initial: ""}, true
	},
	Examples: []Example{
		{ID: "default", Title: "候选与自由输入", Description: "上下键移动，回车或 Tab 选中，Esc 关闭；没有高亮候选时回车留给页面，输入候选之外的文字也会保留。与 Combobox 不同，值不限于候选。", Snippet: `m.host = autocomplete.New()
m.host.View(c, &m.hostField, "Host", "Hostname", autocomplete.Spec{Options: m.known})
if option, ok := m.host.Chosen(); ok {
    // the field already holds option
}`, PageSnippet: `m.host = autocomplete.New()
m.host.View(c, &m.hostField, "Host", "Hostname", autocomplete.Spec{Options: m.known})
if option, ok := m.host.Chosen(); ok {
    // the field already holds option
}`, PageSnippetZh: `m.host = autocomplete.New()
m.host.View(c, &m.hostField, "Host", "Hostname", autocomplete.Spec{Options: m.known})
if option, ok := m.host.Chosen(); ok {
    // 输入框里已经是 option
}`},
	},
}

var (
	autocompleteUI    = tree.Feature("autocomplete")
	autocompleteField = tree.ID(autocompleteUI, "field")
)

var autocompleteOptions = []string{"build-01", "build-02", "db-primary", "db-replica", "staging", "bastion"}

type autocompleteDemo struct {
	a     *autocomplete.Autocomplete
	field ui.Field
	bound bool
}

var autocompleteDemos = map[*State]*autocompleteDemo{}

func autocompleteDemoFor(state *State) *autocompleteDemo {
	if demo := autocompleteDemos[state]; demo != nil {
		return demo
	}
	demo := &autocompleteDemo{a: autocomplete.New().FieldID(autocompleteField)}
	autocompleteDemos[state] = demo
	return demo
}

func (state *State) initializeAutocomplete() { autocompleteDemoFor(state) }

func (state *State) paintAutocomplete(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := autocompleteDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		demo.bound = true
		demo.field.Bind(state.editor)
	}
	state.drawPage(canvas, theme, size, 360, state.Text("Autocomplete · 自动补全", "Autocomplete"), func(c ui.C) {
		demo.a.View(c, &demo.field, state.Text("主机", "Host"), state.Text("主机名", "Hostname"), autocomplete.Spec{Options: autocompleteOptions}).FillWidth()
	})
}
