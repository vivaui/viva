package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var collapsibleComponent = Component{
	ID: "collapsible", Title: "Collapsible", Category: "Disclosure", Summary: "无装饰触发器与调用方绘制内容的折叠原语。",
	paint: (*State).paintCollapsible, initialize: initializeCollapsible,
	Examples: []Example{
		{ID: "default", Title: "构建详情", Description: "点击标题展开或收起调用方内容。", Snippet: `trigger := tree.ID(page, "trigger")
collapsible := components.BeginCollapsible(canvas, theme, trigger, bounds)
content := collapsible.Paint("3 build artifacts", open, 112)
if canvas.Clicked(trigger) {
    open = !open
}
canvas.Rounded(content, theme.Metrics.RadiusSurface, theme.Muted)`, PageSnippet: `ui.Collapsible(c, &m.builds, "3 build artifacts", func() {
    for _, line := range []string{
        "viva-darwin-arm64      3.8 MiB",
        "viva-windows-amd64     4.1 MiB",
        "viva-linux-amd64       3.9 MiB",
    } {
        ui.Text(c, line).Mono().Muted()
    }
})`, PageSnippetZh: `ui.Collapsible(c, &m.builds, "3 个构建产物", func() {
    for _, line := range []string{
        "viva-darwin-arm64      3.8 MiB",
        "viva-windows-amd64     4.1 MiB",
        "viva-linux-amd64       3.9 MiB",
    } {
        ui.Text(c, line).Mono().Muted()
    }
})`},
		{ID: "disabled", Title: "禁用状态", Description: "保持展开快照但禁止触发。", Snippet: `trigger := tree.ID(page, "trigger")
collapsible := components.BeginCollapsible(canvas, theme, trigger, bounds).Disabled()
content := collapsible.Paint("3 build artifacts", open, 112)
if content.Height > 0 {
    canvas.Rounded(content, theme.Metrics.RadiusSurface, theme.Muted)
}`, PageSnippet: `m.builds.Disabled = true
ui.Collapsible(c, &m.builds, "3 build artifacts", func() {
    for _, line := range []string{
        "viva-darwin-arm64      3.8 MiB",
        "viva-windows-amd64     4.1 MiB",
        "viva-linux-amd64       3.9 MiB",
    } {
        ui.Text(c, line).Mono().Muted()
    }
})`, PageSnippetZh: `m.builds.Disabled = true
ui.Collapsible(c, &m.builds, "3 个构建产物", func() {
    for _, line := range []string{
        "viva-darwin-arm64      3.8 MiB",
        "viva-windows-amd64     4.1 MiB",
        "viva-linux-amd64       3.9 MiB",
    } {
        ui.Text(c, line).Mono().Muted()
    }
})`},
	},
}

func initializeCollapsible(state *State) { state.flags = 1 }

// collapsibleDemo is one example's page disclosure state, title and fixed body.
// It is kept beside the State so a warm paint declares no new closures.
type collapsibleDemo struct {
	state ui.CollapsibleState
	title string
	lines [3]string
	c     ui.C
}

var collapsibleDemos = map[*State]*collapsibleDemo{}

func collapsibleDemoFor(state *State) *collapsibleDemo {
	if demo := collapsibleDemos[state]; demo != nil {
		return demo
	}
	demo := &collapsibleDemo{
		title: state.Text("3 个构建产物", "3 build artifacts"),
		lines: [3]string{
			"viva-darwin-arm64      3.8 MiB",
			"viva-windows-amd64     4.1 MiB",
			"viva-linux-amd64       3.9 MiB",
		},
	}
	collapsibleDemos[state] = demo
	return demo
}

func (state *State) paintCollapsible(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := collapsibleDemoFor(state)
	demo.state.Open = state.flags&1 != 0
	demo.state.Disabled = state.exampleID == "disabled"
	state.drawPage(canvas, theme, size, 520, state.Text("Collapsible · 构建输出", "Collapsible · Build output"), func(c ui.C) {
		demo.c = c
		ui.Collapsible(c, &demo.state, demo.title, func() {
			for _, line := range demo.lines {
				ui.Text(demo.c, line).Mono().Muted()
			}
		})
	})
	if demo.state.Open {
		state.flags |= 1
	} else {
		state.flags &^= 1
	}
}
