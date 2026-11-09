package components

import (
	"github.com/vivaui/viva/gfx"
)

const (
	chartBarGap       float32 = 4 // between bars of one group
	chartBarRadius    float32 = 4
	chartBarGroupFrac float32 = 0.8 // share of a band the bars of a group fill
)

// BarChart is the data and presentation of a bar chart. The data shape is
// AreaChart's: Values[k] holds the heights of Series[k], one per category, and
// X, when set, names the categories' numeric values for the tooltip and the
// axis labels (a nil X uses the category index).
type BarChart struct {
	Series []ChartSeries
	X      []float32
	Values [][]float32

	// Stack piles the series of one category into a single bar instead of
	// placing them side by side.
	Stack ChartStack
	// Radius rounds the free end of each bar; zero is 4.
	Radius float32

	XAxis, YAxis ChartAxis
	Grid         ChartGrid
	Legend       ChartLegendPosition

	Tooltip     ChartTooltipOptions
	HideTooltip bool
	ValueFormat func(dst []byte, v float32) []byte

	Label   string
	Version uint32
}

// PaintBarChart paints a bar chart with the same chrome, state and hover model as
// PaintAreaChart. Each category owns an equal band; the bars of a group sit side
// by side, or pile up when Stack is set.
func PaintBarChart(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, id gfx.ID, chart BarChart, state *ChartState) ChartPlot {
	if canvas == nil {
		return ChartPlot{}
	}
	if state == nil {
		state = &ChartState{}
	}
	bounds = chartResolveBounds(bounds)
	spec := chartSpec{
		series: chart.Series, x: chart.X, values: chart.Values, stack: chart.Stack,
		xAxis: chart.XAxis, yAxis: chart.YAxis, grid: chart.Grid, legend: chart.Legend,
		tooltip: chart.Tooltip, hideTooltip: chart.HideTooltip, valueFormat: chart.ValueFormat,
		label: chart.Label, version: chart.Version, band: true, includeZero: true,
	}
	low, high := spec.domain()
	frame := chartBeginFrame(canvas, theme, bounds, &spec, state, low, high)
	plot := frame.plot.Plot
	if plot.Width >= 1 && plot.Height >= 1 {
		geo := &frame.plot.geo
		index, active := state.Index()
		if active && index < geo.count && !spec.hideTooltip {
			band := plot.Width / float32(geo.count)
			highlight := theme.Muted
			highlight.Alpha *= 0.5
			canvas.Fill(gfx.Rect{X: plot.X + float32(index)*band, Y: plot.Y, Width: band, Height: plot.Height}, highlight)
		}
		chartPaintBars(canvas, chart, &spec, &frame)
	}
	chartFinish(canvas, theme, frame, &spec, state, id)
	return frame.plot
}

func chartPaintBars(canvas *gfx.Canvas, chart BarChart, spec *chartSpec, frame *chartFrame) {
	geo := &frame.plot.geo
	plot := frame.plot.Plot
	band := plot.Width / float32(geo.count)
	group := band * chartBarGroupFrac
	visible := 0
	for series := range spec.values {
		if spec.visible(series) {
			visible++
		}
	}
	if visible == 0 {
		return
	}
	gap := chartBarGap * frame.scale
	radius := chart.Radius
	if radius <= 0 {
		radius = chartBarRadius
	}
	radius *= frame.scale
	slots := visible
	if spec.stack != ChartStackNone {
		slots = 1
	}
	barWidth := (group - gap*float32(slots-1)) / float32(slots)
	if barWidth < 1 {
		barWidth = max(group/float32(slots), 0.5)
		gap = 0
	}
	baseline := geo.py(min(max(0, geo.low), geo.high))
	for index := 0; index < geo.count; index++ {
		left := plot.X + float32(index)*band + (band-group)/2
		slot := 0
		topSeries := -1
		if spec.stack != ChartStackNone {
			for series := range spec.values {
				if spec.visible(series) && spec.value(series, index) == spec.value(series, index) {
					topSeries = series
				}
			}
		}
		for series := range spec.values {
			if !spec.visible(series) {
				continue
			}
			base, top := spec.span(series, index)
			if top != top {
				slot++
				continue
			}
			x := left
			if spec.stack == ChartStackNone {
				x += float32(slot) * (barWidth + gap)
				base = 0
			}
			slot++
			y0, y1 := geo.py(top), geo.py(base)
			if spec.stack == ChartStackNone {
				y1 = baseline
			}
			if y0 > y1 {
				y0, y1 = y1, y0
			}
			height := y1 - y0
			if height <= 0 {
				continue
			}
			rect := gfx.Rect{X: x, Y: y0, Width: barWidth, Height: height}
			color := spec.series[series].Color
			if spec.stack != ChartStackNone && series != topSeries {
				canvas.Fill(rect, color)
				continue
			}
			canvas.Rounded(rect, min(radius, barWidth/2, height), color)
			if spec.stack != ChartStackNone && base != 0 {
				// Only the free end of a stack is rounded: square the end that
				// touches the bar below it.
				squareHeight := min(radius, height)
				canvas.Fill(gfx.Rect{X: x, Y: y1 - squareHeight, Width: barWidth, Height: squareHeight}, color)
			}
		}
	}
}
