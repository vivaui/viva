package chart

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// containerDefaultHeight is the height a container takes when the caller gives
// none. It leaves room for the legend and a readable plot.
const containerDefaultHeight float32 = 200

// containerTooltipInset is the gap between the tooltip and the plot edge.
const containerTooltipInset float32 = 8

// containerTooltipLayer is the layer a tooltip with no explicit Layer uses.
const containerTooltipLayer uint16 = 4

// Container is a caller-owned chart container and the geometry drawn inside it.
// PaintChartContainer resolves the legend and the plot rectangle; Draw receives
// that layout so the caller paints its own bars, lines or annotations in it. A
// Tooltip snapshot, when set, is painted over the plot after Draw.
//
// Declare the container with the model and fill Series, Tooltip and any changed
// Options each paint. Draw is set once, when the model is created: ContainerView
// keeps it and the handlers, so a warm paint allocates nothing. Do not copy a
// container after it has been declared.
type Container struct {
	// Series is the metadata the legend and the tooltip read.
	Series []components.ChartSeries
	// Options configures the container chrome, currently its legend.
	Options components.ChartContainerOptions
	// Tooltip is the caller-owned payload painted over the plot. Nil paints no
	// tooltip.
	Tooltip *Tooltip
	// Draw paints the caller's data geometry inside layout.PlotBounds. It runs
	// after the container chrome and before the tooltip; nil draws nothing.
	Draw func(*ui.DrawContext, Layout)

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	prepared bool
}

// Tooltip is the typed snapshot a Container paints: the rows to show and the
// options that frame them. The container measures it and places it at the
// plot's top-right corner, moving it inside the plot when that would overflow.
type Tooltip struct {
	Rows    []components.ChartTooltipRow
	Options components.ChartTooltipOptions
}

// ContainerView declares c's chart container. It paints the container chrome,
// calls Container.Draw with the resolved layout, and paints Container.Tooltip
// when one is set. The returned box is full width and 200 high unless a Width
// or Height follows it. label names the control for assistive technology.
//
// Declare the container with the model, not inside a Memo.
func ContainerView(c ui.C, label string, container *Container) ui.Box {
	if container == nil {
		panic("chart: ContainerView needs its Container")
	}
	container.prepare()
	return ui.Control(c, label, &container.handlers).
		FillWidth().Height(containerDefaultHeight).
		Draw(container.drawFn)
}

func (c *Container) prepare() {
	if c.prepared {
		return
	}
	c.prepared = true
	c.drawFn = c.draw
}

func (c *Container) draw(d *ui.DrawContext) {
	layout := components.PaintChartContainer(d.Canvas, *d.Theme, d.Rect, c.Series, c.Options)
	if c.Draw != nil {
		c.Draw(d, layout)
	}
	if c.Tooltip != nil {
		c.paintTooltip(d, layout)
	}
}

// paintTooltip places the tooltip at the plot's top-right corner and keeps it
// inside the plot when it is wider or taller than the space to the edge.
func (c *Container) paintTooltip(d *ui.DrawContext, layout Layout) {
	tooltip := c.Tooltip
	plot := layout.PlotBounds
	options := tooltip.Options
	if options.Layer == 0 {
		options.Layer = containerTooltipLayer
	}
	size := components.ChartTooltipSize(d.Canvas, *d.Theme, c.Series, tooltip.Rows, options)
	if size.Width <= 0 || size.Height <= 0 {
		return
	}
	x := plot.X + plot.Width - size.Width - containerTooltipInset
	if x < plot.X {
		x = plot.X
	}
	y := plot.Y + containerTooltipInset
	if y+size.Height > plot.Y+plot.Height {
		y = plot.Y + plot.Height - size.Height
	}
	if y < plot.Y {
		y = plot.Y
	}
	components.PaintChartTooltip(d.Canvas, *d.Theme, gfx.Rect{X: x, Y: y}, c.Series, tooltip.Rows, options)
}
