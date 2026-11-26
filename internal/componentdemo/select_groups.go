package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var selectGroupsComponent = Component{
	ID: "select-groups", Title: "Select Groups", Category: "Forms", Summary: "下拉列表里的分组标题行。",
	Source: "base/select.go", paint: (*State).paintSelectGroups,
	Examples: []Example{
		{ID: "default", Title: "分组选项", Description: "SelectGroups 在每组第一个选项前加一行标题；标题不是选项，不能聚焦、点击或类型检索，Picked、DisabledOptions 和 Icons 仍用选项自己的下标。选项须按组排序。", Snippet: `// m.groups is a []ui.SelectGroup kept in the model:
// {Label: "Recent", From: 0}, {Label: "All hosts", From: 2}
ui.Select(c, &m.host, m.hosts, nil, ui.SelectGroups(m.groups))`, PageSnippet: `// m.groups is a []ui.SelectGroup kept in the model:
// {Label: "Recent", From: 0}, {Label: "All hosts", From: 2}
ui.Select(c, &m.host, m.hosts, nil, ui.SelectGroups(m.groups))`, PageSnippetZh: `// m.groups 是保存在模型里的 []ui.SelectGroup：
// {Label: "Recent", From: 0}, {Label: "All hosts", From: 2}
ui.Select(c, &m.host, m.hosts, nil, ui.SelectGroups(m.groups))`},
	},
}

type selectGroupsDemo struct {
	hosts  []string
	groups []ui.SelectGroup
	host   string
}

var selectGroupsDemos = map[*State]*selectGroupsDemo{}

func selectGroupsDemoFor(state *State) *selectGroupsDemo {
	if demo := selectGroupsDemos[state]; demo != nil {
		return demo
	}
	demo := &selectGroupsDemo{
		hosts:  []string{"build-01", "db-primary", "staging", "bastion", "monitor"},
		groups: []ui.SelectGroup{{Label: state.Text("最近", "Recent"), From: 0}, {Label: state.Text("全部主机", "All hosts"), From: 2}},
		host:   "staging",
	}
	selectGroupsDemos[state] = demo
	return demo
}

func (state *State) paintSelectGroups(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := selectGroupsDemoFor(state)
	state.drawPage(canvas, theme, size, 360, state.Text("Select · 分组", "Select · Groups"), func(c ui.C) {
		ui.Select(c, &demo.host, demo.hosts, nil, ui.SelectGroups(demo.groups)).Placeholder(state.Text("主机", "Host")).Width(240)
	})
}
