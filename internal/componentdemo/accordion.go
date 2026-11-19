package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var accordionComponent = Component{
	ID: "accordion", Title: "Accordion", Category: "Disclosure", Summary: "由调用方持有展开状态的多段折叠内容。",
	paint: (*State).paintAccordion, initialize: initializeAccordion,
	Examples: []Example{
		{ID: "default", Title: "常见问题", Description: "点击标题独立展开或收起内容。", Snippet: `items := tree.Sub(page, tree.Name("item"))
accordion := components.BeginAccordion(canvas, theme, bounds)
for index, title := range titles {
    content := accordion.Item(items.At(index), title, open&(1<<index) != 0, 38)
    if content.Height > 0 {
        style := gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground}
        canvas.Label(answers[index], content, style)
    }
    if canvas.Clicked(items.At(index)) {
        open ^= 1 << index
    }
}`, PageSnippet: `m.faq.Multiple = true
ui.Accordion(c, m.faq, []ui.AccordionItem{
    {Title: "Does it repaint constantly?", Content: func() {
        ui.Text(c, "A new frame is submitted only when content changes.").Muted()
    }},
    {Title: "Who owns the state?", Content: func() {
        ui.Text(c, "The expanded state is owned explicitly by the application.").Muted()
    }},
    {Title: "Is it keyboard accessible?", Content: func() {
        ui.Text(c, "Headers declare focusable button semantics.").Muted()
    }},
})`, PageSnippetZh: `m.faq.Multiple = true
ui.Accordion(c, m.faq, []ui.AccordionItem{
    {Title: "是否持续重绘？", Content: func() {
        ui.Text(c, "仅在内容变化时提交新帧。").Muted()
    }},
    {Title: "状态由谁持有？", Content: func() {
        ui.Text(c, "展开位由应用状态明确持有。").Muted()
    }},
    {Title: "是否支持键盘？", Content: func() {
        ui.Text(c, "标题声明为可聚焦按钮语义。").Muted()
    }},
})`},
		{ID: "disabled", Title: "禁用状态", Description: "保留展开内容，但所有标题均不可交互。", Snippet: `items := tree.Sub(page, tree.Name("item"))
accordion := components.BeginAccordion(canvas, theme, bounds).Disabled()
for index, title := range titles {
    content := accordion.Item(items.At(index), title, open&(1<<index) != 0, 38)
    if content.Height > 0 {
        style := gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground}
        canvas.Label(answers[index], content, style)
    }
}`, PageSnippet: `m.faq.Disabled = true
m.faq.Multiple = true
ui.Accordion(c, m.faq, []ui.AccordionItem{
    {Title: "Does it repaint constantly?", Content: func() {
        ui.Text(c, "A new frame is submitted only when content changes.").Muted()
    }},
    {Title: "Who owns the state?", Content: func() {
        ui.Text(c, "The expanded state is owned explicitly by the application.").Muted()
    }},
    {Title: "Is it keyboard accessible?", Content: func() {
        ui.Text(c, "Headers declare focusable button semantics.").Muted()
    }},
})`, PageSnippetZh: `m.faq.Disabled = true
m.faq.Multiple = true
ui.Accordion(c, m.faq, []ui.AccordionItem{
    {Title: "是否持续重绘？", Content: func() {
        ui.Text(c, "仅在内容变化时提交新帧。").Muted()
    }},
    {Title: "状态由谁持有？", Content: func() {
        ui.Text(c, "展开位由应用状态明确持有。").Muted()
    }},
    {Title: "是否支持键盘？", Content: func() {
        ui.Text(c, "标题声明为可聚焦按钮语义。").Muted()
    }},
})`},
	},
}

func initializeAccordion(state *State) { state.flags = 1 }

// accordionDemo is one example's page disclosure state and the fixed items it
// declares. It is kept beside the State rather than rebuilt in each paint, so a
// warm paint allocates nothing and the content closures stay stable.
type accordionDemo struct {
	faq   ui.AccordionState
	items []ui.AccordionItem
	c     ui.C
}

var accordionDemos = map[*State]*accordionDemo{}

func accordionDemoFor(state *State) *accordionDemo {
	if demo := accordionDemos[state]; demo != nil {
		return demo
	}
	titles := [3]string{
		state.Text("是否持续重绘？", "Does it repaint constantly?"),
		state.Text("状态由谁持有？", "Who owns the state?"),
		state.Text("是否支持键盘？", "Is it keyboard accessible?"),
	}
	answers := [3]string{
		state.Text("仅在内容变化时提交新帧。", "A new frame is submitted only when content changes."),
		state.Text("展开位由应用状态明确持有。", "The expanded state is owned explicitly by the application."),
		state.Text("标题声明为可聚焦按钮语义。", "Headers declare focusable button semantics."),
	}
	demo := &accordionDemo{faq: ui.AccordionState{Open: make([]bool, len(titles)), Multiple: true}}
	demo.items = []ui.AccordionItem{
		{Title: titles[0], Content: func() { ui.Text(demo.c, answers[0]).Muted() }},
		{Title: titles[1], Content: func() { ui.Text(demo.c, answers[1]).Muted() }},
		{Title: titles[2], Content: func() { ui.Text(demo.c, answers[2]).Muted() }},
	}
	accordionDemos[state] = demo
	return demo
}

func (state *State) paintAccordion(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := accordionDemoFor(state)
	for index := range demo.faq.Open {
		demo.faq.Open[index] = state.flags&(1<<index) != 0
	}
	demo.faq.Disabled = state.exampleID == "disabled"
	state.drawPage(canvas, theme, size, 560, state.Text("Accordion · 常见问题", "Accordion · Common questions"), func(c ui.C) {
		demo.c = c
		ui.Accordion(c, &demo.faq, demo.items)
	})
	var flags uint64
	for index, open := range demo.faq.Open {
		if open {
			flags |= 1 << index
		}
	}
	state.flags = flags
}
