package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/anchor"
)

var anchorComponent = Component{
	ID: "anchor", Title: "Anchor", Category: "Navigation", Summary: "锚点：长页面的目录，侧边竖线标出当前所在章节。",
	Source: "ui/anchor/anchor.go", paint: (*State).paintAnchor,
	Examples: []Example{
		{ID: "default", Title: "页内目录", Description: "点击链接由页面滚动到对应章节，并告诉控件哪一节是当前的；Level 缩进嵌套标题。", Snippet: `m.toc = anchor.New()
m.toc.View(c, "On this page", anchor.Spec{Items: m.sections, Active: m.section})
if i, ok := m.toc.Clicked(); ok {
    m.section = i
    // scroll the page to m.sections[i]
}`, PageSnippet: `m.toc = anchor.New()
m.toc.View(c, "On this page", anchor.Spec{Items: m.sections, Active: m.section})
if i, ok := m.toc.Clicked(); ok {
    m.section = i
    // scroll the page to m.sections[i]
}`, PageSnippetZh: `m.toc = anchor.New()
m.toc.View(c, "On this page", anchor.Spec{Items: m.sections, Active: m.section})
if i, ok := m.toc.Clicked(); ok {
    m.section = i
    // 把页面滚动到 m.sections[i]
}`},
	},
}

type anchorDemo struct {
	toc    *anchor.Anchor
	items  []anchor.Item
	active int
}

var anchorDemos = map[*State]*anchorDemo{}

func anchorDemoFor(state *State) *anchorDemo {
	if demo := anchorDemos[state]; demo != nil {
		return demo
	}
	demo := &anchorDemo{toc: anchor.New(), items: []anchor.Item{
		{Title: "Overview"}, {Title: "Install"}, {Title: "From source", Level: 1}, {Title: "From a package", Level: 1}, {Title: "Usage"}, {Title: "License"},
	}, active: 1}
	anchorDemos[state] = demo
	return demo
}

func (state *State) paintAnchor(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := anchorDemoFor(state)
	state.drawPage(canvas, theme, size, 280, state.Text("Anchor · 锚点", "Anchor"), func(c ui.C) {
		demo.toc.View(c, state.Text("本页内容", "On this page"), anchor.Spec{Items: demo.items, Active: demo.active}).FillWidth()
		if i, ok := demo.toc.Clicked(); ok {
			demo.active = i
		}
	})
}
