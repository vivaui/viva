package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/chart"
)

var chartComponent = Component{
	ID: "chart", Title: "Chart", Category: "Data Display", Summary: "生产图表容器、图例和提示框配合调用方数据几何。",
	paint: (*State).paintChart, initialize: (*State).initializeChart,
	Examples: []Example{
		{ID: "bars", Title: "提交次数", Description: "图表组件划分绘图区，柱形由调用方绘制。",
			Snippet: `layout := components.PaintChartContainer(canvas, theme, bounds, series,
    components.ChartContainerOptions{Legend: components.ChartLegendBottom})
plot := layout.PlotBounds
for i, scale := range []float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62} {
    y := plot.Y + plot.Height*(1-scale)
    bar := gfx.Rect{X: plot.X + float32(i)*44, Y: y, Width: 30, Height: plot.Height * scale}
    canvas.Rounded(bar, 3, series[i%2].Color)
}`,
			PageSnippet: `palette := c.Theme()
series := []chart.Series{
    {Key: "native", Label: "Native", Color: palette.Chart[0]},
    {Key: "web", Label: "Web", Color: palette.Chart[1]},
}
heights := [...]float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62}
// Set once, when the model is created; ContainerView calls it every paint.
m.commits.Draw = func(d *ui.DrawContext, layout chart.Layout) {
    plot := layout.PlotBounds
    baseline := plot.Y + plot.Height - 14
    barWidth := min(float32(30), (plot.Width-40)/8)
    gap := (plot.Width - barWidth*8) / 9
    for index, scale := range heights {
        x := plot.X + gap + float32(index)*(barWidth+gap)
        height := (plot.Height - 34) * scale
        d.Canvas.Rounded(gfx.Rect{X: x, Y: baseline - height, Width: barWidth, Height: height},
            3, series[index%2].Color)
    }
    d.Canvas.Fill(gfx.Rect{X: plot.X, Y: baseline, Width: plot.Width, Height: 1}, d.Theme.Border)
}
m.commits.Series = series
m.commits.Options = chart.ContainerOptions{Legend: chart.LegendBottom}
chart.ContainerView(c, "Commits", &m.commits).Height(244)`,
			PageSnippetZh: `palette := c.Theme()
series := []chart.Series{
    {Key: "native", Label: "原生", Color: palette.Chart[0]},
    {Key: "web", Label: "网页", Color: palette.Chart[1]},
}
heights := [...]float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62}
// 创建模型时设置一次；ContainerView 在每次绘制时调用它。
m.commits.Draw = func(d *ui.DrawContext, layout chart.Layout) {
    plot := layout.PlotBounds
    baseline := plot.Y + plot.Height - 14
    barWidth := min(float32(30), (plot.Width-40)/8)
    gap := (plot.Width - barWidth*8) / 9
    for index, scale := range heights {
        x := plot.X + gap + float32(index)*(barWidth+gap)
        height := (plot.Height - 34) * scale
        d.Canvas.Rounded(gfx.Rect{X: x, Y: baseline - height, Width: barWidth, Height: height},
            3, series[index%2].Color)
    }
    d.Canvas.Fill(gfx.Rect{X: plot.X, Y: baseline, Width: plot.Width, Height: 1}, d.Theme.Border)
}
m.commits.Series = series
m.commits.Options = chart.ContainerOptions{Legend: chart.LegendBottom}
chart.ContainerView(c, "提交次数", &m.commits).Height(244)`},
		{ID: "tooltip", Title: "数据提示", Description: "在同一系列元数据上绘制生产 Tooltip。",
			Snippet: `layout := components.PaintChartContainer(canvas, theme, bounds, series,
    components.ChartContainerOptions{Legend: components.ChartLegendBottom})
plot := layout.PlotBounds
rows := []components.ChartTooltipRow{{Series: 0, Value: "42"}, {Series: 1, Value: "28"}}
anchor := gfx.Rect{X: plot.X + plot.Width - 150, Y: plot.Y + 10}
components.PaintChartTooltip(canvas, theme, anchor, series, rows,
    components.ChartTooltipOptions{
        Label: "Week 4", Indicator: components.ChartTooltipDot, Layer: 4,
    })`,
			PageSnippet: `palette := c.Theme()
series := []chart.Series{
    {Key: "native", Label: "Native", Color: palette.Chart[0]},
    {Key: "web", Label: "Web", Color: palette.Chart[1]},
}
m.commits.Series = series
m.commits.Options = chart.ContainerOptions{Legend: chart.LegendBottom}
m.commitsTip = chart.Tooltip{
    Rows: []chart.TooltipRow{{Series: 0, Value: "42"}, {Series: 1, Value: "28"}},
    Options: chart.TooltipOptions{Label: "Week 4", Indicator: chart.TooltipDot},
}
m.commits.Tooltip = &m.commitsTip
chart.ContainerView(c, "Commits", &m.commits).Height(244)`,
			PageSnippetZh: `palette := c.Theme()
series := []chart.Series{
    {Key: "native", Label: "原生", Color: palette.Chart[0]},
    {Key: "web", Label: "网页", Color: palette.Chart[1]},
}
m.commits.Series = series
m.commits.Options = chart.ContainerOptions{Legend: chart.LegendBottom}
m.commitsTip = chart.Tooltip{
    Rows: []chart.TooltipRow{{Series: 0, Value: "42"}, {Series: 1, Value: "28"}},
    Options: chart.TooltipOptions{Label: "第 4 周", Indicator: chart.TooltipDot},
}
m.commits.Tooltip = &m.commitsTip
chart.ContainerView(c, "提交次数", &m.commits).Height(244)`},
	},
}

func (state *State) paintChart(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.chart
	native, web := "原生", "网页"
	title, label := "Chart · 最近四周提交", "提交次数"
	if state.english {
		native, web = "Native", "Web"
		title, label = "Chart · Last four weeks", "Commits"
	}
	demo.series = [2]chart.Series{
		{Key: "native", Label: native, Color: theme.Chart[0]},
		{Key: "web", Label: web, Color: theme.Chart[1]},
	}
	demo.container.Series = demo.series[:]
	demo.tooltip.Options = chart.TooltipOptions{
		Label: state.Text("第 4 周", "Week 4"), Indicator: chart.TooltipDot,
	}
	if state.exampleID == "tooltip" {
		demo.container.Tooltip = &demo.tooltip
	} else {
		demo.container.Tooltip = nil
	}
	ring := theme.Foreground
	ring.Alpha *= 0.10
	state.drawPage(canvas, theme, size, 600, title, func(c ui.C) {
		ui.Column(c).FillWidth().Pad(components.CardSpacing(theme, components.CardSizeSmall)).
			Background(theme.Card).Border(1, ring).Radius(theme.Metrics.RadiusSurface + 4).Shadow(gfx.ShadowXS).
			Children(func() {
				chart.ContainerView(c, label, &demo.container).Height(244)
			})
	})
}
