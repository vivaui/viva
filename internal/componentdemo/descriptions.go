package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/descriptions"
)

var descriptionsComponent = Component{
	ID: "descriptions", Title: "Descriptions", Category: "Data Display", Summary: "带标签的键值列表：按列排布，可带边框、纵向或紧凑。",
	Source: "ui/descriptions/descriptions.go", paint: (*State).paintDescriptions,
	Examples: []Example{
		{ID: "default", Title: "详情列表", Description: "标签在前、值在后，每行按列数排布，跨列的条目占多列。", Snippet: `// m.details is a []descriptions.Item kept in the model.
descriptions.View(c, "Host details", descriptions.Spec{
    Title:      "Host details",
    Items:      m.details,
    LabelWidth: 72,
}).FillWidth()`, PageSnippet: `// m.details is a []descriptions.Item kept in the model.
descriptions.View(c, "Host details", descriptions.Spec{
    Title:      "Host details",
    Items:      m.details,
    LabelWidth: 72,
}).FillWidth()`, PageSnippetZh: `// m.details 是保存在模型里的 []descriptions.Item。
descriptions.View(c, "Host details", descriptions.Spec{
    Title:      "Host details",
    Items:      m.details,
    LabelWidth: 72,
}).FillWidth()`},
		{ID: "bordered", Title: "带边框", Description: "表格样式，标签单元着色；Vertical 把标签放在值上方。", Snippet: `descriptions.View(c, "Host details", descriptions.Spec{
    Items:      m.details,
    Columns:    2,
    Bordered:   true,
    LabelWidth: 96,
}).FillWidth()`, PageSnippet: `descriptions.View(c, "Host details", descriptions.Spec{
    Items:      m.details,
    Columns:    2,
    Bordered:   true,
    LabelWidth: 96,
}).FillWidth()`, PageSnippetZh: `descriptions.View(c, "Host details", descriptions.Spec{
    Items:      m.details,
    Columns:    2,
    Bordered:   true,
    LabelWidth: 96,
}).FillWidth()`},
	},
}

type descriptionsDemo struct {
	items []descriptions.Item
}

var descriptionsDemos = map[*State]*descriptionsDemo{}

func descriptionsDemoFor(state *State) *descriptionsDemo {
	if demo := descriptionsDemos[state]; demo != nil {
		return demo
	}
	demo := &descriptionsDemo{items: []descriptions.Item{
		{Label: state.Text("主机", "Host"), Value: "build-01.example.com"},
		{Label: state.Text("端口", "Port"), Value: "22"},
		{Label: state.Text("用户", "User"), Value: "deploy"},
		{Label: state.Text("密钥", "Key"), Value: "ed25519 · SHA256:4kQ…9aZ", Span: 2},
		{Label: state.Text("跳板机", "Jump host"), Value: "bastion.example.com"},
		{Label: state.Text("备注", "Notes"), Value: state.Text("夜间构建机，02:00 窗口内请勿重启。", "Nightly build agent. Do not reboot during the 02:00 window."), Span: 3},
	}}
	descriptionsDemos[state] = demo
	return demo
}

func (state *State) paintDescriptions(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := descriptionsDemoFor(state)
	state.drawPage(canvas, theme, size, 680, state.Text("Descriptions · 详情", "Descriptions · Details"), func(c ui.C) {
		title := state.Text("主机详情", "Host details")
		if state.exampleID == "bordered" {
			descriptions.View(c, title, descriptions.Spec{Items: demo.items, Columns: 2, Bordered: true, LabelWidth: 96}).FillWidth()
			descriptions.View(c, title, descriptions.Spec{Items: demo.items, Vertical: true, Size: descriptions.Small}).FillWidth()
			return
		}
		descriptions.View(c, title, descriptions.Spec{Title: title, Items: demo.items, LabelWidth: 72}).FillWidth()
	})
}
