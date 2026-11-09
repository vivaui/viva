package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var messageComponent = Component{
	ID: "message", Title: "Message", Category: "Communication", Summary: "带头像、元数据与可组合正文槽的消息行。",
	paint: (*State).paintMessage,
	Examples: []Example{
		{ID: "default", Title: "收到的消息", Description: "生产 Message 负责头像和元数据，调用方填充正文槽。", Snippet: `regions := components.Message(canvas, theme, bounds, components.MessageData{
    Header: "Ari · Design", Footer: "09:42", Initials: "AR", ShowAvatar: true,
})
components.Bubble(canvas, theme, regions.Content,
    components.BubbleStart, components.BubbleSecondary, "Native preview ready.")`},
		{ID: "alignment", Title: "双向消息", Description: "起始与结束方向并列展示，结束方向的页眉页脚靠右。", Snippet: `ari := components.Message(canvas, theme, first, components.MessageData{
    Header: "Ari · Design", Footer: "09:42", Initials: "AR", ShowAvatar: true,
})
components.Bubble(canvas, theme, ari.Content, components.BubbleStart,
    components.BubbleSecondary, "Native preview ready.")
you := components.Message(canvas, theme, second, components.MessageData{
    Header: "You", Footer: "Delivered", Initials: "YO",
    Alignment: components.MessageEnd, ShowAvatar: true,
})
components.Bubble(canvas, theme, you.Content, components.BubbleEnd,
    components.BubbleDefault, "Ship the compact variant.")`},
		{ID: "group", Title: "连续消息", Description: "同一发送者的连续消息只在最后一条显示头像，其余预留头像位。", Snippet: `group := components.BeginMessageGroup(theme, bounds)
first := components.Message(canvas, theme, group.Row(44),
    components.MessageData{ReserveAvatar: true})
components.Bubble(canvas, theme, first.Content,
    components.BubbleStart, components.BubbleSecondary, "Native preview is ready.")
last := components.Message(canvas, theme, group.Row(64),
    components.MessageData{Initials: "AR", ShowAvatar: true, Footer: "09:42"})
components.Bubble(canvas, theme, last.Content,
    components.BubbleStart, components.BubbleSecondary, "Ship it when the gate is green.")`},
	},
}

func (state *State) paintMessage(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var rows [3]ui.Box
	state.page(canvas, theme, size, 600, state.Text("Message · 对话组合", "Message · Conversation"), func(f *ui.Frame) {
		switch state.exampleID {
		case "group":
			// The rows of one group are stacked by BeginMessageGroup below.
			rows[0] = f.Slot().FillWidth().Height(210)
		case "alignment":
			rows[0] = f.Slot().FillWidth().Height(118)
			rows[1] = f.Slot().FillWidth().Height(118)
		default:
			rows[0] = f.Slot().FillWidth().Height(118)
		}
	})
	paint := func(bounds gfx.Rect, data components.MessageData, variant components.BubbleVariant, body string) {
		regions := components.Message(canvas, theme, bounds, data)
		alignment := components.BubbleStart
		if data.Alignment == components.MessageEnd {
			alignment = components.BubbleEnd
		}
		components.Bubble(canvas, theme, regions.Content, alignment, variant, body)
	}
	switch state.exampleID {
	case "group":
		group := components.BeginMessageGroup(theme, rows[0].Rect())
		paint(group.Row(44), components.MessageData{ReserveAvatar: true}, components.BubbleSecondary, "Native preview is ready.")
		paint(group.Row(44), components.MessageData{ReserveAvatar: true}, components.BubbleSecondary, "I checked the narrow window too.")
		paint(group.Row(64), components.MessageData{Initials: "AR", ShowAvatar: true, Footer: "09:42"}, components.BubbleSecondary, "Ship it when the gate is green.")
	case "alignment":
		paint(rows[0].Rect(), components.MessageData{Header: "Ari · Design", Footer: "09:42", Initials: "AR", ShowAvatar: true}, components.BubbleSecondary, "Native preview ready.")
		paint(rows[1].Rect(), components.MessageData{Header: "You", Footer: "Delivered", Initials: "YO", Alignment: components.MessageEnd, ShowAvatar: true}, components.BubbleDefault, "Ship the compact variant.")
	default:
		paint(rows[0].Rect(), components.MessageData{Header: "Ari · Design", Footer: "09:42", Initials: "AR", ShowAvatar: true}, components.BubbleSecondary, "Native preview ready.")
	}
}
