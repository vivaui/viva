// Package chart is the opt-in page control for the area and bar charts of
// package components. It wraps PaintAreaChart and PaintBarChart in a
// ui.Control, so a page declares a chart without calling the painters itself:
// the view owns the chart data, its ChartState and the handlers that turn the
// pointer and the keyboard into a hovered point.
//
// It is a separate package so an application that does not import it pays
// nothing for it, and it depends only on the public gfx, components and ui
// packages: it neither reaches into the page engine nor changes it.
//
//	visitors := &chart.AreaChartView{
//		Chart: components.AreaChart{
//			Series: []components.ChartSeries{{Key: "visitors", Label: "Visitors", Color: theme.Chart[0]}},
//			Values: [][]float32{counts},
//		},
//		OnHover: func(index int, active bool) { m.inspect(index, active) },
//	}
//	...
//	chart.View(c, "Visitors", visitors).FillWidth().Height(240)
//
// A chart that is only a container and caller-drawn geometry uses Container:
//
//	commits := &chart.Container{
//		Series:  series,
//		Options: chart.ContainerOptions{Legend: chart.LegendBottom},
//		Draw:    m.drawCommits, // fills layout.PlotBounds
//	}
//	...
//	chart.ContainerView(c, "Commits", commits).Height(240)
//
// The package also names the components package's chart types and constants, so
// a page declares chart data with chart.Series, chart.Axis and
// chart.CurveMonotone without importing components itself.
//
// A view belongs to one place on one page and is kept in the model. Its
// handlers and draw function are made once, so a warm paint allocates nothing;
// do not copy a view after it has been declared.
package chart

import (
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// chartDefaultHeight is the height a view takes when the caller gives none,
// matching the chart components' own initial height.
const chartDefaultHeight float32 = 200

// AreaChartView is one area or line chart's data, state and page control. Fill
// its Chart from the caller's typed slices every paint; State and the handlers
// stay as they are, and the view keeps the last plot so the pointer can be
// mapped back to a data point.
//
// OnHover, OnPointer, OnKey and OnScroll are optional. They may be set once or
// changed between paints; the handlers read them when the event arrives.
type AreaChartView struct {
	// Chart is the data and presentation the components painter draws.
	Chart components.AreaChart
	// State is the hovered point and the caches that keep a steady chart from
	// allocating. Keep it with the view.
	State components.ChartState

	// OnHover is told whenever the hovered point changes, with -1 and false
	// when the pointer leaves. It is called from input handling, and from the
	// paint that notices the pointer left without an exit event.
	OnHover func(index int, active bool)
	// OnPointer receives every pointer event the control gets, after the
	// view's own hover work. Its result is combined with the view's.
	OnPointer func(gfx.PointerEvent) gfx.EventResult
	// OnKey receives every key event the control gets, after the view's own
	// arrow, Home and End handling. Its result is combined with the view's.
	OnKey func(gfx.KeyEvent) gfx.EventResult
	// OnScroll receives the wheel over the control. Unlike the pointer and key
	// callbacks it comes first: the chart has no scrolling of its own, so an
	// unhandled return lets the page scroll what the chart is in.
	OnScroll func(gfx.ScrollEvent) gfx.EventResult

	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	hover    hoverState
	label    string
	prepared bool
}

// View declares an area chart at c. The returned box is full width and 200
// high unless a Width or Height follows it. label names the control for
// assistive technology; when Chart.Label is empty it also names the plot.
//
// Give the chart a size that suits its data. Declare the view on its own, not
// inside a Memo.
func View(c ui.C, label string, chart *AreaChartView) ui.Box {
	if chart == nil {
		panic("chart: View needs its AreaChartView")
	}
	chart.prepare()
	chart.label = label
	return ui.Control(c, label, &chart.handlers).
		FillWidth().Height(chartDefaultHeight).
		Draw(chart.drawFn)
}

func (v *AreaChartView) prepare() {
	if v.prepared {
		return
	}
	v.prepared = true
	v.handlers.Pointer = v.pointer
	v.handlers.Key = v.key
	v.handlers.Scroll = v.scroll
	v.drawFn = v.draw
}

func (v *AreaChartView) pointer(event gfx.PointerEvent) gfx.EventResult {
	var result gfx.EventResult
	switch event.Kind {
	case gfx.PointerMove:
		result = v.hover.pointerMove(&v.State, v.OnHover, event.X)
		result.Handled = true
	case gfx.PointerDown:
		// A tap without a preceding move still hovers the point it lands on.
		// The press stays unhandled so a click on the chart reaches the page.
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

func (v *AreaChartView) key(event gfx.KeyEvent) gfx.EventResult {
	result := v.hover.key(&v.State, v.OnHover, event)
	if v.OnKey != nil {
		result = combine(result, v.OnKey(event))
	}
	return result
}

func (v *AreaChartView) scroll(event gfx.ScrollEvent) gfx.EventResult {
	if v.OnScroll == nil {
		return gfx.EventResult{}
	}
	return v.OnScroll(event)
}

func (v *AreaChartView) draw(d *ui.DrawContext) {
	// A pointer that leaves the window without a press gets no exit event: the
	// repaint that clears the hover is where the data sees it. A press keeps
	// the pointer until it is released, even outside the window.
	if v.hover.pointer && !d.State.Hovered && !d.State.Pressed {
		v.hover.clear(&v.State, v.OnHover, true)
	}
	data := v.Chart
	if data.Label == "" {
		data.Label = v.label
	}
	v.hover.plot = components.PaintAreaChart(d.Canvas, *d.Theme, d.Rect, d.Node.ID, data, &v.State)
}

// hoverState is what a view remembers between paints and input events to turn
// a pointer position or a key into a data point. pointer says the current hover
// came from the pointer, so leaving the chart clears it without clearing a
// keyboard hover.
type hoverState struct {
	plot    components.ChartPlot
	pointer bool
}

// pointerMove sets the point nearest x. It repaints only when the point changes.
func (h *hoverState) pointerMove(state *components.ChartState, notify func(int, bool), x float32) gfx.EventResult {
	index := h.plot.IndexAt(x)
	previous, active := state.Index()
	h.pointer = true
	if index < 0 {
		if !active {
			return gfx.EventResult{}
		}
	} else if active && previous == index {
		return gfx.EventResult{}
	}
	state.Hover(index)
	if notify != nil {
		notify(index, index >= 0)
	}
	return gfx.EventResult{Repaint: true}
}

// clear removes the hover. When pointerOnly is set it does nothing to a hover
// the keyboard set.
func (h *hoverState) clear(state *components.ChartState, notify func(int, bool), pointerOnly bool) gfx.EventResult {
	if pointerOnly && !h.pointer {
		return gfx.EventResult{}
	}
	h.pointer = false
	if _, active := state.Index(); !active {
		return gfx.EventResult{}
	}
	state.Clear()
	if notify != nil {
		notify(-1, false)
	}
	return gfx.EventResult{Repaint: true}
}

// key steps the hovered point with the chart components' own key mapping.
func (h *hoverState) key(state *components.ChartState, notify func(int, bool), event gfx.KeyEvent) gfx.EventResult {
	if event.Release {
		return gfx.EventResult{}
	}
	index, active := state.Index()
	if !active {
		index = -1
	}
	next, handled := components.ChartStepForKey(event.Key, index, h.plot.Count())
	if !handled {
		return gfx.EventResult{}
	}
	h.pointer = false
	if active && next == index {
		return gfx.EventResult{Handled: true}
	}
	state.Hover(next)
	if notify != nil {
		notify(next, true)
	}
	return gfx.EventResult{Handled: true, Repaint: true}
}

// combine folds a callback's result into the view's own. Repaint and Handled
// from either side are kept, and the first side with a focus, blur, activation
// or clipboard request wins.
func combine(result, extra gfx.EventResult) gfx.EventResult {
	result.Handled = result.Handled || extra.Handled
	result.Repaint = result.Repaint || extra.Repaint
	result.Blur = result.Blur || extra.Blur
	result.FocusVisible = result.FocusVisible || extra.FocusVisible
	result.ContinueTraversal = result.ContinueTraversal || extra.ContinueTraversal
	if result.Focus.IsZero() {
		result.Focus = extra.Focus
	}
	if result.Activate.IsZero() {
		result.Activate = extra.Activate
	}
	if result.ClipboardText == "" {
		result.ClipboardText = extra.ClipboardText
	}
	return result
}
