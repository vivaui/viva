package chart

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// BarChartView is one bar chart's data, state and page control. It is the bar
// counterpart of AreaChartView: fill Chart each paint and keep State with the
// view; the pointer, keys and callbacks behave the same way.
type BarChartView struct {
	// Chart is the data and presentation the components painter draws.
	Chart components.BarChart
	// State is the hovered category and the caches that keep a steady chart
	// from allocating. Keep it with the view.
	State components.ChartState

	// OnHover is told whenever the hovered category changes, with -1 and false
	// when the pointer leaves.
	OnHover func(index int, active bool)
	// OnPointer receives every pointer event the control gets, after the
	// view's own hover work. Its result is combined with the view's.
	OnPointer func(gfx.PointerEvent) gfx.EventResult
	// OnKey receives every key event the control gets, after the view's own
	// arrow, Home and End handling. Its result is combined with the view's.
	OnKey func(gfx.KeyEvent) gfx.EventResult
	// OnScroll receives the wheel over the control.
	OnScroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	hover    hoverState
	label    string
	prepared bool
}

// Bar declares a bar chart at c. The returned box is full width and 200 high
// unless a Width or Height follows it. label names the control for assistive
// technology; when Chart.Label is empty it also names the plot.
//
// Declare the view on its own, not inside a Memo.
func Bar(c ui.C, label string, chart *BarChartView) ui.Box {
	if chart == nil {
		panic("chart: Bar needs its BarChartView")
	}
	chart.prepare()
	chart.label = label
	return ui.Control(c, label, &chart.handlers).
		FillWidth().Height(chartDefaultHeight).
		Draw(chart.drawFn)
}

func (v *BarChartView) prepare() {
	if v.prepared {
		return
	}
	v.prepared = true
	v.handlers.Pointer = v.pointer
	v.handlers.Key = v.key
	v.handlers.Scroll = v.scroll
	v.drawFn = v.draw
}

func (v *BarChartView) pointer(event gfx.PointerEvent) gfx.EventResult {
	var result gfx.EventResult
	switch event.Kind {
	case gfx.PointerMove:
		result = v.hover.pointerMove(&v.State, v.OnHover, event.X)
		result.Handled = true
	case gfx.PointerDown:
		result = v.hover.pointerMove(&v.State, v.OnHover, event.X)
	case gfx.PointerExit:
		result = v.hover.clear(&v.State, v.OnHover, true)
		result.Handled = true
	}
	if v.OnPointer != nil {
		result = combine(result, v.OnPointer(event))
	}
	return result
}

func (v *BarChartView) key(event gfx.KeyEvent) gfx.EventResult {
	result := v.hover.key(&v.State, v.OnHover, event)
	if v.OnKey != nil {
		result = combine(result, v.OnKey(event))
	}
	return result
}

func (v *BarChartView) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if v.OnScroll == nil {
		return gfx.EventResult{}
	}
	return v.OnScroll(event)
}

func (v *BarChartView) draw(d *ui.DrawContext) {
	if v.hover.pointer && !d.State.Hovered && !d.State.Pressed {
		v.hover.clear(&v.State, v.OnHover, true)
	}
	chart := v.Chart
	if chart.Label == "" {
		chart.Label = v.label
	}
	v.hover.plot = components.PaintBarChart(d.Canvas, *d.Theme, d.Rect, d.Node.ID, chart, &v.State)
}
