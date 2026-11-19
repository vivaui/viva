package chartview

import "github.com/vivaui/viva/components"

// The chart vocabulary a page names without importing package components. The
// types and constants are aliases of the components package's: a chartview data
// set is the same value the painters read, so passing it on costs nothing.

// Series is one series' metadata: the legend and tooltip label and its color.
type Series = components.ChartSeries

// AreaChart is the data and presentation of an area or line chart.
type AreaChart = components.AreaChart

// BarChart is the data and presentation of a bar chart.
type BarChart = components.BarChart

// Axis configures a chart's X or Y axis.
type Axis = components.ChartAxis

// AreaStyle overrides one series' area fill.
type AreaStyle = components.ChartAreaStyle

// AreaGradient is a vertical fill gradient between two colors.
type AreaGradient = components.ChartAreaGradient

// Layout is the container's resolved bounds: the plot the caller draws its own
// geometry into and the legend chrome around it.
type Layout = components.ChartLayout

// ContainerOptions configures the chart container's legend.
type ContainerOptions = components.ChartContainerOptions

// TooltipRow is one row of a tooltip's caller-owned payload.
type TooltipRow = components.ChartTooltipRow

// TooltipOptions configures the tooltip's label, indicator and layer.
type TooltipOptions = components.ChartTooltipOptions

// Chart legends.
const (
	LegendNone   = components.ChartLegendNone
	LegendTop    = components.ChartLegendTop
	LegendBottom = components.ChartLegendBottom
)

// Area and bar chart curves.
const (
	CurveLinear   = components.ChartCurveLinear
	CurveNatural  = components.ChartCurveNatural
	CurveMonotone = components.ChartCurveMonotone
	CurveStep     = components.ChartCurveStep
)

// Area chart fills.
const (
	FillGradient = components.ChartFillGradient
	FillSolid    = components.ChartFillSolid
	FillNone     = components.ChartFillNone
)

// Chart grids.
const (
	GridNone       = components.ChartGridNone
	GridHorizontal = components.ChartGridHorizontal
	GridVertical   = components.ChartGridVertical
	GridBoth       = components.ChartGridBoth
)

// How series combine at one X.
const (
	StackNone    = components.ChartStackNone
	StackStacked = components.ChartStackStacked
	StackExpand  = components.ChartStackExpand
)

// Tooltip payload indicators.
const (
	TooltipDot    = components.ChartTooltipDot
	TooltipLine   = components.ChartTooltipLine
	TooltipDashed = components.ChartTooltipDashed
)

// Downsample is the page control's name for components.ChartDownsample: it
// returns the ascending indices of the points worth keeping for the bucket
// budget. Reduce once when the data changes and draw the reduced series on
// every paint.
func Downsample(dst []int, x, y []float32, buckets int) []int {
	return components.ChartDownsample(dst, x, y, buckets)
}
