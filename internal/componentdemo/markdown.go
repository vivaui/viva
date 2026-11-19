package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/markdown"
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
})`,
			PageSnippet: "answer := \"## Restarting nginx\\n\" +\n" +
				"    \"Check the **config** first with `nginx -t`, then read the [docs](https://nginx.org/en/docs/) or the errors in the log.\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"1. Check the status\\n\" +\n" +
				"    \"   - run `systemctl status nginx`\\n\" +\n" +
				"    \"   - read the last ~~20~~ 50 lines\\n\" +\n" +
				"    \"2. Restart it\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"> Back up the config before you change it.\\n\"\n" +
				"m.answerView.Source = answer\n" +
				"m.answerView.Style = markdown.Style{\n" +
				"    Size: c.Theme().Metrics.FontSizeM, LineHeight: 20,\n" +
				"}\n" +
				"markdown.View(c, \"Assistant answer\", &m.answerView).FillWidth().Height(420)",
			PageSnippetZh: "answer := \"## 重启 nginx\\n\" +\n" +
				"    \"先用 `nginx -t` 检查 **配置**，再阅读 [文档](https://nginx.org/en/docs/) 或日志里的错误。\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"1. 检查状态\\n\" +\n" +
				"    \"   - 运行 `systemctl status nginx`\\n\" +\n" +
				"    \"   - 查看最后 ~~20~~ 50 行\\n\" +\n" +
				"    \"2. 重启服务\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"> 修改配置前先备份。\\n\"\n" +
				"m.answerView.Source = answer\n" +
				"m.answerView.Style = markdown.Style{\n" +
				"    Size: c.Theme().Metrics.FontSizeM, LineHeight: 20,\n" +
				"}\n" +
				"markdown.View(c, \"助手回答\", &m.answerView).FillWidth().Height(420)"},
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
}`,
			PageSnippet: "blocks := \"```sh\\nsudo systemctl restart nginx\\njournalctl -u nginx --since \\\"10 min ago\\\" | tail -n 50\\n```\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"| Port | Service | Note |\\n|:--|:-:|--:|\\n| 22 | ssh | remote login |\\n| 443 | https | encrypted web traffic and reverse proxy |\\n\"\n" +
				"m.blocksView.Source = blocks\n" +
				"m.blocksView.Style = markdown.Style{\n" +
				"    Size: c.Theme().Metrics.FontSizeM, CodeInset: 28,\n" +
				"}\n" +
				"markdown.View(c, \"Code and table\", &m.blocksView).FillWidth().Height(420)",
			PageSnippetZh: "blocks := \"```sh\\nsudo systemctl restart nginx\\njournalctl -u nginx --since \\\"10 min ago\\\" | tail -n 50\\n```\\n\" +\n" +
				"    \"\\n\" +\n" +
				"    \"| 端口 | 服务 | 说明 |\\n|:--|:-:|--:|\\n| 22 | ssh | 远程登录 |\\n| 443 | https | 加密网络流量与反向代理 |\\n\"\n" +
				"m.blocksView.Source = blocks\n" +
				"m.blocksView.Style = markdown.Style{\n" +
				"    Size: c.Theme().Metrics.FontSizeM, CodeInset: 28,\n" +
				"}\n" +
				"markdown.View(c, \"代码与表格\", &m.blocksView).FillWidth().Height(420)"},
		{ID: "unclosed", Title: "尚未写完", Description: "流式到达的文字中，未闭合的标记与围栏保持原样。", Snippet: `// A partial stream still lays out; unclosed markers stay literal.
doc.Parse(chunk)
view.Layout(canvas, components.MarkdownStyle{Size: theme.Metrics.FontSizeM}, &doc, width)
view.Paint(canvas, components.MarkdownPaint{
    Look: components.MarkdownLookFor(theme), X: x, Y: y,
})`,
			PageSnippet: "partial := \"A **bold phrase that is still arriving, and `a code span\\n\" +\n" +
				"    \"```go\\nfmt.Println(\\\"the fence is not closed yet\"\n" +
				"m.partialView.Source = partial\n" +
				"m.partialView.Style = markdown.Style{Size: c.Theme().Metrics.FontSizeM}\n" +
				"markdown.View(c, \"Incoming answer\", &m.partialView).FillWidth().Height(420)",
			PageSnippetZh: "partial := \"一个仍在到达的**粗体短语，以及 `一段行内代码\\n\" +\n" +
				"    \"```go\\nfmt.Println(\\\"围栏还没有闭合\"\n" +
				"m.partialView.Source = partial\n" +
				"m.partialView.Style = markdown.Style{Size: c.Theme().Metrics.FontSizeM}\n" +
				"markdown.View(c, \"正在到达的回答\", &m.partialView).FillWidth().Height(420)"},
	},
}

// markdownScratch is what the example keeps between paints.
type markdownScratch struct {
	view markdown.Markdown
}

const markdownAnswerZh = "## 重启 nginx\n" +
	"先用 `nginx -t` 检查 **配置**，再阅读 [文档](https://nginx.org/en/docs/) 或日志里的错误。\n" +
	"\n" +
	"1. 检查状态\n" +
	"   - 运行 `systemctl status nginx`\n" +
	"   - 查看最后 ~~20~~ 50 行\n" +
	"2. 重启服务\n" +
	"\n" +
	"> 修改配置前先备份。\n"

const markdownAnswerEnglish = "## Restarting nginx\n" +
	"Check the **config** first with `nginx -t`, then read the [docs](https://nginx.org/en/docs/) or the errors in the log.\n" +
	"\n" +
	"1. Check the status\n" +
	"   - run `systemctl status nginx`\n" +
	"   - read the last ~~20~~ 50 lines\n" +
	"2. Restart it\n" +
	"\n" +
	"> Back up the config before you change it.\n"

const markdownBlocksZh = "```sh\nsudo systemctl restart nginx\njournalctl -u nginx --since \"10 min ago\" | tail -n 50\n```\n" +
	"\n" +
	"| 端口 | 服务 | 说明 |\n|:--|:-:|--:|\n| 22 | ssh | 远程登录 |\n| 443 | https | 加密网络流量与反向代理 |\n"

const markdownBlocksEnglish = "```sh\nsudo systemctl restart nginx\njournalctl -u nginx --since \"10 min ago\" | tail -n 50\n```\n" +
	"\n" +
	"| Port | Service | Note |\n|:--|:-:|--:|\n| 22 | ssh | remote login |\n| 443 | https | encrypted web traffic and reverse proxy |\n"

const markdownUnclosedZh = "一个仍在到达的**粗体短语，以及 `一段行内代码\n" +
	"```go\nfmt.Println(\"围栏还没有闭合"

const markdownUnclosedEnglish = "A **bold phrase that is still arriving, and `a code span\n" +
	"```go\nfmt.Println(\"the fence is not closed yet"

func (state *State) paintMarkdown(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	source := state.Text(markdownAnswerZh, markdownAnswerEnglish)
	switch state.exampleID {
	case "blocks":
		source = state.Text(markdownBlocksZh, markdownBlocksEnglish)
	case "unclosed":
		source = state.Text(markdownUnclosedZh, markdownUnclosedEnglish)
	}
	view := &state.markdown.view
	view.Source = source
	view.Style = markdown.Style{Size: theme.Metrics.FontSizeM, LineHeight: 20, CodeInset: 28}
	label := state.Text("Markdown 文档", "Markdown document")
	state.drawPage(canvas, theme, size, 560, state.Text("Markdown · 排版", "Markdown · Typesetting"), func(c ui.C) {
		markdown.View(c, label, view).FillWidth().Height(max(size.Height-100, 120))
	})
}
