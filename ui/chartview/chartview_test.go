package chartview_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/chartview"
)

// The month data of the components' own area chart examples.
var (
	desktop = []float32{186, 305, 237, 73, 209, 214}
	mobile  = []float32{80, 200, 120, 190, 130, 140}
)

func areaChart() components.AreaChart {
	return components.AreaChart{
		Series: []components.ChartSeries{
			{Key: "desktop", Label: "Desktop", Color: gfx.Color{Red: .2, Green: .4, Blue: .8, Alpha: 1}},
			{Key: "mobile", Label: "Mobile", Color: gfx.Color{Red: .8, Green: .4, Blue: .2, Alpha: 1}},
		},
		Values:  [][]float32{desktop, mobile},
		Curve:   components.ChartCurveMonotone,
		Grid:    components.ChartGridHorizontal,
		Label:   "Monthly visitors",
		Version: 1,
	}
}

func barChart() components.BarChart {
	return components.BarChart{
		Series: []components.ChartSeries{
			{Key: "desktop", Label: "Desktop", Color: gfx.Color{Red: .2, Green: .4, Blue: .8, Alpha: 1}},
			{Key: "mobile", Label: "Mobile", Color: gfx.Color{Red: .8, Green: .4, Blue: .2, Alpha: 1}},
		},
		Values:  [][]float32{desktop, mobile},
		Grid:    components.ChartGridHorizontal,
		Label:   "Monthly visitors",
		Version: 1,
	}
}

type harness struct {
	t      *testing.T
	area   *chartview.AreaChartView
	bar    *chartview.BarChartView
	box    ui.Box
	page   *ui.Page
	drv    *app.Driver
	inv    *app.Invalidator
	hover  int
	active bool
	hovers int
	moves  int
	wheels int
}

func openArea(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, area: &chartview.AreaChartView{Chart: areaChart()}, hover: -1}
	h.area.OnHover = func(index int, active bool) { h.hover, h.active, h.hovers = index, active, h.hovers+1 }
	h.area.OnPointer = func(gfx.PointerEvent) gfx.EventResult { h.moves++; return gfx.EventResult{} }
	h.area.OnScroll = func(gfx.ScrollEvent) gfx.EventResult { h.wheels++; return gfx.EventResult{} }
	h.open(func(c ui.C) { h.box = chartview.View(c, "Visitors", h.area).Size(400, 240) })
	return h
}

func openBar(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, bar: &chartview.BarChartView{Chart: barChart()}, hover: -1}
	h.bar.OnHover = func(index int, active bool) { h.hover, h.active, h.hovers = index, active, h.hovers+1 }
	h.open(func(c ui.C) { h.box = chartview.Bar(c, "Visitors", h.bar).Size(400, 240) })
	return h
}

func (h *harness) open(view func(ui.C)) {
	h.t.Helper()
	var tree gfx.Tree
	h.page = ui.NewPage(ui.PageConfig{Tree: &tree, View: view})
	h.inv = new(app.Invalidator)
	var err error
	h.drv, err = app.NewDriver(ui.Configure(h.page, app.Config{Width: 480, Height: 320, Invalidator: h.inv}), nil)
	if err != nil {
		h.t.Fatal(err)
	}
	h.t.Cleanup(h.drv.Close)
	h.repaint()
}

func (h *harness) repaint() {
	h.t.Helper()
	h.inv.Invalidate()
	if err := h.drv.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) index() (int, bool) {
	if h.area != nil {
		return h.area.State.Index()
	}
	return h.bar.State.Index()
}

func (h *harness) move(x, y float32) {
	h.t.Helper()
	if err := h.drv.Move(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *harness) click(x, y float32) {
	h.t.Helper()
	if err := h.drv.Click(x, y); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

func (h *harness) key(key gfx.Key) {
	h.t.Helper()
	if err := h.drv.Key(key, 0); err != nil {
		h.t.Fatal(err)
	}
	h.repaint()
}

// The page declares the view, paints it, and the pointer moves the hovered
// point and reports it through the callback; leaving clears it again.
func TestAreaViewPointerHover(t *testing.T) {
	h := openArea(t)
	r := h.box.Rect()
	if r.Width != 400 || r.Height != 240 {
		t.Fatalf("chart box = %v, want 400x240", r)
	}
	centerX, centerY := r.X+r.Width/2, r.Y+r.Height/2
	h.move(centerX, centerY)
	index, active := h.index()
	if !active || index < 0 {
		t.Fatalf("hover after a pointer move = (%d, %v), want an active point", index, active)
	}
	if h.hover != index || !h.active {
		t.Fatalf("OnHover = (%d, %v), want (%d, true)", h.hover, h.active, index)
	}
	if h.moves == 0 {
		t.Fatal("OnPointer did not see the pointer")
	}

	// The left edge is an earlier point than the center.
	h.move(r.X+8, centerY)
	left, active := h.index()
	if !active || left >= index {
		t.Fatalf("hover at the left edge = (%d, %v), want a point before %d", left, active, index)
	}
	if h.hover != left {
		t.Fatalf("OnHover = %d after the second move, want %d", h.hover, left)
	}

	// Leaving the window clears the hover through the repaint.
	if err := h.drv.Leave(); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if index, active := h.index(); active || index != -1 {
		t.Fatalf("hover after leaving = (%d, %v), want (-1, false)", index, active)
	}
	if h.hover != -1 || h.active {
		t.Fatalf("OnHover = (%d, %v) after leaving, want (-1, false)", h.hover, h.active)
	}
}

// Arrow, Home and End move the hovered point through the components' own key
// mapping, and the chart reports each move.
func TestAreaViewKeyboard(t *testing.T) {
	h := openArea(t)
	r := h.box.Rect()
	h.click(r.X+r.Width/2, r.Y+r.Height/2)
	before, active := h.index()
	if !active {
		t.Fatal("the click did not hover a point")
	}
	h.key(gfx.KeyRight)
	next, active := h.index()
	if !active || next != min(before+1, 5) {
		t.Fatalf("Right hovered %d, want %d", next, min(before+1, 5))
	}
	if h.hover != next || !h.active {
		t.Fatalf("OnHover = (%d, %v), want (%d, true)", h.hover, h.active, next)
	}
	h.key(gfx.KeyHome)
	if index, _ := h.index(); index != 0 {
		t.Fatalf("Home hovered %d, want 0", index)
	}
	h.key(gfx.KeyEnd)
	if index, _ := h.index(); index != 5 {
		t.Fatalf("End hovered %d, want 5", index)
	}
}

// The wheel has no chart behavior of its own: the callback sees it.
func TestAreaViewScrollCallback(t *testing.T) {
	h := openArea(t)
	r := h.box.Rect()
	h.move(r.X+r.Width/2, r.Y+r.Height/2)
	if err := h.drv.Scroll(gfx.ScrollEvent{X: r.X + r.Width/2, Y: r.Y + r.Height/2, DeltaY: 1}); err != nil {
		t.Fatal(err)
	}
	if h.wheels != 1 {
		t.Fatalf("OnScroll called %d times, want 1", h.wheels)
	}
}

// The bar view declares, paints and hovers with the same model.
func TestBarViewPointerHover(t *testing.T) {
	h := openBar(t)
	r := h.box.Rect()
	h.move(r.X+r.Width/2, r.Y+r.Height/2)
	index, active := h.index()
	if !active || index < 0 {
		t.Fatalf("bar hover = (%d, %v), want an active category", index, active)
	}
	if h.hover != index || !h.active {
		t.Fatalf("OnHover = (%d, %v), want (%d, true)", h.hover, h.active, index)
	}
	if err := h.drv.Leave(); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if index, active := h.index(); active || index != -1 {
		t.Fatalf("bar hover after leaving = (%d, %v), want (-1, false)", index, active)
	}
}

func paintPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 320}, view)
	canvas.EndFrame()
}

// A warm chart paint adds no allocation of its own: the view's handlers and
// draw function are made once, and the components painter reuses ChartState.
func BenchmarkAreaViewPaint(b *testing.B) {
	view := &chartview.AreaChartView{Chart: areaChart()}
	viewFn := func(c ui.C) {
		chartview.View(c, "Visitors", view).Size(400, 240)
	}
	var tree gfx.Tree
	page := ui.NewPage(ui.PageConfig{Tree: &tree, View: viewFn})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	paintPage(page, canvas, viewFn)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		paintPage(page, canvas, viewFn)
	}
}

var containerHeights = [...]float32{0.42, 0.28, 0.60, 0.36, 0.72, 0.48, 0.88, 0.62}

func newContainer() *chartview.Container {
	return &chartview.Container{
		Series: []chartview.Series{
			{Key: "native", Label: "Native", Color: gfx.Color{Red: .2, Green: .52, Blue: .96, Alpha: 1}},
			{Key: "web", Label: "Web", Color: gfx.Color{Red: .53, Green: .36, Blue: .92, Alpha: 1}},
		},
		Options: chartview.ContainerOptions{Legend: chartview.LegendBottom},
		Tooltip: &chartview.Tooltip{
			Rows: []chartview.TooltipRow{{Series: 0, Value: "42"}, {Series: 1, Value: "28"}},
			Options: chartview.TooltipOptions{
				Label: "Week 4", Indicator: chartview.TooltipDot,
			},
		},
		Draw: func(d *ui.DrawContext, layout chartview.Layout) {
			plot := layout.PlotBounds
			baseline := plot.Y + plot.Height - 14
			barWidth := min(float32(30), (plot.Width-40)/8)
			gap := (plot.Width - barWidth*8) / 9
			for index, scale := range containerHeights {
				x := plot.X + gap + float32(index)*(barWidth+gap)
				height := (plot.Height - 34) * scale
				d.Canvas.Rounded(gfx.Rect{X: x, Y: baseline - height, Width: barWidth, Height: height}, 3, gfx.Color{Red: .2, Green: .4, Blue: .8, Alpha: 1})
			}
		},
	}
}

// ContainerView paints the container, hands Draw the resolved plot, and paints
// the tooltip over it.
func TestContainerViewPaint(t *testing.T) {
	container := newContainer()
	drawn, bounds := 0, gfx.Rect{}
	container.Draw = func(d *ui.DrawContext, layout chartview.Layout) {
		drawn++
		bounds = layout.Bounds
		d.Canvas.Fill(layout.PlotBounds, gfx.Color{Alpha: .1})
	}
	viewFn := func(c ui.C) {
		chartview.ContainerView(c, "Commits", container).Size(400, 240)
	}
	var tree gfx.Tree
	page := ui.NewPage(ui.PageConfig{Tree: &tree, View: viewFn})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	paintPage(page, canvas, viewFn)

	if drawn != 1 {
		t.Fatalf("Draw called %d times during one paint, want 1", drawn)
	}
	if bounds.Width != 400 || bounds.Height != 240 {
		t.Fatalf("container bounds = %v, want 400x240", bounds)
	}
	var legend, label, value bool
	for _, text := range canvas.Texts() {
		switch text.Value {
		case "Native":
			legend = true
		case "Week 4":
			label = true
		case "42":
			value = true
		}
	}
	if !legend {
		t.Fatal("the container did not paint its legend")
	}
	if !label || !value {
		t.Fatalf("the tooltip did not paint its label and row: label=%v value=%v", label, value)
	}
}

// A warm container paint adds no allocation: the draw function and handlers are
// made once with the container.
func BenchmarkContainerViewPaint(b *testing.B) {
	container := newContainer()
	viewFn := func(c ui.C) {
		chartview.ContainerView(c, "Commits", container).Size(400, 240)
	}
	var tree gfx.Tree
	page := ui.NewPage(ui.PageConfig{Tree: &tree, View: viewFn})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	paintPage(page, canvas, viewFn)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		paintPage(page, canvas, viewFn)
	}
}
