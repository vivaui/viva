package componentdemo

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/metriccard"
)

var metricCardComponent = Component{
	ID: "metric-card", Title: "Metric Card", Category: "Data Display", Summary: "标题、数值和趋势组成的仪表盘指标卡。",
	paint: (*State).paintMetricCard,
	Examples: []Example{
		{ID: "default", Title: "单项指标", Description: "生产 MetricCard 展示当前值、正向趋势、周期说明和图标。", Snippet: `// bounds is the dashboard slot reserved for the card.
components.MetricCard(canvas, theme, bounds, "Active sessions",
    "12,840", "+18.2% from last week", true)`, PageSnippet: `metriccard.View(c, "Active sessions", metriccard.New().
    Title("Active sessions").Value("12,840").
    Trend("+18.2% from last week").Up().
    Description("Compared with the previous 7 days").
    Icon(m.metricIcon)).
    FillWidth().Height(150)`, PageSnippetZh: `metriccard.View(c, "Active sessions", metriccard.New().
    Title("Active sessions").Value("12,840").
    Trend("+18.2% from last week").Up().
    Description("Compared with the previous 7 days").
    Icon(m.metricIcon)).
    FillWidth().Height(150)`},
		{ID: "trends", Title: "趋势对比", Description: "正向和负向趋势并列展示。", Snippet: `// left and right are equal-width slots in the page's row.
// true paints the trend in Success; false paints it in Danger.
components.MetricCard(canvas, theme, left, "Revenue", "$48.2K", "+12.4%", true)
components.MetricCard(canvas, theme, right, "Refunds", "$1.3K", "+4.1%", false)`, PageSnippet: `ui.Row(c).FillWidth().Gap(16).Children(func() {
    metriccard.View(c, "Revenue", metriccard.New().
        Title("Revenue").Value("$48.2K").Trend("+12.4%").Up()).
        Grow(1).Height(120)
    metriccard.View(c, "Refunds", metriccard.New().
        Title("Refunds").Value("$1.3K").Trend("+4.1%").Down()).
        Grow(1).Height(120)
})`, PageSnippetZh: `ui.Row(c).FillWidth().Gap(16).Children(func() {
    metriccard.View(c, "Revenue", metriccard.New().
        Title("Revenue").Value("$48.2K").Trend("+12.4%").Up()).
        Grow(1).Height(120)
    metriccard.View(c, "Refunds", metriccard.New().
        Title("Refunds").Value("$1.3K").Trend("+4.1%").Down()).
        Grow(1).Height(120)
})`},
	},
}

// metricTrendIcon is Lucide's trending-up arrow, compiled to the gfx.Icon form.
var metricTrendIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(3, 17), gfx.LineTo(9, 11), gfx.LineTo(13, 15), gfx.LineTo(21, 7),
	gfx.MoveTo(15, 7), gfx.LineTo(21, 7), gfx.LineTo(21, 13),
}}

func (state *State) paintMetricCard(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	// Two cards share a row when the page is wide enough for both; otherwise
	// the second example stacks them.
	wide := state.exampleID == "trends" && min(size.Width-48, float32(600)) >= 480
	state.drawPage(canvas, theme, size, 600, state.Text("Metric Card · 关键指标", "Metric Card · Key metrics"), func(c ui.C) {
		switch {
		case wide:
			ui.Row(c).FillWidth().Gap(16).Children(func() {
				metriccard.View(c, "Revenue", metriccard.New().
					Title("Revenue").Value("$48.2K").Trend("+12.4%").Up()).
					Grow(1).Height(120)
				metriccard.View(c, "Refunds", metriccard.New().
					Title("Refunds").Value("$1.3K").Trend("+4.1%").Down()).
					Grow(1).Height(120)
			})
		case state.exampleID == "trends":
			metriccard.View(c, "Active sessions", metriccard.New().
				Title("Active sessions").Value("12,840").
				Trend("+18.2% from last week").Up()).
				FillWidth().Height(120)
			metriccard.View(c, "Error rate", metriccard.New().
				Title("Error rate").Value("0.42%").Trend("-0.08% improvement").Up()).
				FillWidth().Height(120)
		default:
			metriccard.View(c, "Active sessions", metriccard.New().
				Title("Active sessions").Value("12,840").
				Trend("+18.2% from last week").Up().
				Description("Compared with the previous 7 days").
				Icon(metricTrendIcon)).
				FillWidth().Height(150)
		}
	})
}
