package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/inputnumber"
)

var inputNumberComponent = Component{
	ID: "input-number", Title: "Input Number", Category: "Forms", Summary: "数字输入：两侧按钮与方向键步进，范围与小数位可配。",
	Source: "ui/inputnumber/inputnumber.go", paint: (*State).paintInputNumber, initialize: (*State).initializeInputNumber,
	editable: func(exampleID string) (editableField, bool) {
		return editableField{ID: inputNumberField, Initial: inputNumberInitial(exampleID)}, true
	},
	Examples: []Example{
		{ID: "default", Title: "带范围的数字", Description: "输入数字后回车或失焦生效，超出范围会被夹回；上下键步进，PageUp/PageDown 十倍。", Snippet: `m.port = inputnumber.New().Min(1).Max(65535)
// in the page:
m.port.View(c, "Port", &m.portValue).Width(180)
if m.port.Changed() {
    // m.portValue holds the new number
}`, PageSnippet: `m.port = inputnumber.New().Min(1).Max(65535)
// in the page:
m.port.View(c, "Port", &m.portValue).Width(180)
if m.port.Changed() {
    // m.portValue holds the new number
}`, PageSnippetZh: `m.port = inputnumber.New().Min(1).Max(65535)
// 在页面中：
m.port.View(c, "Port", &m.portValue).Width(180)
if m.port.Changed() {
    // m.portValue 已经是新的数字
}`},
		{ID: "decimals", Title: "小数与只读", Description: "小数位跟随步长；ReadOnly 只展示数字。", Snippet: `m.opacity = inputnumber.New().Step(0.1).Min(0).Max(1)
m.total = inputnumber.New().ReadOnly()`, PageSnippet: `m.opacity = inputnumber.New().Step(0.1).Min(0).Max(1)
m.total = inputnumber.New().ReadOnly()`, PageSnippetZh: `m.opacity = inputnumber.New().Step(0.1).Min(0).Max(1)
m.total = inputnumber.New().ReadOnly()`},
	},
}

var (
	inputNumberUI    = tree.Feature("input-number")
	inputNumberField = tree.ID(inputNumberUI, "field")
)

// inputNumberInitial is the text of the example's editable field.
func inputNumberInitial(exampleID string) string {
	if exampleID == "decimals" {
		return "0.5"
	}
	return "22"
}

type inputNumberDemo struct {
	bound                bool
	port, opacity, total *inputnumber.InputNumber
	a, b, c              float64
}

var inputNumberDemos = map[*State]*inputNumberDemo{}

func inputNumberDemoFor(state *State) *inputNumberDemo {
	if demo := inputNumberDemos[state]; demo != nil {
		return demo
	}
	demo := &inputNumberDemo{
		port: inputnumber.New().Min(1).Max(65535).FieldID(inputNumberField), opacity: inputnumber.New().Step(0.1).Min(0).Max(1).FieldID(inputNumberField), total: inputnumber.New().ReadOnly(),
		a: 22, b: 0.5, c: 128,
	}
	inputNumberDemos[state] = demo
	return demo
}

func (state *State) initializeInputNumber() { inputNumberDemoFor(state) }

func (state *State) paintInputNumber(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := inputNumberDemoFor(state)
	if state.editing && state.editor != nil && !demo.bound {
		// The platform edits the example's one editable field; the number field reads
		// what is typed from the same editor.
		demo.bound = true
		demo.port.Field().Bind(state.editor)
		demo.opacity.Field().Bind(state.editor)
	}
	state.drawPage(canvas, theme, size, 320, state.Text("Input Number · 数字输入", "Input Number"), func(c ui.C) {
		if state.exampleID == "decimals" {
			demo.opacity.View(c, state.Text("不透明度", "Opacity"), &demo.b).Width(200)
			demo.total.View(c, state.Text("总数", "Total"), &demo.c).Width(200)
			return
		}
		demo.port.View(c, state.Text("端口", "Port"), &demo.a).Width(200)
	})
}
