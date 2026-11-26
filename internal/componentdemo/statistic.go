package componentdemo

import (
	"math"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/statistic"
)

var statisticComponent = Component{
	ID: "statistic", Title: "Statistic", Category: "Data Display", Summary: "统计数值：标题、大号数字，可设小数位、千分位、前后缀与状态色。",
	Source: "ui/statistic/statistic.go", paint: (*State).paintStatistic,
	Examples: []Example{
		{ID: "default", Title: "数值读数", Description: "只显示数字本身；Precision 定小数位，Group 加千分位，Status 着状态色，NaN 与无穷显示为破折号。", Snippet: `m.sessions = statistic.New()
m.sessions.View(c, statistic.Spec{Title: "Active sessions", Value: m.count, Group: true})
m.latency.View(c, statistic.Spec{Title: "Latency", Value: m.ms, Precision: 1, Suffix: "ms"})`, PageSnippet: `m.sessions = statistic.New()
m.sessions.View(c, statistic.Spec{Title: "Active sessions", Value: m.count, Group: true})
m.latency.View(c, statistic.Spec{Title: "Latency", Value: m.ms, Precision: 1, Suffix: "ms"})`, PageSnippetZh: `m.sessions = statistic.New()
m.sessions.View(c, statistic.Spec{Title: "Active sessions", Value: m.count, Group: true})
m.latency.View(c, statistic.Spec{Title: "Latency", Value: m.ms, Precision: 1, Suffix: "ms"})`},
	},
}

type statisticDemo struct{ sessions, latency, errors, missing *statistic.Statistic }

var statisticDemos = map[*State]*statisticDemo{}

func statisticDemoFor(state *State) *statisticDemo {
	if demo := statisticDemos[state]; demo != nil {
		return demo
	}
	demo := &statisticDemo{statistic.New(), statistic.New(), statistic.New(), statistic.New()}
	statisticDemos[state] = demo
	return demo
}

func (state *State) paintStatistic(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := statisticDemoFor(state)
	state.drawPage(canvas, theme, size, 520, state.Text("Statistic · 统计数值", "Statistic"), func(c ui.C) {
		ui.Row(c).Gap(40).Children(func() {
			demo.sessions.View(c, statistic.Spec{Title: state.Text("活跃会话", "Active sessions"), Value: 12840, Group: true})
			demo.latency.View(c, statistic.Spec{Title: state.Text("延迟", "Latency"), Value: 23.46, Precision: 1, Suffix: " ms", Status: ui.StatusSuccess})
			demo.errors.View(c, statistic.Spec{Title: state.Text("错误率", "Error rate"), Value: 4.2, Precision: 1, Suffix: "%", Status: ui.StatusDanger})
			demo.missing.View(c, statistic.Spec{Title: state.Text("无数据", "No data"), Value: math.NaN()})
		})
	})
}
