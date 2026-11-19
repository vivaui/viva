package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/messageview"
)

var messageComponent = Component{
	ID: "message", Title: "Message", Category: "Communication", Summary: "带头像、元数据与可组合正文槽的消息行。",
	paint: (*State).paintMessage, initialize: (*State).initializeMessage,
	Examples: []Example{
		{ID: "default", Title: "收到的消息", Description: "页面视图组合头像、名称、时间、正文与操作，正文使用 ui.Bubble。", Snippet: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · Design").Time("09:42").Avatar("AR").
    Body("Native preview ready.").
    Actions(func(c ui.C) {
        if ui.Button(c, "Copy").Ghost().Small().Clicked() {
            state.copied = true
        }
    }))`, PageSnippet: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · Design").Time("09:42").Avatar("AR").
    Body("Native preview ready.").
    Actions(func(c ui.C) {
        label := "Copy"
        if m.copied {
            label = "Copied"
        }
        if ui.Button(c, label).Ghost().Small().Clicked() {
            m.copied = true
        }
    }))`, PageSnippetZh: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · 设计").Time("09:42").Avatar("AR").
    Body("原生预览已就绪。").
    Actions(func(c ui.C) {
        label := "复制"
        if m.copied {
            label = "已复制"
        }
        if ui.Button(c, label).Ghost().Small().Clicked() {
            m.copied = true
        }
    }))`},
		{ID: "alignment", Title: "双向消息", Description: "Start 与 End 互为镜像：结束方向的头像在末尾，气泡与元数据靠右。", Snippet: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · Design").Time("09:42").Avatar("AR").
    Body("Native preview ready."))
messageview.View(c, "You", messageview.New().End().
    Name("You").Time("Delivered").Avatar("YO").
    Surface(ui.BubblePrimary).Body("Ship the compact variant."))`, PageSnippet: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · Design").Time("09:42").Avatar("AR").
    Body("Native preview ready."))
messageview.View(c, "You", messageview.New().End().
    Name("You").Time("Delivered").Avatar("YO").
    Surface(ui.BubblePrimary).Body("Ship the compact variant."))`, PageSnippetZh: `messageview.View(c, "Ari", messageview.New().
    Name("Ari · 设计").Time("09:42").Avatar("AR").
    Body("原生预览已就绪。"))
messageview.View(c, "我", messageview.New().End().
    Name("我").Time("已送达").Avatar("YO").
    Surface(ui.BubblePrimary).Body("发布紧凑变体。"))`},
		{ID: "group", Title: "连续消息", Description: "同一发送者的连续行只预留头像位，最后一行显示头像与时间。", Snippet: `messageview.Group(c, "Ari",
    messageview.New().ReserveAvatar().
        Body("Native preview is ready."),
    messageview.New().ReserveAvatar().
        Body("I checked the narrow window too."),
    messageview.New().Avatar("AR").Time("09:42").
        Body("Ship it when the gate is green."),
)`, PageSnippet: `messageview.Group(c, "Ari",
    messageview.New().ReserveAvatar().
        Body("Native preview is ready."),
    messageview.New().ReserveAvatar().
        Body("I checked the narrow window too."),
    messageview.New().Avatar("AR").Time("09:42").
        Body("Ship it when the gate is green."),
)`, PageSnippetZh: `messageview.Group(c, "Ari",
    messageview.New().ReserveAvatar().
        Body("原生预览已就绪。"),
    messageview.New().ReserveAvatar().
        Body("我也检查了窄窗口。"),
    messageview.New().Avatar("AR").Time("09:42").
        Body("门禁通过后就发布。"),
)`},
	},
}

func (state *State) paintMessage(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := messageDemoFor(state)
	state.drawPage(canvas, theme, size, 600, state.Text("Message · 对话组合", "Message · Conversation"), func(c ui.C) {
		switch state.exampleID {
		case "alignment":
			messageview.View(c, state.Text("收到的消息", "Incoming message"), messageview.New().
				Name(state.Text("Ari · 设计", "Ari · Design")).
				Time("09:42").Avatar("AR").
				Body(state.Text("原生预览已就绪。", "Native preview ready."))).FillWidth()
			messageview.View(c, state.Text("发送的消息", "Outgoing message"), messageview.New().End().
				Name(state.Text("我", "You")).
				Time(state.Text("已送达", "Delivered")).Avatar("YO").
				Surface(ui.BubblePrimary).
				Body(state.Text("发布紧凑变体。", "Ship the compact variant."))).FillWidth()
		case "group":
			messageview.Group(c, state.Text("连续消息", "Consecutive messages"),
				messageview.New().ReserveAvatar().
					Body(state.Text("原生预览已就绪。", "Native preview is ready.")),
				messageview.New().ReserveAvatar().
					Body(state.Text("我也检查了窄窗口。", "I checked the narrow window too.")),
				messageview.New().Avatar("AR").Time("09:42").
					Body(state.Text("门禁通过后就发布。", "Ship it when the gate is green.")),
			).FillWidth()
		default:
			messageview.View(c, state.Text("消息", "Message"), messageview.New().
				Name(state.Text("Ari · 设计", "Ari · Design")).
				Time("09:42").Avatar("AR").
				Body(state.Text("原生预览已就绪。", "Native preview ready.")).
				Actions(demo.action)).FillWidth()
		}
	})
}

// messageDemo is the mutable state and stable callback of the message example.
// It is kept beside the State rather than rebuilt in each paint.
type messageDemo struct {
	state  *State
	action func(ui.C)
}

// messageDemos is keyed by State because the constrained demo model has no
// field for this example; a State is painted on one thread at a time.
var messageDemos = map[*State]*messageDemo{}

func (state *State) initializeMessage() {
	messageDemoFor(state)
}

func messageDemoFor(state *State) *messageDemo {
	if demo := messageDemos[state]; demo != nil {
		return demo
	}
	demo := &messageDemo{state: state}
	demo.action = demo.actions
	messageDemos[state] = demo
	return demo
}

// actions declares the copy control; the click changes the caller-owned state.
func (d *messageDemo) actions(c ui.C) {
	label := d.state.Text("复制", "Copy")
	if d.state.values[0] != 0 {
		label = d.state.Text("已复制", "Copied")
	}
	if ui.Button(c, label).Ghost().Small().Clicked() {
		d.state.values[0] = 1
	}
}
