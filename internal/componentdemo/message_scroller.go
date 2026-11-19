package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/messagescroller"
)

var messageScrollerComponent = Component{
	ID: "message-scroller", Title: "Message Scroller", Category: "Communication", Summary: "外部控制偏移的消息视口、滚动条与跳转按钮。",
	paint:      (*State).paintMessageScroller,
	initialize: (*State).initializeMessageScroller,
	scroll:     (*State).scrollMessageScroller,
	Examples: []Example{
		{ID: "default", Title: "消息流", Description: "点击较早/较晚按钮或滚动滚轮，视图把新偏移交给调用方。", Snippet: `stream := &messagescroller.Scroller{
    Offset: m.offset,
    EarlierLabel: "Earlier messages", LaterLabel: "Later messages",
    Content: func(c ui.C) {
        for _, label := range [...]string{
            "Project started", "Build complete", "Review requested",
            "Changes approved", "Release ready",
        } {
            ui.Row(c).Height(62).PadX(12).Items(ui.Center).
                Radius(c.Theme().Metrics.RadiusSurface).
                Background(c.Theme().Muted).Children(func() {
                ui.Text(c, label).FontSize(c.Theme().Metrics.FontSizeM)
            })
        }
    },
}
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "Message stream", stream).FillWidth().Height(278)`, PageSnippet: `stream := &m.scroller
stream.Offset = m.offset
stream.EarlierLabel = "Earlier messages"
stream.LaterLabel = "Later messages"
stream.Content = func(c ui.C) {
    ui.Column(c).Gap(12).Children(func() {
        for _, label := range [...]string{
            "Project started", "Build complete", "Review requested",
            "Changes approved", "Release ready",
        } {
            ui.Row(c).Height(62).PadX(12).Items(ui.Center).
                Radius(c.Theme().Metrics.RadiusSurface).
                Background(c.Theme().Muted).Children(func() {
                ui.Text(c, label).FontSize(c.Theme().Metrics.FontSizeM)
            })
        }
    })
}
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "Message stream", stream).FillWidth().Height(278)`, PageSnippetZh: `stream := &m.scroller
stream.Offset = m.offset
stream.EarlierLabel = "更早的消息"
stream.LaterLabel = "更晚的消息"
stream.Content = func(c ui.C) {
    ui.Column(c).Gap(12).Children(func() {
        for _, label := range [...]string{
            "项目已启动", "构建完成", "请求审查", "更改已批准", "准备发布",
        } {
            ui.Row(c).Height(62).PadX(12).Items(ui.Center).
                Radius(c.Theme().Metrics.RadiusSurface).
                Background(c.Theme().Muted).Children(func() {
                ui.Text(c, label).FontSize(c.Theme().Metrics.FontSizeM)
            })
        }
    })
}
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "消息流", stream).FillWidth().Height(278)`},
		{ID: "end", Title: "接近末尾", Description: "初始偏移为 72；较早/较晚按钮都可用，每次把调用方持有的偏移移动 72 像素。", Snippet: `stream := &messagescroller.Scroller{
    Offset: 72, // near the end of the content
    EarlierLabel: "Earlier messages", LaterLabel: "Later messages",
    Content: func(c ui.C) { /* the same five rows */ },
}
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "Message stream", stream).FillWidth().Height(278)`, PageSnippet: `stream := &m.scroller
stream.Offset = 72 // near the end of the content
stream.EarlierLabel = "Earlier messages"
stream.LaterLabel = "Later messages"
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "Message stream", stream).FillWidth().Height(278)`, PageSnippetZh: `stream := &m.scroller
stream.Offset = 72 // 接近内容末尾
stream.EarlierLabel = "更早的消息"
stream.LaterLabel = "更晚的消息"
stream.OnScroll = func(offset float32) { m.offset = offset }
stream.OnJump = func(direction messagescroller.Direction, end float32) {
    if direction == messagescroller.Earlier {
        m.offset = max(float32(0), m.offset-72)
    } else {
        m.offset = min(end, m.offset+72)
    }
}
messagescroller.View(c, "消息流", stream).FillWidth().Height(278)`},
	},
}

var (
	messageScrollerUI    = tree.Feature("message-scroller")
	messageScrollerBar   = tree.ID(messageScrollerUI, "scrollbar")
	messageScrollerStart = tree.ID(messageScrollerUI, "earlier")
	messageScrollerEnd   = tree.ID(messageScrollerUI, "later")
)

// messageScrollerDemo is one stream example's page control. Its callbacks are
// made once and point at this value, so it is kept beside the State rather than
// rebuilt in each paint.
type messageScrollerDemo struct {
	state *State
	view  messagescroller.Scroller
}

// messageScrollerDemos is keyed by State because the constrained demo model has
// no field for this example; a State is painted on one thread at a time.
var messageScrollerDemos = map[*State]*messageScrollerDemo{}

func (state *State) initializeMessageScroller() {
	messageScrollerDemoFor(state)
	if state.exampleID == "end" {
		// Open near the end of the five rows; the earlier control is available and
		// the later one steps the rest of the way.
		state.values[0] = 72
	}
}

func messageScrollerDemoFor(state *State) *messageScrollerDemo {
	if demo := messageScrollerDemos[state]; demo != nil {
		return demo
	}
	demo := &messageScrollerDemo{state: state}
	demo.view.Content = demo.content
	demo.view.OnScroll = demo.scroll
	demo.view.OnJump = demo.jump
	messageScrollerDemos[state] = demo
	return demo
}

func (state *State) paintMessageScroller(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := messageScrollerDemoFor(state)
	demo.view.Offset = float32(state.values[0])
	demo.view.EarlierLabel = state.Text("更早的消息", "Earlier messages")
	demo.view.LaterLabel = state.Text("更晚的消息", "Later messages")
	demo.view.EarlierID = messageScrollerStart
	demo.view.LaterID = messageScrollerEnd
	demo.view.ScrollbarID = messageScrollerBar
	demo.view.ScrollbarLabel = state.Text("消息位置", "Message position")
	demo.view.ScrollbarValue = state.Text("会话", "Conversation")
	state.drawPage(canvas, theme, size, 560, state.Text("Message Scroller · 受控偏移", "Message Scroller · Controlled offset"), func(c ui.C) {
		messagescroller.View(c, state.Text("消息流", "Message stream"), &demo.view).
			FillWidth().Height(min(278, max(size.Height-84, 180)))
	})
}

// content paints the caller-owned rows inside the scroll area.
func (d *messageScrollerDemo) content(c ui.C) {
	look := c.Theme()
	labels := [...]string{
		d.state.Text("项目已启动", "Project started"),
		d.state.Text("构建完成", "Build complete"),
		d.state.Text("请求审查", "Review requested"),
		d.state.Text("更改已批准", "Changes approved"),
		d.state.Text("准备发布", "Release ready"),
	}
	ui.Column(c).Gap(look.Metrics.GapItem).Children(func() {
		for _, label := range labels {
			ui.Row(c).Height(62).PadX(12).Items(ui.Center).
				Radius(look.Metrics.RadiusSurface).Background(look.Muted).Children(func() {
				ui.Text(c, label).FontSize(look.Metrics.FontSizeM)
			})
		}
	})
}

// scroll stores the offset the reader moved to.
func (d *messageScrollerDemo) scroll(offset float32) {
	d.state.values[0] = int32(offset + 0.5)
}

// jump steps toward the end an earlier/later control asked for; the view hands
// it the target offset, which caps a later step at the end of the content.
func (d *messageScrollerDemo) jump(direction messagescroller.Direction, target float32) {
	offset := d.state.values[0]
	switch direction {
	case messagescroller.Earlier:
		offset = max(0, offset-72)
	case messagescroller.Later:
		offset = min(int32(target), offset+72)
	}
	d.state.values[0] = offset
}

// scrollMessageScroller routes the wheel over the stream to its ScrollState.
func (state *State) scrollMessageScroller(event gfx.ScrollEvent) gfx.EventResult {
	demo := messageScrollerDemoFor(state)
	if !demo.view.Contains(event.X, event.Y) {
		return gfx.EventResult{}
	}
	return demo.view.Wheel(event)
}
