package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/overflowlist"
)

var overflowListComponent = Component{
	ID: "overflow-list", Title: "Overflow List", Category: "Navigation", Summary: "折叠列表：工具栏放不下时，末尾的操作自动收进「更多」菜单。",
	Source: "ui/overflowlist/overflowlist.go", paint: (*State).paintOverflowList,
	Examples: []Example{
		{ID: "default", Title: "随宽度折叠", Description: "同一组操作放在三种宽度里：放得下的留在行内，放不下的从末尾起进「更多」菜单，禁用状态保持；Min 保证至少留几个。", Snippet: `m.tools = overflowlist.New()
m.tools.View(c, "Actions", overflowlist.Spec{Items: m.actions}).FillWidth()
if i, ok := m.tools.Clicked(); ok {
    m.run(m.actions[i])
}`, PageSnippet: `m.tools = overflowlist.New()
m.tools.View(c, "Actions", overflowlist.Spec{Items: m.actions}).FillWidth()
if i, ok := m.tools.Clicked(); ok {
    m.run(m.actions[i])
}`, PageSnippetZh: `m.tools = overflowlist.New()
m.tools.View(c, "Actions", overflowlist.Spec{Items: m.actions}).FillWidth()
if i, ok := m.tools.Clicked(); ok {
    m.run(m.actions[i])
}`},
	},
}

type overflowListDemo struct {
	rows  [3]*overflowlist.OverflowList
	items []overflowlist.Item
}

var overflowListDemos = map[*State]*overflowListDemo{}

func overflowListDemoFor(state *State) *overflowListDemo {
	if demo := overflowListDemos[state]; demo != nil {
		return demo
	}
	icon := gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
		gfx.MoveTo(12, 5), gfx.LineTo(12, 19), gfx.MoveTo(5, 12), gfx.LineTo(19, 12),
	}}
	demo := &overflowListDemo{rows: [3]*overflowlist.OverflowList{overflowlist.New(), overflowlist.New(), overflowlist.New()}}
	for _, name := range []string{"Upload", "New file", "New folder", "Refresh", "Follow", "Download", "Delete"} {
		demo.items = append(demo.items, overflowlist.Item{Label: name, Icon: icon, Disabled: name == "Delete"})
	}
	overflowListDemos[state] = demo
	return demo
}

func (state *State) paintOverflowList(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := overflowListDemoFor(state)
	state.drawPage(canvas, theme, size, 440, state.Text("Overflow List · 折叠列表", "Overflow List"), func(c ui.C) {
		ui.Column(c).Gap(14).Children(func() {
			for i, width := range [3]float32{340, 220, 130} {
				demo.rows[i].View(c.KeyInt(i), state.Text("操作", "Actions"), overflowlist.Spec{Items: demo.items, MoreLabel: state.Text("更多", "More")}).Width(width)
			}
		})
	})
}
