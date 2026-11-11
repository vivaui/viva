package componentdemo

import (
	"math"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var chartAreaComponent = Component{
	ID: "chart-area", Title: "Area Chart", Category: "Data Display", Summary: "由类型化切片绘制的面积图与折线图，含坐标轴、网格、堆叠和悬停提示。",
	paint: (*State).paintChartArea, pointer: (*State).pointerChart, key: (*State).keyChart,
	initialize: (*State).initializeChart,
	Examples: []Example{
		{ID: "area-style", Title: "自定义渐变填充", Description: "每个序列单独配置填充透明度和渐变端点，折线与图例颜色保持独立。", Height: 320, Snippet: `transparent := theme.Chart[0]
transparent.Alpha = 0
series[0].AreaStyle = &components.ChartAreaStyle{
    Opacity: .24,
    Gradient: &components.ChartAreaGradient{
        From: theme.Chart[0],
        To: transparent,
    },
}
chart := components.AreaChart{
    Series: series, Values: values,
    Curve: components.ChartCurveMonotone,
    Fill: components.ChartFillGradient,
}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "default", Title: "渐变面积", Description: "单序列、平滑曲线和从序列色渐隐的填充。", Height: 320, Snippet: `chart := components.AreaChart{
    Series: []components.ChartSeries{{Key: "desktop", Label: "Desktop", Color: theme.Chart[0]}},
    Values: [][]float32{desktop},
    Curve:  components.ChartCurveMonotone,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
}
plot := components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "linear", Title: "折线与数据点", Description: "无填充的折线图，标出每个数据点。", Height: 320, Snippet: `chart := components.AreaChart{
    Series: series, Values: values,
    Fill:   components.ChartFillNone,
    Dots:   true,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridBoth,
    Legend: components.ChartLegendBottom,
}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "step", Title: "阶梯曲线", Description: "每个值保持到相邻两点的中点。", Height: 320, Snippet: `chart := components.AreaChart{
    Series: series, Values: values,
    Curve:  components.ChartCurveStep,
    Fill:   components.ChartFillSolid,
    XAxis:  components.ChartAxis{Format: formatMonth},
}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "stacked", Title: "堆叠", Description: "按序列顺序自下而上堆叠，纵轴随总量缩放。", Height: 320, Snippet: `chart := components.AreaChart{
    Series: series, Values: values,
    Curve:  components.ChartCurveNatural,
    Stack:  components.ChartStackStacked,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
    Legend: components.ChartLegendBottom,
}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "expanded", Title: "百分比堆叠", Description: "每个横坐标归一到 100%，纵轴以百分比标注。", Height: 320, Snippet: `chart := components.AreaChart{
    Series: series, Values: values,
    Stack:  components.ChartStackExpand,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "interactive", Title: "悬停与键盘", Description: "指针或方向键移动悬停点，提示框按边缘自动翻转。", Height: 320, Snippet: `plot := components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)
// From the pointer: state.Hover(plot.IndexAt(event.X)), and state.Clear() on exit.
// From the keyboard:
if index, ok := components.ChartStepForKey(event.Key, current, plot.Count()); ok {
    state.Hover(index)
}`},
	},
}

var chartBarComponent = Component{
	ID: "chart-bar", Title: "Bar Chart", Category: "Data Display", Summary: "分组与堆叠柱状图，与面积图共用坐标轴、状态和悬停模型。",
	paint: (*State).paintChartBar, pointer: (*State).pointerChart, key: (*State).keyChart,
	initialize: (*State).initializeChart,
	Examples: []Example{
		{ID: "default", Title: "分组柱状图", Description: "同一类别的序列并排排列，顶端圆角。", Height: 320, Snippet: `chart := components.BarChart{
    Series: series, Values: values,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Grid:   components.ChartGridHorizontal,
    Legend: components.ChartLegendBottom,
}
plot := components.PaintBarChart(canvas, theme, bounds, chartID, chart, &state)`},
		{ID: "stacked", Title: "堆叠柱状图", Description: "同一类别的序列叠成一根柱，只有最上端圆角。", Height: 320, Snippet: `chart := components.BarChart{
    Series: series, Values: values,
    Stack:  components.ChartStackStacked,
    XAxis:  components.ChartAxis{Format: formatMonth},
    Legend: components.ChartLegendBottom,
}
components.PaintBarChart(canvas, theme, bounds, chartID, chart, &state)`},
	},
}

var chartDownsampleComponent = Component{
	ID: "chart-downsample", Title: "Chart Downsample", Category: "Data Display", Summary: "把十万点的历史曲线按像素预算降采样后再绘制。",
	paint: (*State).paintChartDownsample, pointer: (*State).pointerChart, key: (*State).keyChart,
	initialize: (*State).initializeChart,
	Examples: []Example{
		{ID: "default", Title: "最大三角形降采样", Description: "保留首尾和每个桶中形状最突出的点，毛刺不会被抹平。", Height: 320, Snippet: `// Reduce once when the data changes, then draw the reduced series every frame.
keep = components.ChartDownsample(keep[:0], nil, samples, 400)
for i, index := range keep {
    reduced[i] = samples[index]
    x[i] = float32(index)
}
chart := components.AreaChart{Series: series, X: x[:len(keep)], Values: [][]float32{reduced[:len(keep)]}}
components.PaintAreaChart(canvas, theme, bounds, chartID, chart, &state)`},
	},
}

var (
	chartDemoUI   = tree.Feature("chart-demo")
	chartDemoPlot = tree.ID(chartDemoUI, "plot")
)

// The months and visitor counts of shadcn's area chart examples.
var (
	chartDemoMonths  = [...]string{"January", "February", "March", "April", "May", "June"}
	chartDemoDesktop = []float32{186, 305, 237, 73, 209, 214}
	chartDemoMobile  = []float32{80, 200, 120, 190, 130, 140}
)

// chartDemoState holds what a chart needs between paints: the hovered point, the
// last plot (to map pointer positions) and, for the downsample example, the
// 100,000 samples and their reduction.
type chartDemoState struct {
	areaStyle    components.ChartAreaStyle
	areaGradient components.ChartAreaGradient
	chart        components.ChartState
	plot         components.ChartPlot
	samples      []float32
	keep         []int
	x, y         []float32
	// series and values are refilled by every paint, so the chart declaration
	// itself allocates nothing.
	series [2]components.ChartSeries
	values [2][]float32
}

func (state *State) initializeChart() {
	demo := &chartDemoState{}
	if state.componentID == "chart-downsample" {
		const count = 100_000
		demo.samples = make([]float32, count)
		for i := range demo.samples {
			t := float64(i)
			demo.samples[i] = float32(40*math.Sin(t/9000) + 15*math.Sin(t/700) + 4*math.Sin(t/37))
		}
		for _, spike := range [...]int{23_000, 61_500, 88_800} {
			demo.samples[spike] += 60
		}
		demo.keep = components.ChartDownsample(nil, nil, demo.samples, 400)
		demo.x = make([]float32, len(demo.keep))
		demo.y = make([]float32, len(demo.keep))
		for i, index := range demo.keep {
			demo.x[i] = float32(index)
			demo.y[i] = demo.samples[index]
		}
	}
	state.chart = demo
}

func chartDemoMonth(dst []byte, v float32) []byte {
	if index := int(v); index >= 0 && index < len(chartDemoMonths) {
		return append(dst, chartDemoMonths[index][:3]...)
	}
	return dst
}

// fill points the demo's series and values at the month data in the theme's colors.
func (demo *chartDemoState) fill(theme gfx.Theme, english bool) {
	desktop, mobile := "桌面端", "移动端"
	if english {
		desktop, mobile = "Desktop", "Mobile"
	}
	demo.series = [2]components.ChartSeries{
		{Key: "desktop", Label: desktop, Color: theme.Chart[0]},
		{Key: "mobile", Label: mobile, Color: theme.Chart[1]},
	}
	demo.values = [2][]float32{chartDemoDesktop, chartDemoMobile}
}

func (state *State) chartFrame(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size, title string, paint func(bounds gfx.Rect)) {
	var card ui.Box
	state.drawPage(canvas, theme, size, 640, title, func(c ui.C) {
		card = ui.Slot(c).FillWidth().Height(280)
	})
	bounds := card.Rect()
	components.PaintCard(canvas, theme, bounds, components.CardOptions{Size: components.CardSizeSmall})
	paint(gfx.Rect{X: bounds.X + 16, Y: bounds.Y + 14, Width: bounds.Width - 32, Height: bounds.Height - 28})
}

func (state *State) paintChartArea(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.chart
	demo.fill(theme, state.english)
	series := demo.series[:]
	chart := components.AreaChart{
		Series: series, Values: demo.values[:],
		XAxis: components.ChartAxis{Format: chartDemoMonth}, Grid: components.ChartGridHorizontal,
		Label:   state.Text("月访问量面积图", "Monthly visitors area chart"),
		Version: 1,
	}
	title := state.Text("面积图 · 月访问量", "Area chart · Monthly visitors")
	switch state.exampleID {
	case "area-style":
		demo.areaGradient = components.ChartAreaGradient{From: theme.Chart[0], To: theme.Chart[0]}
		demo.areaGradient.To.Alpha = 0
		demo.areaStyle = components.ChartAreaStyle{Opacity: .24, Gradient: &demo.areaGradient}
		series[0].AreaStyle = &demo.areaStyle
		chart.Curve, chart.Legend = components.ChartCurveMonotone, components.ChartLegendBottom
	case "default":
		chart.Series, chart.Values = series[:1], chart.Values[:1]
		chart.Curve = components.ChartCurveMonotone
	case "linear":
		chart.Fill, chart.Dots, chart.Grid, chart.Legend = components.ChartFillNone, true, components.ChartGridBoth, components.ChartLegendBottom
	case "step":
		chart.Curve, chart.Fill = components.ChartCurveStep, components.ChartFillSolid
	case "stacked":
		chart.Curve, chart.Stack, chart.Legend = components.ChartCurveNatural, components.ChartStackStacked, components.ChartLegendBottom
	case "expanded":
		chart.Stack, chart.Curve = components.ChartStackExpand, components.ChartCurveMonotone
	case "interactive":
		chart.Curve, chart.Legend = components.ChartCurveMonotone, components.ChartLegendBottom
	}
	state.chartFrame(canvas, theme, size, title, func(bounds gfx.Rect) {
		demo.plot = components.PaintAreaChart(canvas, theme, bounds, chartDemoPlot, chart, &demo.chart)
	})
}

func (state *State) paintChartBar(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.chart
	demo.fill(theme, state.english)
	chart := components.BarChart{
		Series: demo.series[:], Values: demo.values[:],
		XAxis: components.ChartAxis{Format: chartDemoMonth}, Grid: components.ChartGridHorizontal,
		Legend: components.ChartLegendBottom, Label: state.Text("月访问量柱状图", "Monthly visitors bar chart"),
		Version: 1,
	}
	if state.exampleID == "stacked" {
		chart.Stack = components.ChartStackStacked
	}
	state.chartFrame(canvas, theme, size, state.Text("柱状图 · 月访问量", "Bar chart · Monthly visitors"), func(bounds gfx.Rect) {
		demo.plot = components.PaintBarChart(canvas, theme, bounds, chartDemoPlot, chart, &demo.chart)
	})
}

func (state *State) paintChartDownsample(canvas *gfx.Canvas, theme gfx.Theme, size gfx.Size) {
	demo := state.chart
	label := state.Text("CPU 采样", "CPU samples")
	demo.series[0] = components.ChartSeries{Key: "cpu", Label: label, Color: theme.Chart[2]}
	demo.values[0] = demo.y
	chart := components.AreaChart{
		Series: demo.series[:1],
		X:      demo.x, Values: demo.values[:1],
		Fill: components.ChartFillSolid, Curve: components.ChartCurveLinear, StrokeWidth: 1.5,
		XAxis: components.ChartAxis{Linear: true, Ticks: 6},
		Grid:  components.ChartGridHorizontal, Label: label, Version: 1,
	}
	title := state.Text("降采样 · 100,000 点 → 400 点", "Downsample · 100,000 points → 400")
	state.chartFrame(canvas, theme, size, title, func(bounds gfx.Rect) {
		demo.plot = components.PaintAreaChart(canvas, theme, bounds, chartDemoPlot, chart, &demo.chart)
	})
}

func (state *State) pointerChart(event gfx.PointerEvent) gfx.EventResult {
	demo := state.chart
	if demo == nil || (event.Kind != gfx.PointerMove && event.Kind != gfx.PointerExit) {
		return gfx.EventResult{}
	}
	previous, wasActive := demo.chart.Index()
	if event.Kind == gfx.PointerExit || event.Target != chartDemoPlot {
		demo.chart.Clear()
		return gfx.EventResult{Handled: wasActive, Repaint: wasActive}
	}
	index := demo.plot.IndexAt(event.X)
	demo.chart.Hover(index)
	changed := !wasActive || previous != index
	return gfx.EventResult{Handled: true, Repaint: changed}
}

func (state *State) keyChart(event gfx.KeyEvent) gfx.EventResult {
	demo := state.chart
	if demo == nil || event.Release || demo.plot.Count() == 0 {
		return gfx.EventResult{}
	}
	index, active := demo.chart.Index()
	if !active {
		index = -1
	}
	next, handled := components.ChartStepForKey(event.Key, index, demo.plot.Count())
	if !handled {
		return gfx.EventResult{}
	}
	demo.chart.Hover(next)
	return gfx.EventResult{Handled: true, Repaint: next != index}
}
