package componentdemo

import (
	"github.com/vivaui/viva/components"
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
}`},
		{ID: "variants", Title: "语义变体", Description: "展示次要、描边与破坏性消息。", Snippet: `components.Bubble(canvas, theme, first, components.BubbleStart,
    components.BubbleSecondary, "The build is done — ready for review.")
components.Bubble(canvas, theme, second, components.BubbleEnd,
    components.BubbleOutline, "Got it. I will check the narrow window layout first.")
components.Bubble(canvas, theme, third, components.BubbleStart,
    components.BubbleDestructive, "Release blocked: the signing configuration is missing.")`},
	},
}

var (
	bubbleUI       = tree.Feature("bubble")
	bubbleReaction = tree.ID(bubbleUI, "reaction")
)

func (state *State) paintBubble(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box // the conversation uses two; the variants three
	variants := state.exampleID == "variants"
	state.page(canvas, theme, size, 560, state.Text("Bubble · 项目讨论", "Bubble · Project discussion"), func(f *ui.Frame) {
		if variants {
			for index := range rows {
				rows[index] = f.Slot().FillWidth().Height(54)
			}
			return
		}
		rows[0] = f.Slot().FillWidth().Height(50)
		rows[1] = f.Slot().FillWidth().Height(90)
	})
	if variants {
		components.Bubble(canvas, theme, rows[0].Rect(), components.BubbleStart, components.BubbleSecondary, state.Text("构建已完成，可以开始验收。", "The build is done — ready for review."))
		components.Bubble(canvas, theme, rows[1].Rect(), components.BubbleEnd, components.BubbleOutline, state.Text("收到，我先检查窄窗口布局。", "Got it. I will check the narrow window layout first."))
		components.Bubble(canvas, theme, rows[2].Rect(), components.BubbleStart, components.BubbleDestructive, state.Text("发布被阻止：签名配置缺失。", "Release blocked: the signing configuration is missing."))
		return
	}
	components.Bubble(canvas, theme, rows[0].Rect(), components.BubbleStart, components.BubbleSecondary, state.Text("预览已更新，能看到新状态吗？", "The preview is updated — can you see the new state?"))
	reply := components.Bubble(canvas, theme, rows[1].Rect(), components.BubbleEnd, components.BubbleTinted, state.Text("可以，亮暗主题下都很清晰。窄窗口里这条较长的回复会在 80% 宽度处换行，而短消息只占自己的宽度。", "Yes, it reads clearly in light and dark. In a narrow window this longer reply wraps at 80% width, while short messages take only their own width."))
	reaction := "👍 4"
	if state.flags&1 != 0 {
		reaction = "👍 5"
	}
	components.BubbleReaction(canvas, theme, bubbleReaction, reply, components.BubbleReactionBottom, components.BubbleEnd, reaction)
	if canvas.Clicked(bubbleReaction) {
		state.flags ^= 1
	}
}
