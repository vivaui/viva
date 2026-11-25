package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/timeline"
)

var timelineComponent = Component{
	ID: "timeline", Title: "Timeline", Category: "Data Display", Summary: "按时间先后排列的事件，圆点用色表达结果。",
	Source: "ui/timeline/timeline.go", paint: (*State).paintTimeline,
	Examples: []Example{
		{ID: "default", Title: "事件时间线", Description: "圆点颜色表达事件结果；Pending 在末尾补一个空心点表示待办。", Snippet: `// m.events is a []timeline.Item kept in the model.
timeline.View(c, "History", timeline.Spec{
    Items:   m.events,
    Pending: "Waiting for review",
})`, PageSnippet: `// m.events is a []timeline.Item kept in the model.
timeline.View(c, "History", timeline.Spec{
    Items:   m.events,
    Pending: "Waiting for review",
})`, PageSnippetZh: `// m.events 是保存在模型里的 []timeline.Item。
timeline.View(c, "History", timeline.Spec{
    Items:   m.events,
    Pending: "Waiting for review",
})`},
		{ID: "labels", Title: "时间列", Description: "LabelWidth 把时间对齐成一列。", Snippet: `timeline.View(c, "History", timeline.Spec{
    Items:      m.events,
    LabelWidth: 48,
})`, PageSnippet: `timeline.View(c, "History", timeline.Spec{
    Items:      m.events,
    LabelWidth: 48,
})`, PageSnippetZh: `timeline.View(c, "History", timeline.Spec{
    Items:      m.events,
    LabelWidth: 48,
})`},
	},
}

type timelineDemo struct {
	events  []timeline.Item
	pending string
}

var timelineDemos = map[*State]*timelineDemo{}

func timelineDemoFor(state *State) *timelineDemo {
	if demo := timelineDemos[state]; demo != nil {
		return demo
	}
	demo := &timelineDemo{pending: state.Text("等待复核", "Waiting for review"), events: []timeline.Item{
		{Title: state.Text("已连接", "Connected"), Description: state.Text("build-01 接受了密钥", "build-01 accepted the key"), Label: "09:41"},
		{Title: state.Text("已上传 release.tar.gz", "Uploaded release.tar.gz"), Description: "48.2 MB · 3.1 s", Label: "09:42", Tone: timeline.Success},
		{Title: state.Text("重启失败", "Restart failed"), Description: "exit status 1", Label: "09:44", Tone: timeline.Danger},
		{Title: state.Text("已回滚", "Rolled back"), Label: "09:45", Tone: timeline.Muted},
	}}
	timelineDemos[state] = demo
	return demo
}

func (state *State) paintTimeline(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := timelineDemoFor(state)
	state.drawPage(canvas, theme, size, 420, state.Text("Timeline · 时间线", "Timeline · History"), func(c ui.C) {
		spec := timeline.Spec{Items: demo.events, Pending: demo.pending}
		if state.exampleID == "labels" {
			spec = timeline.Spec{Items: demo.events, LabelWidth: 48}
		}
		timeline.View(c, state.Text("历史", "History"), spec).FillWidth()
	})
}
