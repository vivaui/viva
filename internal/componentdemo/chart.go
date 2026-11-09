package componentdemo

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var chartComponent = Component{
	ID: "chart", Title: "Chart", Category: "Data Display", Summary: "生产图表容器、图例和提示框配合调用方数据几何。",
	paint: (*State).paintChart,
	Examples: []Example{
		{ID: "bars", Title: "提交次数", Description: "图表组件划分绘图区，柱形由调用方绘制。", Snippet: `layout := components.PaintChartContainer(canvas, theme, bounds, series,
    components.ChartContainerOptions{Legend: components.ChartLegendBottom})
plot := layout.PlotBounds
for i, scale := range []float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62} {
    y := plot.Y + plot.Height*(1-scale)
    bar := gfx.Rect{X: plot.X + float32(i)*44, Y: y, Width: 30, Height: plot.Height * scale}
    canvas.Rounded(bar, 3, series[i%2].Color)
}`},
		{ID: "tooltip", Title: "数据提示", Description: "在同一系列元数据上绘制生产 Tooltip。", Snippet: `layout := components.PaintChartContainer(canvas, theme, bounds, series,
    components.ChartContainerOptions{Legend: components.ChartLegendBottom})
plot := layout.PlotBounds
rows := []components.ChartTooltipRow{{Series: 0, Value: "42"}, {Series: 1, Value: "28"}}
anchor := gfx.Rect{X: plot.X + plot.Width - 150, Y: plot.Y + 10}
components.PaintChartTooltip(canvas, theme, anchor, series, rows,
    components.ChartTooltipOptions{
        Label: "Week 4", Indicator: components.ChartTooltipDot, Layer: 4,
    })`},
	},
}

var chartDemoSeries = [...]components.ChartSeries{
	{Key: "native", Label: "Native", Color: gfx.Color{Red: 0.20, Green: 0.52, Blue: 0.96, Alpha: 1}},
	{Key: "web", Label: "Web", Color: gfx.Color{Red: 0.53, Green: 0.36, Blue: 0.92, Alpha: 1}},
}

var chartDemoRows = [...]components.ChartTooltipRow{
	{Series: 0, Value: "42"},
	{Series: 1, Value: "28"},
}

func (state *State) paintChart(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	// The card is a slot the page reserves; the plotting inside it is geometry
	// the chart container and this function draw.
	var card ui.Box
	state.page(canvas, theme, size, 600, state.Text("Chart · 最近四周提交", "Chart · Last four weeks"), func(f *ui.Frame) {
		card = f.Slot().FillWidth().Height(244)
	})
	bounds := card.Rect()
	components.PaintCard(canvas, theme, bounds, components.CardOptions{Size: components.CardSizeSmall})
	inner := gfx.Rect{X: bounds.X + 16, Y: bounds.Y + 14, Width: bounds.Width - 32, Height: bounds.Height - 28}
	layout := components.PaintChartContainer(canvas, theme, inner, chartDemoSeries[:], components.ChartContainerOptions{Legend: components.ChartLegendBottom})
	plot := layout.PlotBounds
	baseline := plot.Y + plot.Height - 14
	barWidth := min(float32(30), (plot.Width-40)/8)
	gap := (plot.Width - barWidth*8) / 9
	heights := [...]float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62}
	for index, scale := range heights {
		x := plot.X + gap + float32(index)*(barWidth+gap)
		height := (plot.Height - 34) * scale
		color := chartDemoSeries[index%2].Color
		canvas.Rounded(gfx.Rect{X: x, Y: baseline - height, Width: barWidth, Height: height}, 3, color)
	}
	canvas.Fill(gfx.Rect{X: plot.X, Y: baseline, Width: plot.Width, Height: 1}, theme.Border)
	if state.exampleID == "tooltip" {
		originX := plot.X + plot.Width - 150
		if originX < plot.X {
			originX = plot.X
		}
		components.PaintChartTooltip(canvas, theme, gfx.Rect{X: originX, Y: plot.Y + 10}, chartDemoSeries[:], chartDemoRows[:], components.ChartTooltipOptions{
			Label: state.Text("第 4 周", "Week 4"), Indicator: components.ChartTooltipDot, Layer: 4,
		})
	}
}
