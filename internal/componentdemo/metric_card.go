package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var metricCardComponent = Component{
	ID: "metric-card", Title: "Metric Card", Category: "Data Display", Summary: "标题、数值和趋势组成的仪表盘指标卡。",
	paint: (*State).paintMetricCard,
	Examples: []Example{
		{ID: "default", Title: "单项指标", Description: "生产 MetricCard 展示当前值与正向趋势。", Snippet: `// bounds is the dashboard slot reserved for the card.
components.MetricCard(canvas, theme, bounds, "Active sessions",
    "12,840", "+18.2% from last week", true)`},
		{ID: "trends", Title: "趋势对比", Description: "正向和负向趋势并列展示。", Snippet: `// left and right are equal-width slots in the page's row.
// true paints the trend in Success; false paints it in Danger.
components.MetricCard(canvas, theme, left, "Revenue", "$48.2K", "+12.4%", true)
components.MetricCard(canvas, theme, right, "Refunds", "$1.3K", "+4.1%", false)`},
	},
}

func (state *State) paintMetricCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	var cards [2]ui.Box
	// Two cards share a row when the page is wide enough for both; otherwise
	// the second example stacks them.
	wide := state.exampleID == "trends" && min(size.Width-48, float32(600)) >= 480
	state.page(canvas, theme, size, 600, state.Text("Metric Card · 关键指标", "Metric Card · Key metrics"), func(f *ui.Frame) {
		switch {
		case wide:
			f.Row().FillWidth().Gap(16).Children(func() {
				cards[0] = f.Slot().Grow(1).Height(120)
				cards[1] = f.Slot().Grow(1).Height(120)
			})
		case state.exampleID == "trends":
			cards[0] = f.Slot().FillWidth().Height(120)
			cards[1] = f.Slot().FillWidth().Height(120)
		default:
			cards[0] = f.Slot().FillWidth().Height(120)
		}
	})
	switch {
	case wide:
		components.MetricCard(canvas, theme, cards[0].Rect(), "Revenue", "$48.2K", "+12.4%", true)
		components.MetricCard(canvas, theme, cards[1].Rect(), "Refunds", "$1.3K", "+4.1%", false)
	case state.exampleID == "trends":
		components.MetricCard(canvas, theme, cards[0].Rect(), "Active sessions", "12,840", "+18.2% from last week", true)
		components.MetricCard(canvas, theme, cards[1].Rect(), "Error rate", "0.42%", "-0.08% improvement", true)
	default:
		components.MetricCard(canvas, theme, cards[0].Rect(), "Active sessions", "12,840", "+18.2% from last week", true)
	}
}
