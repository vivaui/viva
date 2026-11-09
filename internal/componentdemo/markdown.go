package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/markdown"
	"github.com/vivaui/viva/ui"
)

var markdownComponent = Component{
	ID: "markdown", Title: "Markdown", Category: "Display", Summary: "解析并排版聊天与说明文字中的 Markdown：标题、列表、引用、代码块、表格与链接。",
	paint: (*State).paintMarkdown, initialize: func(state *State) { state.markdown = &markdownScratch{} },
	Examples: []Example{
		{ID: "answer", Title: "助手回答", Description: "标题、段落内的粗体与行内代码、嵌套列表、引用与链接。", Snippet: `// doc and view are caller-owned and reused across paints.
doc.Parse(source)
view.Layout(canvas, components.MarkdownStyle{Size: theme.Metrics.FontSizeM, LineHeight: 20}, &doc, width)
view.Paint(canvas, components.MarkdownPaint{
    Look: components.MarkdownLookFor(theme), X: x, Y: y,
})`},
		{ID: "blocks", Title: "代码块与表格", Description: "代码块按单元格换行并预留按钮位置；表格列宽随内容或可用宽度分配。", Snippet: `view.Layout(canvas, components.MarkdownStyle{Size: theme.Metrics.FontSizeM, CodeInset: 28}, &doc, width)
view.Paint(canvas, components.MarkdownPaint{
    Look: components.MarkdownLookFor(theme), X: x, Y: y,
})
for _, code := range view.Codes() {
    button := gfx.Rect{X: x + code.Bounds.X + code.Bounds.Width - 28,
        Y: y + code.Bounds.Y + 8, Width: 20, Height: 20}
    canvas.Rounded(button, 4, theme.Muted)
    style := gfx.TextStyle{Size: theme.Metrics.FontSizeS, Color: theme.MutedForeground}
    canvas.LabelCenter("Copy", button, style)
}`},
		{ID: "unclosed", Title: "尚未写完", Description: "流式到达的文字中，未闭合的标记与围栏保持原样。", Snippet: `// A partial stream still lays out; unclosed markers stay literal.
doc.Parse(chunk)
view.Layout(canvas, components.MarkdownStyle{Size: theme.Metrics.FontSizeM}, &doc, width)
view.Paint(canvas, components.MarkdownPaint{
    Look: components.MarkdownLookFor(theme), X: x, Y: y,
})`},
	},
}

// markdownScratch is what the example keeps between paints.
type markdownScratch struct {
	doc  markdown.Document
	view components.MarkdownView
}

// markdownAnswer is the assistant's reply; only the sentence about the log
// differs between the Chinese preview and the English one.
const markdownAnswerRest = "\n" +
	"1. Check the status\n" +
	"   - run `systemctl status nginx`\n" +
	"   - read the last ~~20~~ 50 lines\n" +
	"2. Restart it\n" +
	"\n" +
	"> Back up the config before you change it.\n"

const markdownAnswer = "## Restarting nginx\n" +
	"Check the **config** first with `nginx -t`, then read the [docs](https://nginx.org/en/docs/) or the 日志 里的错误。\n" +
	markdownAnswerRest

const markdownAnswerEnglish = "## Restarting nginx\n" +
	"Check the **config** first with `nginx -t`, then read the [docs](https://nginx.org/en/docs/) or the errors in the log.\n" +
	markdownAnswerRest

const markdownBlocks = "```sh\nsudo systemctl restart nginx\njournalctl -u nginx --since \"10 min ago\" | tail -n 50\n```\n" +
	"\n" +
	"| Port | Service | Note |\n|:--|:-:|--:|\n| 22 | ssh | remote login |\n| 443 | https | encrypted web traffic and reverse proxy |\n"

const markdownUnclosed = "A **bold phrase that is still arriving, and `a code span\n" +
	"```go\nfmt.Println(\"the fence is not closed yet"

func (state *State) paintMarkdown(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var stage ui.Box
	state.page(canvas, theme, size, 560, state.Text("Markdown · 排版", "Markdown · Typesetting"), func(f *ui.Frame) {
		// The document paints itself into the rest of the stage; its height follows its text.
		stage = f.Slot().FillWidth().Height(max(size.Height-100, 120))
	})
	source := state.Text(markdownAnswer, markdownAnswerEnglish)
	switch state.exampleID {
	case "blocks":
		source = markdownBlocks
	case "unclosed":
		source = markdownUnclosed
	}
	bounds := stage.Rect()
	doc := &state.markdown.doc
	doc.Parse(source)
	view := &state.markdown.view
	view.Layout(canvas, components.MarkdownStyle{Size: theme.Metrics.FontSizeM, LineHeight: 20, CodeInset: 28}, doc, bounds.Width)
	view.Paint(canvas, components.MarkdownPaint{Look: components.MarkdownLookFor(theme), X: bounds.X, Y: bounds.Y, Clip: gfx.Rect{Y: 0, Width: size.Width, Height: size.Height}})
}
