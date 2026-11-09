package components

import "github.com/vivaui/viva/gfx"

const (
	chartInitialWidth  float32 = 320
	chartInitialHeight float32 = 200
)

// ChartSeries is caller-owned metadata shared by a chart's legend, tooltip,
// and data geometry. Key identifies the series to application code; chart
// chrome only reads Label and Color.
type ChartSeries struct {
	Key    string
	Label  string
	Color  gfx.Color
	Hidden bool
	// AreaStyle optionally overrides the fill of this series in AreaChart.
	// Other chart kinds ignore it. Nil preserves the chart's default fill.
	AreaStyle *ChartAreaStyle
}

// ChartAreaDefaultOpacity is the fill alpha a chart uses when its series has no
// ChartAreaStyle. Set ChartAreaStyle.Opacity to this value to keep the default
// alpha while overriding only the color or gradient.
const ChartAreaDefaultOpacity = 0.4

// ChartAreaStyle controls a series' fill independently of its line. The
// pointer on ChartSeries is optional; an explicit zero Opacity is transparent,
// so use ChartAreaDefaultOpacity when only the color should change.
type ChartAreaStyle struct {
	// Color overrides the series fill color. A zero color uses the series color.
	Color gfx.Color
	// Opacity multiplies the fill alpha, clamped to [0,1]. Non-finite is zero.
	Opacity float32
	// Gradient overrides the endpoint colors of ChartFillGradient. Nil fades
	// Color (or the series color) using the chart's default alpha stops.
	Gradient *ChartAreaGradient
}

// ChartAreaGradient is a vertical, baseline-relative color gradient. From is
// at the farthest curve edge and To is at the baseline, including below zero.
// Colors include alpha; ChartAreaStyle.Opacity multiplies both endpoints.
type ChartAreaGradient struct{ From, To gfx.Color }

// ChartLegendPosition selects where the chart container reserves its legend.
type ChartLegendPosition uint8

const (
	ChartLegendNone ChartLegendPosition = iota
	ChartLegendTop
	ChartLegendBottom
)

// ChartContainerOptions configures the source-owned chart chrome. The caller
// remains responsible for painting axes and data geometry inside PlotBounds.
type ChartContainerOptions struct {
	Legend ChartLegendPosition
}

// ChartLayout reports the resolved container and the space left for caller-
// owned chart geometry. LegendBounds includes the official 12px separation.
type ChartLayout struct {
	Bounds       gfx.Rect
	PlotBounds   gfx.Rect
	LegendBounds gfx.Rect
}

// ChartTooltipIndicator selects the official tooltip payload marker.
type ChartTooltipIndicator uint8

const (
	ChartTooltipDot ChartTooltipIndicator = iota
	ChartTooltipLine
	ChartTooltipDashed
)

// ChartTooltipRow is one caller-owned payload snapshot. Series indexes the
// metadata supplied to PaintChartTooltip; Label overrides that series label.
type ChartTooltipRow struct {
	Series int
	Label  string
	Value  string
	Hidden bool
}

// ChartTooltipOptions configures the official tooltip-content shell.
type ChartTooltipOptions struct {
	Label         string
	Indicator     ChartTooltipIndicator
	HideLabel     bool
	HideIndicator bool
	Layer         uint16
}

// PaintChartContainer paints optional legend chrome and returns the plot
// rectangle for caller-owned data geometry. Like shadcn's ChartContainer, it
// is not a chart engine: it performs no scaling, ticks, paths, or animation.
func PaintChartContainer(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, series []ChartSeries, options ChartContainerOptions) ChartLayout {
	if canvas == nil {
		return ChartLayout{}
	}
	bounds = chartResolveBounds(bounds)
	layout := ChartLayout{Bounds: bounds, PlotBounds: bounds}
	if options.Legend == ChartLegendNone || chartVisibleSeries(series) == 0 {
		return layout
	}

	scale := theme.Metrics.ControlHeight / 32
	legendHeight := (16 + 12) * scale
	if legendHeight > bounds.Height {
		legendHeight = bounds.Height
	}
	layout.LegendBounds = gfx.Rect{X: bounds.X, Width: bounds.Width, Height: legendHeight}
	if options.Legend == ChartLegendTop {
		layout.LegendBounds.Y = bounds.Y
		layout.PlotBounds.Y += legendHeight
		layout.PlotBounds.Height -= legendHeight
		PaintChartLegend(canvas, theme, gfx.Rect{
			X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: 16 * scale,
		}, series)
	} else {
		layout.LegendBounds.Y = bounds.Y + bounds.Height - legendHeight
		layout.PlotBounds.Height -= legendHeight
		PaintChartLegend(canvas, theme, gfx.Rect{
			X: bounds.X, Y: layout.LegendBounds.Y + 12*scale,
			Width: bounds.Width, Height: 16 * scale,
		}, series)
	}
	return layout
}

// PaintChartLegend paints the official centered legend row with 8px series
// swatches, 6px item-internal gaps, and 16px gaps between entries.
func PaintChartLegend(canvas *gfx.Canvas, theme gfx.Theme, bounds gfx.Rect, series []ChartSeries) gfx.Rect {
	if canvas == nil || bounds.Width <= 0 || bounds.Height <= 0 {
		return gfx.Rect{}
	}
	scale := theme.Metrics.ControlHeight / 32
	swatchSize := 8 * scale
	itemGap := 6 * scale
	seriesGap := 16 * scale
	visible := chartVisibleSeries(series)
	if visible == 0 {
		return gfx.Rect{}
	}

	totalWidth := float32(visible-1) * seriesGap
	for index := range series {
		if series[index].Hidden {
			continue
		}
		totalWidth += swatchSize + itemGap + canvas.TextWidth(series[index].Label, theme.Metrics.FontSizeS)
	}
	painted := gfx.Rect{
		X:     bounds.X + (bounds.Width-totalWidth)/2,
		Y:     bounds.Y,
		Width: totalWidth, Height: bounds.Height,
	}
	x := painted.X
	remaining := visible
	for index := range series {
		item := series[index]
		if item.Hidden {
			continue
		}
		swatch := gfx.Rect{
			X: x, Y: bounds.Y + (bounds.Height-swatchSize)/2,
			Width: swatchSize, Height: swatchSize,
		}
		canvas.Rounded(swatch, 2*scale, item.Color)
		x += swatchSize + itemGap
		labelWidth := canvas.TextWidth(item.Label, theme.Metrics.FontSizeS)
		canvas.Label(item.Label, gfx.Rect{X: x, Y: bounds.Y, Width: labelWidth, Height: bounds.Height}, gfx.TextStyle{
			Size: theme.Metrics.FontSizeS, Color: theme.Foreground,
		})
		x += labelWidth
		remaining--
		if remaining > 0 {
			x += seriesGap
		}
	}
	return painted
}

// ChartTooltipSize measures the tooltip PaintChartTooltip would paint for the
// same arguments without painting it, so a chart can place it beside a cursor and
// flip it away from an edge before drawing. It is the zero Size when no row shows.
func ChartTooltipSize(canvas *gfx.Canvas, theme gfx.Theme, series []ChartSeries, rows []ChartTooltipRow, options ChartTooltipOptions) gfx.Size {
	if canvas == nil || chartVisibleRows(rows) == 0 {
		return gfx.Size{}
	}
	scale := theme.Metrics.ControlHeight / 32
	fontSize := theme.Metrics.FontSizeS
	paddingX := 10 * scale
	paddingY := 6 * scale
	rowHeight := 16 * scale
	gap := 6 * scale
	nestedLabel := chartVisibleRows(rows) == 1 && options.Indicator != ChartTooltipDot && !options.HideLabel && options.Label != ""

	contentWidth := float32(0)
	if !options.HideLabel && !nestedLabel {
		contentWidth = canvas.TextWidth(options.Label, fontSize)
	}
	for index := range rows {
		row := rows[index]
		if row.Hidden {
			continue
		}
		label := chartTooltipLabel(series, row)
		width := canvas.TextWidth(label, fontSize) + canvas.TextWidth(row.Value, fontSize)
		if label != "" && row.Value != "" {
			width += 16 * scale
		}
		if !options.HideIndicator {
			width += chartIndicatorWidth(options.Indicator, scale) + 8*scale
		}
		if width > contentWidth {
			contentWidth = width
		}
	}
	width := contentWidth + paddingX*2
	if minimum := 128 * scale; width < minimum {
		width = minimum
	}
	visibleRows := chartVisibleRows(rows)
	height := paddingY*2 + float32(visibleRows)*rowHeight + float32(visibleRows-1)*gap
	if !options.HideLabel && options.Label != "" {
		height += rowHeight + gap
	}
	return gfx.Size{Width: width, Height: height}
}

// PaintChartTooltip paints the official tooltip-content shell at origin.X/Y.
// It reads immutable payload rows and series metadata and performs no value
// formatting, lifecycle management, hit testing, or data-geometry rendering.
func PaintChartTooltip(canvas *gfx.Canvas, theme gfx.Theme, origin gfx.Rect, series []ChartSeries, rows []ChartTooltipRow, options ChartTooltipOptions) gfx.Rect {
	if canvas == nil || chartVisibleRows(rows) == 0 {
		return gfx.Rect{}
	}
	scale := theme.Metrics.ControlHeight / 32
	fontSize := theme.Metrics.FontSizeS
	paddingX := 10 * scale
	paddingY := 6 * scale
	rowHeight := 16 * scale
	gap := 6 * scale
	nestedLabel := chartVisibleRows(rows) == 1 && options.Indicator != ChartTooltipDot && !options.HideLabel && options.Label != ""

	size := ChartTooltipSize(canvas, theme, series, rows, options)
	contentWidth := size.Width - paddingX*2
	bounds := gfx.Rect{X: origin.X, Y: origin.Y, Width: size.Width, Height: size.Height}

	canvas.PushLayer(options.Layer)
	border := theme.Border
	border.Alpha *= 0.5
	canvas.RoundedBorder(bounds, theme.Metrics.RadiusSurface, 1, border, theme.Background)

	y := bounds.Y + paddingY
	if !options.HideLabel && options.Label != "" && !nestedLabel {
		canvas.Label(options.Label, gfx.Rect{X: bounds.X + paddingX, Y: y, Width: contentWidth, Height: rowHeight}, gfx.TextStyle{
			Size: fontSize, Weight: gfx.WeightMedium, Color: theme.Foreground,
		})
		y += rowHeight + gap
	}
	for index := range rows {
		row := rows[index]
		if row.Hidden {
			continue
		}
		rowY := y
		rowBlockHeight := rowHeight
		if nestedLabel {
			rowBlockHeight += rowHeight + gap
		}
		x := bounds.X + paddingX
		color := chartTooltipColor(series, row.Series)
		if !options.HideIndicator {
			chartPaintIndicator(canvas, options.Indicator, gfx.Rect{X: x, Y: rowY, Width: chartIndicatorWidth(options.Indicator, scale), Height: rowBlockHeight}, color, scale)
			x += chartIndicatorWidth(options.Indicator, scale) + 8*scale
		}
		if nestedLabel {
			canvas.Label(options.Label, gfx.Rect{X: x, Y: rowY, Width: bounds.X + bounds.Width - paddingX - x, Height: rowHeight}, gfx.TextStyle{
				Size: fontSize, Weight: gfx.WeightMedium, Color: theme.Foreground,
			})
			rowY += rowHeight + gap
		}
		label := chartTooltipLabel(series, row)
		valueWidth := canvas.TextWidth(row.Value, fontSize)
		labelWidth := bounds.X + bounds.Width - paddingX - x
		if row.Value != "" {
			labelWidth -= valueWidth + 8*scale
		}
		if labelWidth < 0 {
			labelWidth = 0
		}
		canvas.Label(label, gfx.Rect{X: x, Y: rowY, Width: labelWidth, Height: rowHeight}, gfx.TextStyle{
			Size: fontSize, Color: theme.MutedForeground,
		})
		if row.Value != "" {
			canvas.Label(row.Value, gfx.Rect{
				X: bounds.X + bounds.Width - paddingX - valueWidth,
				Y: rowY, Width: valueWidth, Height: rowHeight,
			}, gfx.TextStyle{Size: fontSize, Weight: gfx.WeightMedium, Color: theme.Foreground})
		}
		y += rowBlockHeight + gap
	}
	canvas.PopLayer()
	return bounds
}

func chartResolveBounds(bounds gfx.Rect) gfx.Rect {
	if bounds.Width <= 0 {
		bounds.Width = chartInitialWidth
	}
	if bounds.Height <= 0 {
		bounds.Height = chartInitialHeight
	}
	return bounds
}

func chartVisibleSeries(series []ChartSeries) int {
	visible := 0
	for index := range series {
		if !series[index].Hidden {
			visible++
		}
	}
	return visible
}

func chartVisibleRows(rows []ChartTooltipRow) int {
	visible := 0
	for index := range rows {
		if !rows[index].Hidden {
			visible++
		}
	}
	return visible
}

func chartTooltipLabel(series []ChartSeries, row ChartTooltipRow) string {
	if row.Label != "" {
		return row.Label
	}
	if row.Series >= 0 && row.Series < len(series) {
		return series[row.Series].Label
	}
	return ""
}

func chartTooltipColor(series []ChartSeries, index int) gfx.Color {
	if index >= 0 && index < len(series) {
		return series[index].Color
	}
	return gfx.Color{}
}

func chartIndicatorWidth(indicator ChartTooltipIndicator, scale float32) float32 {
	switch indicator {
	case ChartTooltipLine:
		return 4 * scale
	case ChartTooltipDashed:
		return 2 * scale
	default:
		return 10 * scale
	}
}

func chartPaintIndicator(canvas *gfx.Canvas, indicator ChartTooltipIndicator, bounds gfx.Rect, color gfx.Color, scale float32) {
	switch indicator {
	case ChartTooltipLine:
		canvas.Rounded(gfx.Rect{
			X: bounds.X, Y: bounds.Y, Width: bounds.Width, Height: bounds.Height,
		}, 2*scale, color)
	case ChartTooltipDashed:
		dashHeight := 3 * scale
		gap := 2 * scale
		for y := bounds.Y + scale/2; y+dashHeight <= bounds.Y+bounds.Height; y += dashHeight + gap {
			canvas.Fill(gfx.Rect{X: bounds.X, Y: y, Width: bounds.Width, Height: dashHeight}, color)
		}
	default:
		size := 10 * scale
		canvas.Rounded(gfx.Rect{
			X: bounds.X, Y: bounds.Y + (bounds.Height-size)/2,
			Width: size, Height: size,
		}, 2*scale, color)
	}
}
