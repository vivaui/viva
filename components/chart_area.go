package components

import (
	"math"
	"strconv"

	"github.com/vivaui/viva/gfx"
)

// ChartStack selects how series combine at one X.
type ChartStack uint8

const (
	// ChartStackNone draws every series from the baseline, overlapping.
	ChartStackNone ChartStack = iota
	// ChartStackStacked piles each series on the ones before it. A missing value
	// counts as zero.
	ChartStackStacked
	// ChartStackExpand stacks and normalizes every X to 100%, so the value axis
	// runs from 0 to 1.
	ChartStackExpand
)

// ChartFill selects how an area chart paints under its lines.
type ChartFill uint8

const (
	// ChartFillGradient fades the series color toward the baseline, the shadcn
	// area chart look. It is the zero value.
	ChartFillGradient ChartFill = iota
	// ChartFillSolid fills with a flat translucent series color.
	ChartFillSolid
	// ChartFillNone draws only the lines: a line chart.
	ChartFillNone
)

const (
	chartLineWidth        float32 = 2
	chartFillOpacity      float32 = ChartAreaDefaultOpacity
	chartGradientTop      float32 = 0.8
	chartGradientBottom   float32 = 0.1
	chartTooltipLayer     uint16  = 4
	chartXAxisHeight      float32 = 28
	chartPlotInsetTop     float32 = 5
	chartPlotInsetSide    float32 = 12
	chartAxisGap          float32 = 8
	chartMinXLabelSpacing float32 = 40
)

// AreaChart is the data and presentation of an area or line chart. It is a value
// the caller fills each paint from its own typed slices: there is no reflection,
// no map of fields and no retained copy of the data.
//
// Series carries the metadata the legend and the tooltip share; Values[k] holds
// the Y values of Series[k], one per X, NaN marking a gap. X is the shared,
// ascending X value of every point; nil spaces the points evenly and uses the
// point index as the X value, the usual category axis whose labels Format names.
type AreaChart struct {
	Series []ChartSeries
	X      []float32
	Values [][]float32

	Curve ChartCurve
	// Stack selects how the series combine at one X. Stacked series must not be
	// negative: a negative value stacks as zero (its tooltip still shows it).
	Stack ChartStack
	Fill  ChartFill
	// Dots marks every data point, as the dotted line chart does.
	Dots bool
	// StrokeWidth is the line width in logical pixels. Zero is 2.
	StrokeWidth float32

	XAxis, YAxis ChartAxis
	Grid         ChartGrid
	Legend       ChartLegendPosition

	// Tooltip configures the hover tooltip; its Label is replaced by the X label
	// of the hovered point and a zero Layer is 4.
	Tooltip     ChartTooltipOptions
	HideTooltip bool
	// ValueFormat writes a tooltip value. Nil writes the shortest exact decimal.
	ValueFormat func(dst []byte, v float32) []byte

	// Label names the plot for assistive technology.
	Label string
	// Version identifies the data and its formatting. Bump it when either
	// changes: the chart caches axis labels and tooltip text per version.
	Version uint32
}

// ChartPlot reports where a chart painted, so the caller can turn a pointer
// position into the hovered data point and place its own annotations.
type ChartPlot struct {
	ChartLayout
	// Plot is the data area inside the axes, where lines and bars are drawn.
	Plot gfx.Rect
	geo  chartGeometry
}

// Count is the number of data points.
func (plot ChartPlot) Count() int { return plot.geo.count }

// IndexAt returns the data point nearest to canvas X, or -1 when there is none.
// Feed it the pointer position to drive ChartState.Hover.
func (plot ChartPlot) IndexAt(x float32) int { return plot.geo.indexAt(x) }

// PointX is the canvas X of data point index.
func (plot ChartPlot) PointX(index int) float32 { return plot.geo.px(index) }

// ValueY is the canvas Y of a Y value.
func (plot ChartPlot) ValueY(value float32) float32 { return plot.geo.py(value) }

// ChartState is the caller-owned state of one chart: the hovered point and the
// caches that keep a steady chart from allocating. Keep it as long as the chart
// lives and pass the same one every paint. A nil state is allowed, and makes the
// chart format its labels and tooltip afresh each paint.
type ChartState struct {
	active  bool
	index   int
	labels  chartLabels
	tooltip chartTooltipText
	scratch chartScratch
	buffer  []byte
	text    []byte
}

// Hover sets the hovered data point; a negative index clears it.
func (state *ChartState) Hover(index int) {
	state.active = index >= 0
	state.index = index
}

// Clear removes the hover.
func (state *ChartState) Clear() { state.Hover(-1) }

// Index is the hovered data point.
func (state *ChartState) Index() (int, bool) { return state.index, state.active }

// ChartStepForKey is a chart's keyboard movement: Left and Right step the hovered
// point, Home and End jump to the ends. With no point hovered (index < 0) the
// first key lands on the first or last. The caller owns the index.
func ChartStepForKey(key gfx.Key, index, count int) (int, bool) {
	if count <= 0 {
		return index, false
	}
	switch key {
	case gfx.KeyLeft:
		if index < 0 {
			return count - 1, true
		}
		return max(index-1, 0), true
	case gfx.KeyRight:
		if index < 0 {
			return 0, true
		}
		return min(index+1, count-1), true
	case gfx.KeyHome:
		return 0, true
	case gfx.KeyEnd:
		return count - 1, true
	}
	return index, false
}

// chartSpec is what every chart kind shares, by value, so the painting code
// reads one shape whichever chart type filled it.
type chartSpec struct {
	series      []ChartSeries
	x           []float32
	values      [][]float32
	stack       ChartStack
	xAxis       ChartAxis
	yAxis       ChartAxis
	grid        ChartGrid
	legend      ChartLegendPosition
	tooltip     ChartTooltipOptions
	hideTooltip bool
	valueFormat func(dst []byte, v float32) []byte
	label       string
	version     uint32
	// band places points at the centers of equal bands, as bars need; otherwise
	// the first point sits on the left edge and the last on the right.
	band        bool
	includeZero bool
}

func (spec *chartSpec) visible(series int) bool {
	return series < len(spec.series) && series < len(spec.values) && !spec.series[series].Hidden
}

func (spec *chartSpec) count() int {
	if spec.x != nil {
		return len(spec.x)
	}
	count := 0
	for series := range spec.values {
		if spec.visible(series) && len(spec.values[series]) > count {
			count = len(spec.values[series])
		}
	}
	return count
}

// value is the raw Y of a series at a point, NaN when it has none.
func (spec *chartSpec) value(series, index int) float32 {
	if series < len(spec.values) && index < len(spec.values[series]) {
		if v := spec.values[series][index]; chartFinite(v) {
			return v
		}
	}
	return float32(math.NaN())
}

func chartFinite(value float32) bool { return value-value == 0 }

// span is the extent a series covers at one point: from the baseline to its value
// when unstacked, between the stack below it and its own top when stacked.
func (spec *chartSpec) span(series, index int) (base, top float32) {
	if spec.stack == ChartStackNone {
		return 0, spec.value(series, index)
	}
	var sum, total float32
	for other := range spec.values {
		if !spec.visible(other) {
			continue
		}
		v := spec.value(other, index)
		if v != v || v < 0 {
			v = 0
		}
		if other == series {
			base = sum
		}
		sum += v
		if other == series {
			top = sum
		}
		total += v
	}
	if spec.stack == ChartStackExpand {
		if total == 0 {
			return 0, 0
		}
		return base / total, top / total
	}
	return base, top
}

// domain is the data extent of the value axis.
func (spec *chartSpec) domain() (low, high float32) {
	if spec.stack == ChartStackExpand {
		return 0, 1
	}
	low, high = float32(math.Inf(1)), float32(math.Inf(-1))
	count := spec.count()
	for index := 0; index < count; index++ {
		var sum float32
		for series := range spec.values {
			if !spec.visible(series) {
				continue
			}
			v := spec.value(series, index)
			if spec.stack == ChartStackNone {
				if v == v {
					low, high = min(low, v), max(high, v)
				}
				continue
			}
			if v != v || v < 0 {
				v = 0
			}
			low, high = min(low, sum, sum+v), max(high, sum, sum+v)
			sum += v
		}
	}
	if spec.includeZero || spec.stack != ChartStackNone {
		low, high = min(low, 0), max(high, 0)
	}
	if low > high {
		return 0, 1
	}
	return low, high
}

// chartGeometry maps data to canvas coordinates.
type chartGeometry struct {
	plot       gfx.Rect
	x          []float32
	count      int
	xMin, xMax float32
	low, high  float32
	band       bool
}

func (geo *chartGeometry) px(index int) float32 {
	if geo.count <= 0 || geo.plot.Width <= 0 {
		return geo.plot.X
	}
	if geo.band {
		return geo.plot.X + (float32(index)+0.5)*geo.plot.Width/float32(geo.count)
	}
	value := float32(index)
	if geo.x != nil && index >= 0 && index < len(geo.x) {
		value = geo.x[index]
	}
	if geo.xMax <= geo.xMin {
		return geo.plot.X + geo.plot.Width/2
	}
	return geo.plot.X + (value-geo.xMin)/(geo.xMax-geo.xMin)*geo.plot.Width
}

func (geo *chartGeometry) py(value float32) float32 {
	if geo.high <= geo.low {
		return geo.plot.Y + geo.plot.Height
	}
	return geo.plot.Y + geo.plot.Height - (value-geo.low)/(geo.high-geo.low)*geo.plot.Height
}

func (geo *chartGeometry) indexAt(x float32) int {
	if geo.count <= 0 || geo.plot.Width <= 0 {
		return -1
	}
	if geo.band {
		index := int(math.Floor(float64((x - geo.plot.X) / (geo.plot.Width / float32(geo.count)))))
		return min(max(index, 0), geo.count-1)
	}
	if geo.xMax <= geo.xMin {
		return 0
	}
	target := geo.xMin + (x-geo.plot.X)/geo.plot.Width*(geo.xMax-geo.xMin)
	if geo.x == nil {
		return min(max(int(math.Round(float64(target))), 0), geo.count-1)
	}
	// Nearest of the ascending X values: binary search for the first at or
	// above the target, then the closer of it and its predecessor.
	low, high := 0, geo.count
	for low < high {
		middle := int(uint(low+high) >> 1)
		if geo.x[middle] < target {
			low = middle + 1
		} else {
			high = middle
		}
	}
	if low >= geo.count {
		return geo.count - 1
	}
	if low > 0 && target-geo.x[low-1] <= geo.x[low]-target {
		return low - 1
	}
	return low
}

// chartLabels caches the formatted axis labels and where they sit.
type chartLabels struct {
	key       chartLabelKey
	valid     bool
	yValues   []float32
	yText     []string
	yWidth    float32
	xFrac     []float32 // tick positions, as a share of the plot width
	xText     []string
	xBuffer   []byte
	yStep     float32
	plotLeft  float32 // offsets from the current plot container, never window coordinates
	plotRight float32
}

// chartLabelKey is every input of a label set. Equal keys give equal labels; the
// Format funcs are the one input it cannot see, so a state must keep its funcs.
type chartLabelKey struct {
	version              uint32
	count                int
	xMin, xMax           float32
	low, high            float32
	yTicks, xTicks       int
	width, height        float32
	fontSize, scale      float32
	hideX, hideY, expand bool
	band                 bool
	gridVertical         bool
	linear               bool
}

// chartTooltipText caches the tooltip text of one hovered point.
type chartTooltipText struct {
	valid       bool
	index       int
	version     uint32
	label       string
	description string
	rows        []ChartTooltipRow
}

// chartFrame is a painted chart's container, axes and grid, ready for its marks.
type chartFrame struct {
	layout ChartLayout
	plot   ChartPlot
	scale  float32
}

// chartBeginFrame paints everything under the data: legend, grid and axis
// labels. dataLow and dataHigh are the extent of the values.
func chartBeginFrame(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, spec *chartSpec, state *ChartState, dataLow, dataHigh float32) chartFrame {
	scale := theme.Metrics.ControlHeight / 32
	fontSize := theme.Metrics.FontSizeS
	rowHeight := 16 * scale
	layout := PaintChartContainer(canvas, theme, bounds, spec.series, ChartContainerOptions{Legend: spec.legend})
	area := layout.PlotBounds

	top := area.Y + chartPlotInsetTop*scale
	bottomInset := chartPlotInsetTop * scale
	if !spec.xAxis.Hidden {
		bottomInset = chartXAxisHeight * scale
	}
	height := area.Y + area.Height - bottomInset - top
	count := spec.count()
	geo := chartGeometry{count: count, x: spec.x, band: spec.band}
	if height < 1 || area.Width < 1 {
		return chartFrame{layout: layout, plot: ChartPlot{ChartLayout: layout, geo: geo}, scale: scale}
	}

	yTicks := spec.yAxis.Ticks
	if yTicks < 2 {
		yTicks = chartTickCount(height, scale)
	}
	low, high := dataLow, dataHigh
	var step float32
	switch {
	case spec.yAxis.Fixed:
		low, high = spec.yAxis.Min, spec.yAxis.Max
		if !(high > low) {
			high = low + 1
		}
		step = float32(chartNiceStep(float64(high-low), yTicks))
	case spec.stack == ChartStackExpand:
		low, high = 0, 1
		step = float32(chartNiceStep(1, yTicks))
	default:
		low, high, step = chartNiceDomain(dataLow, dataHigh, yTicks)
	}

	xMin, xMax := float32(0), float32(max(count-1, 0))
	if spec.x != nil && count > 0 {
		xMin, xMax = spec.x[0], spec.x[count-1]
	}
	if spec.xAxis.Fixed {
		xMin, xMax = spec.xAxis.Min, spec.xAxis.Max
	}

	key := chartLabelKey{
		version: spec.version, count: count, xMin: xMin, xMax: xMax, low: low, high: high,
		yTicks: yTicks, xTicks: spec.xAxis.Ticks, width: area.Width, height: height,
		fontSize: fontSize, scale: scale, hideX: spec.xAxis.Hidden, hideY: spec.yAxis.Hidden,
		expand: spec.stack == ChartStackExpand, band: spec.band,
		gridVertical: spec.grid == ChartGridVertical || spec.grid == ChartGridBoth,
		linear:       spec.xAxis.Linear,
	}
	labels := &state.labels
	if !labels.valid || labels.key != key {
		chartBuildLabels(canvas, spec, labels, key, area, step, fontSize, scale)
	}

	plot := gfx.Rect{X: area.X + labels.plotLeft, Y: top, Width: labels.plotRight - labels.plotLeft, Height: height}
	geo.plot, geo.xMin, geo.xMax, geo.low, geo.high = plot, xMin, xMax, low, high
	if plot.Width < 1 {
		return chartFrame{layout: layout, plot: ChartPlot{ChartLayout: layout, geo: chartGeometry{count: count, x: spec.x, band: spec.band}}, scale: scale}
	}

	gridColor := theme.Border
	gridColor.Alpha *= 0.5
	if spec.grid == ChartGridHorizontal || spec.grid == ChartGridBoth {
		for _, value := range labels.yValues {
			y := geo.py(value)
			canvas.Fill(gfx.Rect{X: plot.X, Y: y - 0.5, Width: plot.Width, Height: 1}, gridColor)
		}
	}
	if labels.key.gridVertical {
		for _, frac := range labels.xFrac {
			x := plot.X + frac*plot.Width
			canvas.Fill(gfx.Rect{X: x - 0.5, Y: plot.Y, Width: 1, Height: plot.Height}, gridColor)
		}
	}
	labelStyle := gfx.TextStyle{Size: fontSize, Color: theme.MutedForeground}
	if !spec.yAxis.Hidden {
		gap := chartAxisGap * scale
		for index, value := range labels.yValues {
			if index >= len(labels.yText) {
				break
			}
			y := geo.py(value)
			canvas.LabelAligned(labels.yText[index], gfx.Rect{X: plot.X - gap - labels.yWidth, Y: y - rowHeight/2, Width: labels.yWidth, Height: rowHeight}, labelStyle, gfx.AlignRight)
		}
	}
	if !spec.xAxis.Hidden {
		for index, frac := range labels.xFrac {
			if index >= len(labels.xText) {
				break
			}
			text := labels.xText[index]
			width := min(canvas.TextWidth(text, fontSize)+2, area.Width)
			// Edge ticks remain anchored to their data, but their labels must
			// fit inside the chart container rather than spill into a neighbor.
			x := min(max(plot.X+frac*plot.Width-width/2, area.X), area.X+area.Width-width)
			text = canvas.FitLine(text, fontSize, gfx.WeightRegular, width)
			canvas.Label(text, gfx.Rect{X: x, Y: plot.Y + plot.Height + chartAxisGap*scale, Width: width, Height: rowHeight}, labelStyle)
		}
	}
	return chartFrame{layout: layout, plot: ChartPlot{ChartLayout: layout, Plot: plot, geo: geo}, scale: scale}
}

// chartBuildLabels formats the tick labels and decides which points get an X
// label. It runs when the inputs change, not on every paint.
func chartBuildLabels(canvas *gfx.Canvas, spec *chartSpec, labels *chartLabels, key chartLabelKey, area gfx.Rect, step, fontSize, scale float32) {
	labels.key, labels.valid = key, true
	labels.yStep = step
	labels.yValues = labels.yValues[:0]
	labels.yText = labels.yText[:0]
	labels.yWidth = 0
	if step > 0 {
		first := float64(math.Ceil(float64(key.low/step) - 1e-4))
		for index := 0; index < 64; index++ {
			value := float32(first+float64(index)) * step
			if value > key.high+step*1e-4 {
				break
			}
			labels.yValues = append(labels.yValues, value)
		}
	}
	if !spec.yAxis.Hidden {
		decimals := chartTickDecimals(step)
		for _, value := range labels.yValues {
			labels.xBuffer = labels.xBuffer[:0]
			switch {
			case spec.yAxis.Format != nil:
				labels.xBuffer = spec.yAxis.Format(labels.xBuffer, value)
			case spec.stack == ChartStackExpand:
				labels.xBuffer = chartAppendPercent(labels.xBuffer, value)
			default:
				labels.xBuffer = chartAppendNumber(labels.xBuffer, value, decimals)
			}
			text := string(labels.xBuffer)
			labels.yText = append(labels.yText, text)
			labels.yWidth = max(labels.yWidth, canvas.TextWidth(text, fontSize))
		}
	}

	labels.plotLeft = chartPlotInsetSide * scale
	if !spec.yAxis.Hidden {
		labels.plotLeft = labels.yWidth + chartAxisGap*scale
	}
	labels.plotRight = area.Width - chartPlotInsetSide*scale
	plotWidth := labels.plotRight - labels.plotLeft

	labels.xFrac = labels.xFrac[:0]
	labels.xText = labels.xText[:0]
	count := key.count
	if count == 0 || plotWidth < 1 || (spec.xAxis.Hidden && !key.gridVertical) {
		return
	}
	appendLabel := func(value float32, decimals int) string {
		labels.xBuffer = labels.xBuffer[:0]
		switch {
		case spec.xAxis.Format != nil:
			labels.xBuffer = spec.xAxis.Format(labels.xBuffer, value)
		case decimals >= 0:
			labels.xBuffer = chartAppendNumber(labels.xBuffer, value, decimals)
		default:
			labels.xBuffer = chartAppendShortest(labels.xBuffer, value)
		}
		return string(labels.xBuffer)
	}
	pointValue := func(index int) float32 {
		if spec.x != nil {
			return spec.x[index]
		}
		return float32(index)
	}
	gap := chartAxisGap * scale

	if spec.xAxis.Linear && !spec.band && key.xMax > key.xMin {
		// Ticks at round values across the domain, not at the data points.
		widest := float32(0)
		if !spec.xAxis.Hidden {
			for _, value := range [3]float32{key.xMin, (key.xMin + key.xMax) / 2, key.xMax} {
				widest = max(widest, canvas.TextWidth(appendLabel(value, -1), fontSize))
			}
		}
		wanted := spec.xAxis.Ticks
		if wanted < 2 {
			wanted = max(int(plotWidth/(widest+2*gap+chartMinXLabelSpacing*scale/2)), 2)
		}
		step := float32(chartNiceStep(float64(key.xMax-key.xMin), wanted-1))
		decimals := chartTickDecimals(step)
		first := float64(math.Ceil(float64(key.xMin/step) - 1e-4))
		for index := 0; index < 256; index++ {
			value := float32(first+float64(index)) * step
			if value > key.xMax+step*1e-4 {
				break
			}
			labels.xFrac = append(labels.xFrac, (value-key.xMin)/(key.xMax-key.xMin))
			if !spec.xAxis.Hidden {
				labels.xText = append(labels.xText, appendLabel(value, decimals))
			}
		}
		return
	}

	fraction := func(index int) float32 {
		switch {
		case spec.band:
			return (float32(index) + 0.5) / float32(count)
		case key.xMax > key.xMin:
			return (pointValue(index) - key.xMin) / (key.xMax - key.xMin)
		}
		return 0.5
	}
	spacing := plotWidth
	if count > 1 {
		spacing = plotWidth / float32(count-1)
		if spec.band {
			spacing = plotWidth / float32(count)
		}
	}
	minimum := chartMinXLabelSpacing * scale
	if !spec.xAxis.Hidden {
		// The widest of the first, middle and last labels stands for them all.
		widest := float32(0)
		for _, index := range [3]int{0, count / 2, count - 1} {
			widest = max(widest, canvas.TextWidth(appendLabel(pointValue(index), -1), fontSize))
		}
		minimum = widest + 2*gap
	}
	stride := 1
	if spec.xAxis.Ticks >= 2 && count > 1 {
		stride = (count - 1 + spec.xAxis.Ticks - 2) / (spec.xAxis.Ticks - 1)
	} else if spacing > 0 && spacing < minimum {
		stride = int(math.Ceil(float64(minimum / spacing)))
	}
	stride = max(stride, 1)
	for index := 0; index < count; index += stride {
		labels.xFrac = append(labels.xFrac, fraction(index))
		if !spec.xAxis.Hidden {
			labels.xText = append(labels.xText, appendLabel(pointValue(index), -1))
		}
	}
}

// chartBuildTooltip formats the tooltip text of one point, once per point and
// data version.
func chartBuildTooltip(spec *chartSpec, state *ChartState, index int) *chartTooltipText {
	cache := &state.tooltip
	if cache.valid && cache.index == index && cache.version == spec.version {
		return cache
	}
	cache.valid, cache.index, cache.version = true, index, spec.version
	value := float32(index)
	if spec.x != nil && index < len(spec.x) {
		value = spec.x[index]
	}
	buffer := state.buffer[:0]
	if spec.xAxis.Format != nil {
		buffer = spec.xAxis.Format(buffer, value)
	} else {
		buffer = chartAppendShortest(buffer, value)
	}
	cache.label = string(buffer)
	description := append(buffer[:0], cache.label...)
	cache.rows = cache.rows[:0]
	for series := range spec.values {
		if !spec.visible(series) {
			continue
		}
		v := spec.value(series, index)
		if v != v {
			continue
		}
		formatted := state.text[:0]
		if spec.valueFormat != nil {
			formatted = spec.valueFormat(formatted, v)
		} else {
			formatted = chartAppendShortest(formatted, v)
		}
		state.text = formatted
		text := string(formatted)
		cache.rows = append(cache.rows, ChartTooltipRow{Series: series, Value: text})
		description = append(description, ", "...)
		description = append(description, spec.series[series].Label...)
		description = append(description, ' ')
		description = append(description, text...)
	}
	cache.description = string(description)
	state.buffer = description[:0]
	return cache
}

// chartFinish registers the plot's hit region and paints the hover tooltip. The
// hovered mark (area line and dots, bar band) is the chart's own business.
func chartFinish(canvas *gfx.Canvas, theme gfx.Theme, frame chartFrame, spec *chartSpec, state *ChartState, id gfx.ID) {
	plot := frame.plot.Plot
	if plot.Width < 1 || plot.Height < 1 {
		return
	}
	count := frame.plot.geo.count
	index, active := state.Index()
	active = active && index < count
	value := ""
	var cache *chartTooltipText
	if active {
		cache = chartBuildTooltip(spec, state, index)
		value = cache.description
	} else if !id.IsZero() && count > 0 && canvas.Focused() == id {
		// A pointer fills the group's value by hovering a point; a keyboard
		// user has no hover, so the focused chart describes its latest point
		// instead of announcing nothing.
		cache = chartBuildTooltip(spec, state, count-1)
		value = cache.description
	}
	label := spec.label
	if label == "" {
		label = "Chart"
	}
	canvas.HitWithOptions(id, plot, label, gfx.HitOptions{Role: gfx.RoleGroup, Cursor: gfx.PointerCursorCrosshair, Value: value})
	if !active || spec.hideTooltip || len(cache.rows) == 0 {
		return
	}
	options := spec.tooltip
	options.Label = cache.label
	if options.Layer == 0 {
		options.Layer = chartTooltipLayer
	}
	size := ChartTooltipSize(canvas, theme, spec.series, cache.rows, options)
	gap := 12 * frame.scale
	x := frame.plot.geo.px(index) + gap
	if x+size.Width > frame.layout.Bounds.X+frame.layout.Bounds.Width {
		x = frame.plot.geo.px(index) - gap - size.Width
	}
	x = max(x, frame.layout.Bounds.X)
	y := min(plot.Y, frame.layout.Bounds.Y+frame.layout.Bounds.Height-size.Height)
	PaintChartTooltip(canvas, theme, gfx.Rect{X: x, Y: max(y, frame.layout.Bounds.Y)}, spec.series, cache.rows, options)
}

// PaintAreaChart paints an area or line chart into bounds, with its legend,
// axes, grid, series, the hover mark and tooltip for state's hovered point, and
// registers the plot as the hit region id. It returns the geometry for turning
// pointer positions into points; the caller feeds the result back through
// ChartState.Hover from its own pointer and key events (ChartStepForKey).
//
// A steady chart allocates nothing per paint once its state has warmed up.
func PaintAreaChart(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, id gfx.ID, chart AreaChart, state *ChartState) ChartPlot {
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
		label: chart.Label, version: chart.Version,
		includeZero: chart.Fill != ChartFillNone,
	}
	low, high := spec.domain()
	frame := chartBeginFrame(canvas, theme, bounds, &spec, state, low, high)
	if frame.plot.Plot.Width >= 1 && frame.plot.Plot.Height >= 1 {
		chartPaintAreaSeries(canvas, theme, &chart, &spec, &frame, state)
		chartPaintAreaCursor(canvas, theme, &spec, &frame, state)
	}
	chartFinish(canvas, theme, frame, &spec, state, id)
	state.scratch.clip()
	return frame.plot
}

// chartNextRun finds the next stretch of consecutive points that have a value,
// from index from; end < start when there is none.
func chartNextRun(spec *chartSpec, series, from, count int) (start, end int) {
	for from < count && spec.value(series, from) != spec.value(series, from) {
		from++
	}
	end = from
	for end < count && spec.value(series, end) == spec.value(series, end) {
		end++
	}
	return from, end - 1
}

func chartPaintAreaSeries(canvas *gfx.Canvas, theme gfx.Theme, chart *AreaChart, spec *chartSpec, frame *chartFrame, state *ChartState) {
	geo := &frame.plot.geo
	plot := frame.plot.Plot
	count := geo.count
	// Fixed domains may deliberately retain a point outside the visible window
	// for interpolation. Neither its stroke nor fill may cover axes or siblings.
	canvas.Clip(plot)
	defer canvas.Unclip()
	scratch := &state.scratch
	stacked := spec.stack != ChartStackNone
	width := chart.StrokeWidth
	if width <= 0 {
		width = chartLineWidth
	}
	width *= frame.scale
	// Series coordinates are local to the plot so one view box covers them all.
	origin := gfx.Point{X: plot.X, Y: plot.Y}
	baseline := geo.py(min(max(0, geo.low), geo.high)) - origin.Y

	for series := range spec.values {
		if !spec.visible(series) {
			continue
		}
		color := spec.series[series].Color
		fillVisible := chart.Fill != ChartFillNone
		if area := spec.series[series].AreaStyle; area != nil {
			fillVisible = fillVisible && chartFinite(area.Opacity) && area.Opacity > 0
		}
		from := 0
		for from < count {
			start, end := from, count-1
			if !stacked {
				start, end = chartNextRun(spec, series, from, count)
			}
			from = end + 1
			if end < start {
				break
			}
			points := chartGrow(scratch.points, end-start+1)
			scratch.points = points
			for index := start; index <= end; index++ {
				_, top := spec.span(series, index)
				points[index-start] = gfx.Point{X: geo.px(index) - origin.X, Y: geo.py(top) - origin.Y}
			}
			if fillVisible && end > start {
				chartPaintAreaFill(canvas, chart, spec, frame, scratch, series, start, end, color, baseline, origin)
			}
			if end > start && width > 0 {
				builder := canvas.PathBuilder()
				builder.MoveTo(points[0].X, points[0].Y)
				chartCurveForward(builder, chart.Curve, points, scratch)
				canvas.DrawPath(builder.Path(plot.Width, plot.Height), plot,
					gfx.PathStyle{Stroke: color, StrokeWidth: width, Cap: gfx.CapRound, Join: gfx.JoinRound})
			}
			if chart.Dots {
				radius := 3 * frame.scale
				for _, point := range points {
					canvas.Rounded(gfx.Rect{X: origin.X + point.X - radius, Y: origin.Y + point.Y - radius, Width: 2 * radius, Height: 2 * radius}, radius, color)
				}
			}
		}
	}
}

func chartPaintAreaFill(canvas *gfx.Canvas, chart *AreaChart, spec *chartSpec, frame *chartFrame, scratch *chartScratch, series, start, end int, color gfx.Color, baseline float32, origin gfx.Point) {
	geo := &frame.plot.geo
	plot := frame.plot.Plot
	points := scratch.points
	view := func(builder *gfx.PathBuilder) gfx.Path { return builder.Path(plot.Width, plot.Height) }
	var gradient gfx.LinearGradient
	areaStyle := spec.series[series].AreaStyle

	if spec.stack != ChartStackNone {
		// A stack's band lies between the curve of the stack below it and its own,
		// both functions of X, and stacked values never go below zero.
		lower := chartGrow(scratch.lower, end-start+1)
		scratch.lower = lower
		top, bottom := float32(math.MaxFloat32), float32(0)
		for index := start; index <= end; index++ {
			base, _ := spec.span(series, index)
			y := geo.py(base) - origin.Y
			lower[index-start] = gfx.Point{X: geo.px(index) - origin.X, Y: y}
			top, bottom = min(top, points[index-start].Y), max(bottom, y)
		}
		builder := canvas.PathBuilder()
		builder.MoveTo(points[0].X, points[0].Y)
		chartCurveForward(builder, chart.Curve, points, scratch)
		last := lower[len(lower)-1]
		builder.LineTo(last.X, last.Y)
		chartCurveReverse(builder, chart.Curve, lower, scratch)
		builder.Close()
		canvas.DrawPath(view(builder), plot, chartAreaFillStyle(&gradient, chart.Fill, color, top, bottom, areaStyle))
		return
	}

	// An unstacked area lies between its curve and the baseline. Where the curve
	// crosses the baseline, closing the curve straight to the baseline would make
	// a self-intersecting outline, so the two sides are filled apart: the region
	// under the curve (closed along the plot's bottom) clipped to above the
	// baseline, and the region over the curve (closed along its top) clipped to
	// below it. Both are simple outlines because the curve is a function of X.
	high, low := float32(-math.MaxFloat32), float32(math.MaxFloat32)
	for _, point := range points {
		high, low = max(high, point.Y), min(low, point.Y)
	}
	if chart.Curve == ChartCurveNatural && len(points) >= 3 {
		// A natural spline can swing past its points; its control points bound it.
		chartControls(chart.Curve, points, scratch)
		for index := range scratch.c1 {
			high = max(high, scratch.c1[index].Y, scratch.c2[index].Y)
			low = min(low, scratch.c1[index].Y, scratch.c2[index].Y)
		}
	}
	const slack = 0.01
	if low < baseline-slack && high > baseline+slack {
		for side := 0; side < 2; side++ {
			edge, extreme := plot.Height, low // fill the region under the curve
			clip := gfx.Rect{X: plot.X, Y: plot.Y, Width: plot.Width, Height: baseline}
			if side == 1 { // fill the region over the curve
				edge, extreme = 0, high
				clip = gfx.Rect{X: plot.X, Y: plot.Y + baseline, Width: plot.Width, Height: plot.Height - baseline}
			}
			builder := canvas.PathBuilder()
			builder.MoveTo(points[0].X, points[0].Y)
			chartCurveForward(builder, chart.Curve, points, scratch)
			builder.LineTo(points[len(points)-1].X, edge)
			builder.LineTo(points[0].X, edge)
			builder.Close()
			canvas.Clip(clip)
			canvas.DrawPath(view(builder), plot, chartAreaFillStyle(&gradient, chart.Fill, color, extreme, baseline, areaStyle))
			canvas.Unclip()
		}
		return
	}
	extreme := low
	if low >= baseline-slack {
		extreme = high // the whole curve is below the baseline: values under zero
	}
	builder := canvas.PathBuilder()
	builder.MoveTo(points[0].X, points[0].Y)
	chartCurveForward(builder, chart.Curve, points, scratch)
	builder.LineTo(points[len(points)-1].X, baseline)
	builder.LineTo(points[0].X, baseline)
	builder.Close()
	canvas.DrawPath(view(builder), plot, chartAreaFillStyle(&gradient, chart.Fill, color, extreme, baseline, areaStyle))
}

// chartAreaFillStyle is the fill of an area whose farthest edge from its baseline
// is at extreme (plot-local Y): a flat translucent color, or a vertical gradient
// from the full area opacity at the extreme to a faint tail at the baseline. The
// gradient is written to the caller's storage so a paint allocates nothing.
func chartAreaFillStyle(gradient *gfx.LinearGradient, fill ChartFill, color gfx.Color, extreme, baseline float32, area *ChartAreaStyle) gfx.PathStyle {
	var style gfx.PathStyle
	opacity := chartFillOpacity
	if area != nil {
		opacity = area.Opacity
		if !chartFinite(opacity) {
			opacity = 0
		}
		opacity = min(max(opacity, 0), 1)
		if area.Color != (gfx.Color{}) {
			color = area.Color
		}
	}
	if fill == ChartFillSolid {
		style.Fill = color
		style.Fill.Alpha *= opacity
		return style
	}
	span := baseline - extreme
	if span > -1 && span < 1 {
		if span < 0 {
			span = -1
		} else {
			span = 1
		}
	}
	from, to := color, color
	from.Alpha *= chartGradientTop
	to.Alpha *= chartGradientBottom
	if area != nil && area.Gradient != nil {
		from, to = area.Gradient.From, area.Gradient.To
	}
	from.Alpha *= opacity
	to.Alpha *= opacity
	*gradient = gfx.LinearGradient{
		Start: gfx.Point{Y: extreme + 0.05*span}, End: gfx.Point{Y: extreme + 0.95*span},
		From: from, To: to,
	}
	style.FillGradient = gradient
	return style
}

// chartPaintAreaCursor draws the hovered point's vertical line and one dot per
// series.
func chartPaintAreaCursor(canvas *gfx.Canvas, theme gfx.Theme, spec *chartSpec, frame *chartFrame, state *ChartState) {
	index, active := state.Index()
	geo := &frame.plot.geo
	if !active || index >= geo.count || spec.hideTooltip {
		return
	}
	plot := frame.plot.Plot
	x := geo.px(index)
	if x < plot.X || x > plot.X+plot.Width {
		return
	}
	canvas.Clip(plot)
	defer canvas.Unclip()
	canvas.Fill(gfx.Rect{X: x - 0.5, Y: plot.Y, Width: 1, Height: plot.Height}, theme.Border)
	radius := 4 * frame.scale
	ring := 2 * frame.scale
	for series := range spec.values {
		if !spec.visible(series) {
			continue
		}
		_, top := spec.span(series, index)
		if top != top {
			continue
		}
		y := geo.py(top)
		canvas.Rounded(gfx.Rect{X: x - radius - ring, Y: y - radius - ring, Width: 2 * (radius + ring), Height: 2 * (radius + ring)}, radius+ring, theme.Background)
		canvas.Rounded(gfx.Rect{X: x - radius, Y: y - radius, Width: 2 * radius, Height: 2 * radius}, radius, spec.series[series].Color)
	}
}

// ChartAxis configures one chart axis. The zero value is a visible axis with an
// automatic domain, an automatic tick count and a plain number format.
type ChartAxis struct {
	// Hidden removes the axis labels. A hidden Y axis still places the grid
	// lines, exactly as Recharts keeps ticks for a hidden axis.
	Hidden bool
	// Fixed pins the domain to [Min, Max]; otherwise it follows the data and, on
	// the value axis, is rounded out to whole ticks.
	Fixed    bool
	Min, Max float32
	// Linear places the X ticks at round values across the domain instead of at
	// the data points, as a numeric axis does. Without it the labels name the
	// points themselves, the category axis a month or a name needs. It has no
	// effect on bar charts, whose categories are always bands.
	Linear bool
	// Ticks is the wanted tick count. Zero picks one from the plot size: on a
	// value axis a nice step, on the X axis every data point that fits.
	Ticks int
	// Format appends the label of value v to dst and returns it, the way
	// strconv.AppendFloat does, so a label costs no allocation of its own. Nil
	// writes the number with just enough decimals for the tick step. On the X
	// axis v is the X value, or the point index when the chart has no X slice.
	Format func(dst []byte, v float32) []byte
}

// ChartGrid selects which grid lines a chart draws at the axis ticks.
type ChartGrid uint8

const (
	ChartGridNone ChartGrid = iota
	ChartGridHorizontal
	ChartGridVertical
	ChartGridBoth
)

// chartDefaultYTicks is the value-axis tick count wanted at the default 200px
// chart height; shorter plots ask for fewer.
const chartDefaultYTicks = 5

// chartNiceStep is the 1, 2, 5 times a power of ten step that gives about count
// intervals over span.
func chartNiceStep(span float64, count int) float64 {
	if !(span > 0) || count < 1 || math.IsInf(span, 0) {
		return 1
	}
	raw := span / float64(count)
	exponent := math.Floor(math.Log10(raw))
	scale := math.Pow(10, exponent)
	fraction := raw / scale
	switch {
	case fraction <= 1.5:
		fraction = 1
	case fraction <= 3:
		fraction = 2
	case fraction <= 7:
		fraction = 5
	default:
		fraction = 10
	}
	return fraction * scale
}

// chartNiceDomain rounds [low, high] out to whole multiples of the nice step for
// about count ticks and returns the rounded domain with that step. A flat domain
// is opened up so the axis never collapses.
func chartNiceDomain(low, high float32, count int) (float32, float32, float32) {
	lo, hi := float64(low), float64(high)
	if !(hi >= lo) || math.IsNaN(lo) || math.IsNaN(hi) {
		return 0, 1, 1
	}
	if hi == lo {
		if lo == 0 {
			return 0, 1, 1
		}
		delta := math.Abs(lo) * 0.5
		lo, hi = lo-delta, hi+delta
	}
	step := chartNiceStep(hi-lo, count)
	lo = math.Floor(lo/step) * step
	hi = math.Ceil(hi/step) * step
	return float32(lo), float32(hi), float32(step)
}

// chartTickCount is how many value-axis ticks fit a plot of the given height.
func chartTickCount(plotHeight, scale float32) int {
	count := int(plotHeight / (40 * scale))
	if count > chartDefaultYTicks {
		count = chartDefaultYTicks
	}
	if count < 2 {
		count = 2
	}
	return count
}

// chartTickDecimals is how many decimals a tick step needs to print exactly.
func chartTickDecimals(step float32) int {
	if !(step > 0) {
		return 0
	}
	decimals := 0
	for value := float64(step); decimals < 6 && math.Abs(value-math.Round(value)) > 1e-4*math.Max(1, math.Abs(value)); decimals++ {
		value *= 10
	}
	return decimals
}

// chartAppendNumber is the default tick label: fixed decimals, no "-0".
func chartAppendNumber(dst []byte, value float32, decimals int) []byte {
	if value == 0 {
		value = 0
	}
	return strconv.AppendFloat(dst, float64(value), 'f', decimals, 32)
}

// chartAppendShortest is the default tooltip value: the shortest decimal that
// reads back as the same float32.
func chartAppendShortest(dst []byte, value float32) []byte {
	if value == 0 {
		value = 0
	}
	return strconv.AppendFloat(dst, float64(value), 'f', -1, 32)
}

func chartAppendPercent(dst []byte, value float32) []byte {
	dst = chartAppendNumber(dst, value*100, 0)
	return append(dst, '%')
}

// ChartCurve selects how a series line connects its points.
type ChartCurve uint8

const (
	// ChartCurveLinear joins points with straight segments.
	ChartCurveLinear ChartCurve = iota
	// ChartCurveNatural is a natural cubic spline, d3's curveNatural and the
	// "natural" type of Recharts. It is smooth but may overshoot between points.
	ChartCurveNatural
	// ChartCurveMonotone is d3's curveMonotoneX: smooth, and never overshooting
	// the data between two points.
	ChartCurveMonotone
	// ChartCurveStep holds each value halfway to the next point.
	ChartCurveStep
)

// chartScratch is the working memory of curve construction. A ChartState owns
// one so that steady-state paints allocate nothing; without a state each paint
// makes a fresh one.
type chartScratch struct {
	points []gfx.Point
	lower  []gfx.Point
	c1, c2 []gfx.Point
	rhs    []gfx.Point
	coefA  []float32
	coefB  []float32
}

func chartGrow[T any](items []T, count int) []T {
	if cap(items) < count {
		return make([]T, count, count+count/4)
	}
	return items[:count]
}

// clip gives back arrays that a past peak left far larger than the content.
func (scratch *chartScratch) clip() {
	const minCapacity = 4096
	if cap(scratch.points) > minCapacity && len(scratch.points)*4 < cap(scratch.points) {
		*scratch = chartScratch{}
	}
}

// chartCurveForward appends the curve through points to builder. The builder's
// current point must already be points[0]; it ends at the last point.
func chartCurveForward(builder *gfx.PathBuilder, curve ChartCurve, points []gfx.Point, scratch *chartScratch) {
	count := len(points)
	if count < 2 {
		return
	}
	switch chartEffectiveCurve(curve, count) {
	case ChartCurveStep:
		for index := 1; index < count; index++ {
			previous, point := points[index-1], points[index]
			middle := (previous.X + point.X) / 2
			builder.LineTo(middle, previous.Y)
			builder.LineTo(middle, point.Y)
			builder.LineTo(point.X, point.Y)
		}
	case ChartCurveNatural, ChartCurveMonotone:
		chartControls(curve, points, scratch)
		for index := 0; index < count-1; index++ {
			first, second, end := scratch.c1[index], scratch.c2[index], points[index+1]
			builder.CubicTo(first.X, first.Y, second.X, second.Y, end.X, end.Y)
		}
	default:
		for index := 1; index < count; index++ {
			builder.LineTo(points[index].X, points[index].Y)
		}
	}
}

// chartCurveReverse is chartCurveForward walked backwards: the builder's current
// point must be the last point and it ends at the first. The path is the same
// curve, which is what closes a stacked area along its lower edge.
func chartCurveReverse(builder *gfx.PathBuilder, curve ChartCurve, points []gfx.Point, scratch *chartScratch) {
	count := len(points)
	if count < 2 {
		return
	}
	switch chartEffectiveCurve(curve, count) {
	case ChartCurveStep:
		for index := count - 1; index > 0; index-- {
			previous, point := points[index-1], points[index]
			middle := (previous.X + point.X) / 2
			builder.LineTo(middle, point.Y)
			builder.LineTo(middle, previous.Y)
			builder.LineTo(previous.X, previous.Y)
		}
	case ChartCurveNatural, ChartCurveMonotone:
		chartControls(curve, points, scratch)
		for index := count - 2; index >= 0; index-- {
			first, second, end := scratch.c1[index], scratch.c2[index], points[index]
			builder.CubicTo(second.X, second.Y, first.X, first.Y, end.X, end.Y)
		}
	default:
		for index := count - 2; index >= 0; index-- {
			builder.LineTo(points[index].X, points[index].Y)
		}
	}
}

// chartEffectiveCurve falls back to straight segments where a spline has too
// few points to bend, as d3 does.
func chartEffectiveCurve(curve ChartCurve, count int) ChartCurve {
	if (curve == ChartCurveNatural || curve == ChartCurveMonotone) && count < 3 {
		return ChartCurveLinear
	}
	return curve
}

func chartControls(curve ChartCurve, points []gfx.Point, scratch *chartScratch) {
	segments := len(points) - 1
	scratch.c1 = chartGrow(scratch.c1, segments)
	scratch.c2 = chartGrow(scratch.c2, segments)
	if curve == ChartCurveNatural {
		chartNaturalControls(points, scratch)
	} else {
		chartMonotoneControls(points, scratch)
	}
}

// chartNaturalControls solves the tridiagonal system of d3's natural spline for
// the two control points of every segment. The system is linear with the same
// coefficients in x and y, so the right-hand side is carried as a point.
func chartNaturalControls(points []gfx.Point, scratch *chartScratch) {
	segments := len(points) - 1
	scratch.coefA = chartGrow(scratch.coefA, segments)
	scratch.coefB = chartGrow(scratch.coefB, segments)
	scratch.rhs = chartGrow(scratch.rhs, segments)
	a, b, r := scratch.coefA, scratch.coefB, scratch.rhs
	c1, c2 := scratch.c1, scratch.c2

	a[0], b[0] = 0, 2
	r[0] = gfx.Point{X: points[0].X + 2*points[1].X, Y: points[0].Y + 2*points[1].Y}
	for index := 1; index < segments-1; index++ {
		a[index], b[index] = 1, 4
		r[index] = gfx.Point{X: 4*points[index].X + 2*points[index+1].X, Y: 4*points[index].Y + 2*points[index+1].Y}
	}
	last := segments - 1
	a[last], b[last] = 2, 7
	r[last] = gfx.Point{X: 8*points[last].X + points[segments].X, Y: 8*points[last].Y + points[segments].Y}
	for index := 1; index < segments; index++ {
		ratio := a[index] / b[index-1]
		b[index] -= ratio
		r[index].X -= ratio * r[index-1].X
		r[index].Y -= ratio * r[index-1].Y
	}
	c1[last] = gfx.Point{X: r[last].X / b[last], Y: r[last].Y / b[last]}
	for index := segments - 2; index >= 0; index-- {
		c1[index] = gfx.Point{X: (r[index].X - c1[index+1].X) / b[index], Y: (r[index].Y - c1[index+1].Y) / b[index]}
	}
	c2[last] = gfx.Point{X: (points[segments].X + c1[last].X) / 2, Y: (points[segments].Y + c1[last].Y) / 2}
	for index := 0; index < last; index++ {
		c2[index] = gfx.Point{X: 2*points[index+1].X - c1[index+1].X, Y: 2*points[index+1].Y - c1[index+1].Y}
	}
}

// chartMonotoneControls computes d3's monotoneX tangents (Fritsch and Carlson
// with a limit on each side) and turns them into cubic Bezier control points.
func chartMonotoneControls(points []gfx.Point, scratch *chartScratch) {
	segments := len(points) - 1
	scratch.coefA = chartGrow(scratch.coefA, segments+1)
	scratch.coefB = chartGrow(scratch.coefB, segments+1)
	slope, tangent := scratch.coefA, scratch.coefB
	for index := 0; index < segments; index++ {
		width := points[index+1].X - points[index].X
		if width != 0 {
			slope[index] = (points[index+1].Y - points[index].Y) / width
		} else {
			slope[index] = 0
		}
	}
	for index := 1; index < segments; index++ {
		tangent[index] = chartMonotoneTangent(
			points[index].X-points[index-1].X, points[index+1].X-points[index].X,
			slope[index-1], slope[index])
	}
	tangent[0] = chartMonotoneEnd(points[1].X-points[0].X, slope[0], tangent[1])
	tangent[segments] = chartMonotoneEnd(points[segments].X-points[segments-1].X, slope[segments-1], tangent[segments-1])
	for index := 0; index < segments; index++ {
		third := (points[index+1].X - points[index].X) / 3
		scratch.c1[index] = gfx.Point{X: points[index].X + third, Y: points[index].Y + third*tangent[index]}
		scratch.c2[index] = gfx.Point{X: points[index+1].X - third, Y: points[index+1].Y - third*tangent[index+1]}
	}
}

func chartMonotoneTangent(widthBefore, widthAfter, slopeBefore, slopeAfter float32) float32 {
	total := widthBefore + widthAfter
	if total == 0 {
		return 0
	}
	weighted := (slopeBefore*widthAfter + slopeAfter*widthBefore) / total
	limit := min(chartAbs(slopeBefore), chartAbs(slopeAfter), 0.5*chartAbs(weighted))
	result := (chartSign(slopeBefore) + chartSign(slopeAfter)) * limit
	if result != result {
		return 0
	}
	return result
}

func chartMonotoneEnd(width, slope, neighbor float32) float32 {
	if width == 0 {
		return neighbor
	}
	return (3*slope - neighbor) / 2
}

func chartAbs(value float32) float32 {
	if value < 0 {
		return -value
	}
	return value
}

func chartSign(value float32) float32 {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	}
	return 0
}
