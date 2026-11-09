package componentdemo

import (
	"github.com/vivaui/viva/components"
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
}`},
		{ID: "disabled", Title: "禁用状态", Description: "保留展开内容，但所有标题均不可交互。", Snippet: `items := tree.Sub(page, tree.Name("item"))
accordion := components.BeginAccordion(canvas, theme, bounds).Disabled()
for index, title := range titles {
    content := accordion.Item(items.At(index), title, open&(1<<index) != 0, 38)
    if content.Height > 0 {
        style := gfx.TextStyle{Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground}
        canvas.Label(answers[index], content, style)
    }
}`},
	},
}

var (
	accordionUI = tree.Feature("accordion")
	// accordionItem's item i is the i-th disclosure header.
	accordionItem = tree.Sub(accordionUI, tree.Name("item"))
)

const accordionContentHeight = 38

func initializeAccordion(state *State) { state.flags = 1 }

func (state *State) paintAccordion(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	titles := [...]string{
		state.Text("是否持续重绘？", "Does it repaint constantly?"),
		state.Text("状态由谁持有？", "Who owns the state?"),
		state.Text("是否支持键盘？", "Is it keyboard accessible?"),
	}
	answers := [...]string{
		state.Text("仅在内容变化时提交新帧。", "A new frame is submitted only when content changes."),
		state.Text("展开位由应用状态明确持有。", "The expanded state is owned explicitly by the application."),
		state.Text("标题声明为可聚焦按钮语义。", "Headers declare focusable button semantics."),
	}
	// The accordion paints its own rows; the slot reserves the height they take:
	// every header, a divider between them and the answer of each open item.
	height := components.AccordionHeaderExtent(theme)*float32(len(titles)) + float32(len(titles)-1)
	for index := range titles {
		if state.flags&(1<<index) != 0 {
			height += accordionContentHeight + 16
		}
	}
	var slot ui.Box
	state.page(canvas, theme, size, 560, state.Text("Accordion · 常见问题", "Accordion · Common questions"), func(f *ui.Frame) {
		slot = f.Slot().FillWidth().Height(height)
	})
	accordion := components.BeginAccordion(canvas, theme, slot.Rect())
	disabled := state.exampleID == "disabled"
	if disabled {
		accordion = accordion.Disabled()
	}
	for index := range titles {
		open := state.flags&(1<<index) != 0
		id := accordionItem.At(index)
		content := accordion.Item(id, titles[index], open, accordionContentHeight)
		if content.Height > 0 {
			canvas.Label(answers[index], gfx.Rect{X: content.X, Y: content.Y, Width: content.Width - 20, Height: 24}, gfx.TextStyle{
				Size: theme.Metrics.FontSizeM, Color: theme.MutedForeground,
			})
		}
		if !disabled && canvas.Clicked(id) {
			state.flags ^= 1 << index
		}
	}
}
