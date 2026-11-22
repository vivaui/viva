package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var bubbleComponent = Component{
	ID: "bubble", Title: "Bubble", Category: "Communication", Summary: "用于对话内容、对齐和轻量回应的消息气泡。",
	paint: (*State).paintBubble,
	Examples: []Example{
		{ID: "conversation", Title: "双向对话", Description: "开始和结束对齐由生产 Bubble 处理。", Snippet: `question := components.Bubble(canvas, theme, top, components.BubbleStart,
    components.BubbleSecondary, "The preview is updated — can you see the new state?")
reply := components.Bubble(canvas, theme, bottom, components.BubbleEnd,
    components.BubbleTinted, "Yes, it reads clearly in light and dark.")
reaction := tree.ID(page, "reaction")
components.BubbleReaction(canvas, theme, reaction, reply,
    components.BubbleReactionBottom, components.BubbleEnd, reactionLabel)
if canvas.Clicked(reaction) {
    liked = !liked
}`, PageSnippet: `ui.Bubble(c, "The preview is updated — can you see the new state?", ui.BubbleSecondary).Self(ui.Start)
ui.Bubble(c, "Yes, it reads clearly in light and dark. In a narrow window this longer reply wraps at 80% width, while short messages take only their own width.", ui.BubbleTinted).Self(ui.End)
count := "👍 4"
if m.liked {
    count = "👍 5"
}
if ui.Text(c, count).Self(ui.End).Muted().Hit(m.reactionID, count).Clicked() {
    m.liked = !m.liked
}`, PageSnippetZh: `ui.Bubble(c, "预览已更新，能看到新状态吗？", ui.BubbleSecondary).Self(ui.Start)
ui.Bubble(c, "可以，亮暗主题下都很清晰。窄窗口里这条较长的回复会在 80% 宽度处换行，而短消息只占自己的宽度。", ui.BubbleTinted).Self(ui.End)
count := "👍 4"
if m.liked {
    count = "👍 5"
}
if ui.Text(c, count).Self(ui.End).Muted().Hit(m.reactionID, count).Clicked() {
    m.liked = !m.liked
}`},
		{ID: "variants", Title: "语义变体", Description: "展示次要、描边与破坏性消息。", Snippet: `components.Bubble(canvas, theme, first, components.BubbleStart,
    components.BubbleSecondary, "The build is done — ready for review.")
components.Bubble(canvas, theme, second, components.BubbleEnd,
    components.BubbleOutline, "Got it. I will check the narrow window layout first.")
components.Bubble(canvas, theme, third, components.BubbleStart,
    components.BubbleDestructive, "Release blocked: the signing configuration is missing.")`, PageSnippet: `ui.Bubble(c, "The build is done — ready for review.", ui.BubbleSecondary).Self(ui.Start)
ui.Bubble(c, "Got it. I will check the narrow window layout first.", ui.BubbleOutline).Self(ui.End)
ui.Bubble(c, "Release blocked: the signing configuration is missing.", ui.BubbleDestructive).Self(ui.Start)`, PageSnippetZh: `ui.Bubble(c, "构建已完成，可以开始验收。", ui.BubbleSecondary).Self(ui.Start)
ui.Bubble(c, "收到，我先检查窄窗口布局。", ui.BubbleOutline).Self(ui.End)
ui.Bubble(c, "发布被阻止：签名配置缺失。", ui.BubbleDestructive).Self(ui.Start)`},
	},
}

var (
	bubbleUI       = tree.Feature("bubble")
	bubbleReaction = tree.ID(bubbleUI, "reaction")
)

func (state *State) paintBubble(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	variants := state.exampleID == "variants"
	state.drawPage(canvas, theme, size, 560, state.Text("Bubble · 项目讨论", "Bubble · Project discussion"), func(c ui.C) {
		if variants {
			ui.Bubble(c, state.Text("构建已完成，可以开始验收。", "The build is done — ready for review."), ui.BubbleSecondary).Self(ui.Start)
			ui.Bubble(c, state.Text("收到，我先检查窄窗口布局。", "Got it. I will check the narrow window layout first."), ui.BubbleOutline).Self(ui.End)
			ui.Bubble(c, state.Text("发布被阻止：签名配置缺失。", "Release blocked: the signing configuration is missing."), ui.BubbleDestructive).Self(ui.Start)
			return
		}
		ui.Bubble(c, state.Text("预览已更新，能看到新状态吗？", "The preview is updated — can you see the new state?"), ui.BubbleSecondary).Self(ui.Start)
		ui.Bubble(c, state.Text("可以，亮暗主题下都很清晰。窄窗口里这条较长的回复会在 80% 宽度处换行，而短消息只占自己的宽度。", "Yes, it reads clearly in light and dark. In a narrow window this longer reply wraps at 80% width, while short messages take only their own width."), ui.BubbleTinted).Self(ui.End)
		reaction := "👍 4"
		if state.flags&1 != 0 {
			reaction = "👍 5"
		}
		if ui.Text(c, reaction).Self(ui.End).FontSize(theme.Metrics.FontSizeS).Muted().
			Hit(bubbleReaction, reaction).Clicked() {
			state.flags ^= 1
		}
	})
}
