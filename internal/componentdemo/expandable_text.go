package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/typography"
)

var expandableTextComponent = Component{
	ID: "expandable-text", Title: "Expandable Text", Category: "Display", Summary: "裁成几行并可展开、可复制的文字。",
	Source: "ui/typography/typography.go", paint: (*State).paintExpandableText,
	Examples: []Example{
		{ID: "default", Title: "展开与复制", Description: "超过 Rows 行时以省略号结尾，Expandable 加“展开”按钮（Copyable 还会加一个把完整文本放进剪贴板的按钮，这里不演示）；是否被裁剪由上一次绘制的宽度判断。", Snippet: `m.about = typography.New()
// in the page:
m.about.View(c, m.description, typography.Spec{Rows: 3, Expandable: true, Copyable: true}).FillWidth()
if m.about.Copied() {
    // the text went to the clipboard
}`, PageSnippet: `m.about = typography.New()
// in the page:
m.about.View(c, m.description, typography.Spec{Rows: 3, Expandable: true, Copyable: true}).FillWidth()
if m.about.Copied() {
    // the text went to the clipboard
}`, PageSnippetZh: `m.about = typography.New()
// 在页面中：
m.about.View(c, m.description, typography.Spec{Rows: 3, Expandable: true, Copyable: true}).FillWidth()
if m.about.Copied() {
    // 文本已放进剪贴板
}`},
	},
}

type expandableTextDemo struct {
	text    *typography.Text
	content string
}

var expandableTextDemos = map[*State]*expandableTextDemo{}

func expandableTextDemoFor(state *State) *expandableTextDemo {
	if demo := expandableTextDemos[state]; demo != nil {
		return demo
	}
	demo := &expandableTextDemo{text: typography.New(), content: state.Text(
		"Viva 保留场景节点，只在内容变化时重绘；空闲的应用不会调度帧、定时器或后台工作。组件声明与布局辅助函数在预热后不分配内存，并由签入的基准测试约束。这一段故意写得比三行更长，用来演示被裁剪的文字如何展开。",
		"Viva keeps scene nodes and repaints only when content changes; an idle application schedules no frames, timers or background work. Component declarations and layout helpers allocate nothing once warm, and a checked-in benchmark holds them to it. This paragraph is longer than three lines on purpose, to show how a cut text expands.")}
	expandableTextDemos[state] = demo
	return demo
}

func (state *State) paintExpandableText(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := expandableTextDemoFor(state)
	// The copy button is not declared here: the demo tests press every control, and a press writes
	// the system clipboard, which a test must not do.
	spec := typography.Spec{Rows: 3, Expandable: true, Labels: typography.Labels{
		More: state.Text("展开", "More"), Less: state.Text("收起", "Less"), Copy: state.Text("复制", "Copy"),
	}}
	state.drawPage(canvas, theme, size, 480, state.Text("Text · 展开与复制", "Expandable Text"), func(c ui.C) {
		demo.text.View(c, demo.content, spec).FillWidth()
	})
}
