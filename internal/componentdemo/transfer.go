package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/transfer"
)

var transferComponent = Component{
	ID: "transfer", Title: "Transfer", Category: "Forms", Summary: "穿梭框：在两个列表之间移动条目，支持全选与禁用项。",
	Source: "ui/transfer/transfer.go", paint: (*State).paintTransfer,
	Examples: []Example{
		{ID: "default", Title: "在两个列表间移动", Description: "勾选条目后点中间的按钮移动；列表头的勾选框全选或清空；禁用项不会被选中或移动。", Snippet: `m.moves = transfer.New()
// m.hosts is a []transfer.Item kept in the model; Target is the right list.
m.moves.View(c, "Hosts", transfer.Spec{
    Items:  m.hosts,
    Titles: [2]string{"Available", "Selected"},
}).FillWidth()
if m.moves.Changed() {
    // items moved
}`, PageSnippet: `m.moves = transfer.New()
// m.hosts is a []transfer.Item kept in the model; Target is the right list.
m.moves.View(c, "Hosts", transfer.Spec{
    Items:  m.hosts,
    Titles: [2]string{"Available", "Selected"},
}).FillWidth()
if m.moves.Changed() {
    // items moved
}`, PageSnippetZh: `m.moves = transfer.New()
// m.hosts 是保存在模型里的 []transfer.Item；Target 为 true 表示在右侧列表。
m.moves.View(c, "Hosts", transfer.Spec{
    Items:  m.hosts,
    Titles: [2]string{"Available", "Selected"},
}).FillWidth()
if m.moves.Changed() {
    // 条目发生了移动
}`},
	},
}

type transferDemo struct {
	moves *transfer.Transfer
	items []transfer.Item
}

var transferDemos = map[*State]*transferDemo{}

func transferDemoFor(state *State) *transferDemo {
	if demo := transferDemos[state]; demo != nil {
		return demo
	}
	demo := &transferDemo{moves: transfer.New(), items: []transfer.Item{
		{Title: "build-01"}, {Title: "build-02", Checked: true}, {Title: "db-primary", Target: true},
		{Title: "db-replica", Target: true, Checked: true}, {Title: "staging", Disabled: true}, {Title: "bastion"},
		{Title: "monitor"}, {Title: "cache-01"}, {Title: "cache-02"}, {Title: "queue"},
	}}
	transferDemos[state] = demo
	return demo
}

func (state *State) paintTransfer(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := transferDemoFor(state)
	state.drawPage(canvas, theme, size, 640, state.Text("Transfer · 穿梭框", "Transfer"), func(c ui.C) {
		demo.moves.View(c, state.Text("主机", "Hosts"), transfer.Spec{
			Items:      demo.items,
			Titles:     [2]string{state.Text("可选", "Available"), state.Text("已选", "Selected")},
			Height:     200,
			MoveLabels: [2]string{state.Text("移到右侧", "Move right"), state.Text("移到左侧", "Move left")},
		}).FillWidth()
	})
}
