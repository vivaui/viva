package components

import (
	"math"
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestChartAreaStyleOpacityAndGradient(t *testing.T) {
	color := gfx.RGB(80, 180, 220)
	close := func(a, b float32) bool { return math.Abs(float64(a-b)) < .00001 }
	var gradient gfx.LinearGradient
	defaults := chartAreaFillStyle(&gradient, ChartFillGradient, color, 10, 100, nil)
	if defaults.FillGradient == nil || !close(gradient.From.Alpha, .32) || !close(gradient.To.Alpha, .04) {
		t.Fatalf("defaults changed: %+v", gradient)
	}
	for _, opacity := range []float32{0, .2, 1, 2, -1, float32(math.NaN()), float32(math.Inf(1))} {
		expected := min(max(opacity, 0), 1)
		if !chartFinite(opacity) {
			expected = 0
		}
		fill := chartAreaFillStyle(&gradient, ChartFillSolid, color, 10, 100, &ChartAreaStyle{Opacity: opacity})
		if fill.Fill.Alpha != expected || fill.Fill.Red != color.Red {
			t.Fatalf("opacity %v: %+v", opacity, fill)
		}
	}
	from, to := gfx.RGB(100, 10, 200), gfx.Color{Red: .2, Blue: 1, Alpha: .1}
	for _, bounds := range [][2]float32{{10, 100}, {100, 10}, {10, 10}, {10, 10.1}, {10.1, 10}} {
		style := ChartAreaStyle{Opacity: .25, Gradient: &ChartAreaGradient{From: from, To: to}}
		chartAreaFillStyle(&gradient, ChartFillGradient, color, bounds[0], bounds[1], &style)
		if !close(gradient.From.Alpha, .25) || !close(gradient.To.Alpha, .025) || gradient.From.Red != from.Red || gradient.To.Blue != to.Blue {
			t.Fatalf("custom gradient: %+v", gradient)
		}
		if !chartFinite(gradient.Start.Y) || !chartFinite(gradient.End.Y) || gradient.Start == gradient.End {
			t.Fatal("degenerate gradient")
		}
		if bounds[1] < bounds[0] && gradient.End.Y >= gradient.Start.Y {
			t.Fatal("subpixel negative area inverted its gradient")
		}
	}
	override := gfx.RGB(255, 0, 0)
	fill := chartAreaFillStyle(&gradient, ChartFillSolid, color, 10, 100, &ChartAreaStyle{Color: override, Opacity: .5})
	if fill.Fill.Red != override.Red || fill.Fill.Green != 0 || fill.Fill.Alpha != .5 {
		t.Fatal("fill color override changed")
	}
}

func TestTransparentAreaDoesNotTessellateAnInvisibleFill(t *testing.T) {
	theme := gfx.LightTheme()
	chart := chartTestArea(theme)
	chart.HideTooltip = true
	chart.YAxis = ChartAxis{Fixed: true, Min: 0, Max: 400}
	var canvas gfx.Canvas
	chart.Fill = ChartFillNone
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 320, Height: 180}, gfx.Item(1), chart, nil)
	lines := slices.Clone(canvas.Triangles())
	canvas.Reset()
	area := ChartAreaStyle{Opacity: 0}
	for i := range chart.Series {
		chart.Series[i].AreaStyle = &area
	}
	chart.Fill = ChartFillGradient
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 320, Height: 180}, gfx.Item(1), chart, nil)
	if !slices.Equal(lines, canvas.Triangles()) {
		t.Fatal("transparent series changed the line or tessellated a fill")
	}
}

func TestChartCustomAreaStyleGapsClipAndSteadyPaint(t *testing.T) {
	theme := gfx.LightTheme()
	area := ChartAreaStyle{Opacity: .2, Gradient: &ChartAreaGradient{From: theme.Chart[0], To: gfx.Color{}}}
	chart := AreaChart{Series: []ChartSeries{{Key: "cpu", Color: theme.Chart[0], AreaStyle: &area}}, X: []float32{-5, 0, 1, 2, 3, 4, 5}, Values: [][]float32{{15, 25, 35, float32(math.NaN()), 45, 55, 65}},
		Curve: ChartCurveMonotone, XAxis: ChartAxis{Hidden: true, Fixed: true, Min: 0, Max: 4}, YAxis: ChartAxis{Hidden: true, Fixed: true, Min: 0, Max: 100}, HideTooltip: true}
	var canvas gfx.Canvas
	var state ChartState
	paint := func() {
		canvas.Reset()
		PaintAreaChart(&canvas, theme, gfx.Rect{X: 40, Y: 70, Width: 300, Height: 140}, gfx.Item(1), chart, &state)
	}
	paint()
	paint()
	plot := PaintAreaChart(&canvas, theme, gfx.Rect{X: 40, Y: 70, Width: 300, Height: 140}, gfx.Item(1), chart, &state).Plot
	if len(canvas.Triangles()) == 0 {
		t.Fatal("custom area style painted nothing")
	}
	for _, triangle := range canvas.Triangles() {
		if !triangle.HasClip || triangle.ClipOf != plot {
			t.Fatalf("series escaped the fixed domain: %+v", triangle.ClipOf)
		}
	}
	spec := chartSpec{series: chart.Series, values: chart.Values}
	if start, end := chartNextRun(&spec, 0, 0, 7); start != 0 || end != 2 {
		t.Fatalf("first run=%d..%d", start, end)
	}
	if start, end := chartNextRun(&spec, 0, 3, 7); start != 4 || end != 6 {
		t.Fatalf("gap was connected: %d..%d", start, end)
	}
	if allocs := testing.AllocsPerRun(30, paint); allocs != 0 {
		t.Fatalf("custom area paint allocations=%v", allocs)
	}
}
