package components

import (
	"math"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func chartTestSeries(theme gfx.Theme) []ChartSeries {
	return []ChartSeries{
		{Key: "desktop", Label: "Desktop", Color: theme.Chart[0]},
		{Key: "mobile", Label: "Mobile", Color: theme.Chart[1]},
	}
}

var chartTestMonths = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"}

func chartTestMonth(dst []byte, v float32) []byte { return append(dst, chartTestMonths[int(v)]...) }

func chartTestArea(theme gfx.Theme) AreaChart {
	return AreaChart{
		Series: chartTestSeries(theme),
		Values: [][]float32{{186, 305, 237, 73, 209, 214}, {80, 200, 120, 190, 130, 140}},
		XAxis:  ChartAxis{Format: chartTestMonth},
		Grid:   ChartGridHorizontal,
		Legend: ChartLegendBottom,
	}
}

func TestChartNiceDomain(t *testing.T) {
	low, high, step := chartNiceDomain(0, 305, 5)
	if low != 0 || high != 350 || step != 50 {
		t.Fatalf("domain = %v %v step %v", low, high, step)
	}
	low, high, _ = chartNiceDomain(7, 7, 4)
	if !(low < 7 && high > 7) {
		t.Fatalf("flat domain = %v %v", low, high)
	}
	if low, high, _ = chartNiceDomain(float32(math.NaN()), 1, 4); low != 0 || high != 1 {
		t.Fatalf("NaN domain = %v %v", low, high)
	}
	if chartTickDecimals(0.25) != 2 || chartTickDecimals(50) != 0 {
		t.Fatalf("decimals = %d %d", chartTickDecimals(0.25), chartTickDecimals(50))
	}
}

func TestChartStackedSpanAndDomain(t *testing.T) {
	spec := chartSpec{
		series: []ChartSeries{{}, {}, {Hidden: true}},
		values: [][]float32{{1, 2}, {3, float32(math.NaN())}, {100, 100}},
		stack:  ChartStackStacked,
	}
	if base, top := spec.span(1, 0); base != 1 || top != 4 {
		t.Fatalf("span = %v %v", base, top)
	}
	if base, top := spec.span(1, 1); base != 2 || top != 2 {
		t.Fatalf("gap counts as zero: %v %v", base, top)
	}
	if low, high := spec.domain(); low != 0 || high != 4 {
		t.Fatalf("domain = %v %v", low, high)
	}
	spec.stack = ChartStackExpand
	if base, top := spec.span(0, 0); base != 0 || top != 0.25 {
		t.Fatalf("expand = %v %v", base, top)
	}
	if low, high := spec.domain(); low != 0 || high != 1 {
		t.Fatalf("expand domain = %v %v", low, high)
	}
}

func TestChartIndexAt(t *testing.T) {
	category := &chartGeometry{plot: gfx.Rect{X: 10, Width: 100}, count: 5, xMin: 0, xMax: 4}
	for x, want := range map[float32]int{-50: 0, 10: 0, 40: 1, 60: 2, 110: 4, 500: 4} {
		if got := category.indexAt(x); got != want {
			t.Fatalf("category indexAt(%v) = %d, want %d", x, got, want)
		}
	}
	numeric := &chartGeometry{plot: gfx.Rect{Width: 100}, count: 4, x: []float32{0, 1, 10, 100}, xMin: 0, xMax: 100}
	for x, want := range map[float32]int{0: 0, 0.4: 0, 0.8: 1, 6: 2, 54: 2, 70: 3, 100: 3} {
		if got := numeric.indexAt(x); got != want {
			t.Fatalf("numeric indexAt(%v) = %d, want %d", x, got, want)
		}
	}
	band := &chartGeometry{plot: gfx.Rect{Width: 100}, count: 4, band: true}
	if band.indexAt(24) != 0 || band.indexAt(26) != 1 || band.px(0) != 12.5 {
		t.Fatalf("band mapping: %d %d %v", band.indexAt(24), band.indexAt(26), band.px(0))
	}
	empty := chartGeometry{}
	if empty.indexAt(5) != -1 {
		t.Fatal("empty geometry has no index")
	}
}

func TestChartCachedGeometryFollowsContainerPosition(t *testing.T) {
	for _, hidden := range []bool{false, true} {
		theme := gfx.LightTheme()
		chart := chartTestArea(theme)
		chart.YAxis.Hidden = hidden
		var state ChartState
		var canvas gfx.Canvas
		bounds := gfx.Rect{X: 10, Y: 20, Width: 300, Height: 180}
		first := PaintAreaChart(&canvas, theme, bounds, gfx.ID{}, chart, &state)
		key := state.labels.key
		bounds.X += 640
		bounds.Y += 80
		canvas.Reset()
		second := PaintAreaChart(&canvas, theme, bounds, gfx.ID{}, chart, &state)
		if state.labels.key != key {
			t.Fatal("moving a chart invalidated its label cache")
		}
		if second.Plot.X != first.Plot.X+640 || second.Plot.Y != first.Plot.Y+80 || second.Plot.Width != first.Plot.Width || second.Plot.Height != first.Plot.Height {
			t.Fatalf("moved chart geometry: first=%+v second=%+v", first.Plot, second.Plot)
		}
		if got, want := second.PointX(2), first.PointX(2)+640; math.Abs(float64(got-want)) > .001 {
			t.Fatalf("data point=%v, want %v", got, want)
		}
		if got := second.IndexAt(second.PointX(2)); got != 2 {
			t.Fatalf("moved hit geometry=%d, want 2", got)
		}
	}
}

func TestChartStepForKey(t *testing.T) {
	cases := []struct {
		key          gfx.Key
		index, count int
		want         int
		handled      bool
	}{
		{gfx.KeyRight, -1, 6, 0, true}, {gfx.KeyLeft, -1, 6, 5, true},
		{gfx.KeyRight, 2, 6, 3, true}, {gfx.KeyRight, 5, 6, 5, true}, {gfx.KeyLeft, 0, 6, 0, true},
		{gfx.KeyHome, 3, 6, 0, true}, {gfx.KeyEnd, 3, 6, 5, true},
		{gfx.KeyUp, 3, 6, 3, false}, {gfx.KeyRight, 0, 0, 0, false},
	}
	for _, c := range cases {
		if got, handled := ChartStepForKey(c.key, c.index, c.count); got != c.want || handled != c.handled {
			t.Fatalf("ChartStepForKey(%v,%d,%d) = %d,%v want %d,%v", c.key, c.index, c.count, got, handled, c.want, c.handled)
		}
	}
}

func TestChartMonotoneDoesNotOvershoot(t *testing.T) {
	points := []gfx.Point{{X: 0, Y: 0}, {X: 1, Y: 0}, {X: 2, Y: 10}, {X: 3, Y: 10}, {X: 4, Y: 0}}
	var scratch chartScratch
	chartControls(ChartCurveMonotone, points, &scratch)
	for index := range points[:len(points)-1] {
		low := min(points[index].Y, points[index+1].Y)
		high := max(points[index].Y, points[index+1].Y)
		for _, control := range []gfx.Point{scratch.c1[index], scratch.c2[index]} {
			if control.Y < low-1e-4 || control.Y > high+1e-4 {
				t.Fatalf("segment %d control %v leaves [%v,%v]", index, control, low, high)
			}
		}
	}
}

func TestChartNaturalSplineIsSmooth(t *testing.T) {
	// A natural spline through collinear points is that line.
	points := []gfx.Point{{X: 0, Y: 0}, {X: 1, Y: 2}, {X: 2, Y: 4}, {X: 3, Y: 6}}
	var scratch chartScratch
	chartControls(ChartCurveNatural, points, &scratch)
	for index := 0; index < 3; index++ {
		for _, c := range []gfx.Point{scratch.c1[index], scratch.c2[index]} {
			if math.Abs(float64(c.Y-2*c.X)) > 1e-4 {
				t.Fatalf("control %v off the line", c)
			}
		}
	}
}

func TestPaintAreaChartDrawsSeriesAndGrid(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	plot := PaintAreaChart(&canvas, theme, gfx.Rect{X: 0, Y: 0, Width: 400, Height: 240}, gfx.Item(1), chartTestArea(theme), nil)
	if plot.Plot.Width <= 0 || plot.Plot.Height <= 0 || plot.Count() != 6 {
		t.Fatalf("plot = %+v count %d", plot.Plot, plot.Count())
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("series painted nothing")
	}
	var months int
	for _, text := range canvas.Texts() {
		for _, month := range chartTestMonths {
			if text.Value == month {
				months++
			}
		}
	}
	if months != len(chartTestMonths) {
		t.Fatalf("x labels painted = %d, want %d", months, len(chartTestMonths))
	}
	hits := canvas.Hits()
	if len(hits) != 1 || hits[0].ID != gfx.Item(1) || hits[0].Bounds != plot.Plot {
		t.Fatalf("hits = %+v", hits)
	}
}

func TestPaintAreaChartHoverShowsTooltip(t *testing.T) {
	theme := gfx.LightTheme()
	chart := chartTestArea(theme)
	var state ChartState
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	plot := PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	state.Hover(plot.IndexAt(plot.PointX(2)))
	canvas.Reset()
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	found := map[string]bool{}
	for _, text := range canvas.Texts() {
		found[text.Value] = true
	}
	for _, want := range []string{"237", "120", "Desktop", "Mobile"} {
		if !found[want] {
			t.Fatalf("tooltip text %q missing from %v", want, found)
		}
	}
	if hits := canvas.Hits(); len(hits) != 1 || hits[0].Value != "Mar, Desktop 237, Mobile 120" {
		t.Fatalf("hit value = %+v", hits)
	}
	chart.HideTooltip = true
	canvas.Reset()
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	for _, text := range canvas.Texts() {
		if text.Value == "237" {
			t.Fatal("hidden tooltip still painted")
		}
	}
}

func TestPaintAreaChartGapSplitsLine(t *testing.T) {
	theme := gfx.LightTheme()
	nan := float32(math.NaN())
	chart := AreaChart{Series: chartTestSeries(theme)[:1], Values: [][]float32{{1, 2, nan, 3, 4}}, Fill: ChartFillNone, XAxis: ChartAxis{Hidden: true}}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	plot := PaintAreaChart(&canvas, theme, gfx.Rect{Width: 300, Height: 200}, gfx.Item(1), chart, nil)
	gapX := plot.PointX(2)
	for _, triangle := range canvas.Triangles() {
		inside := 0
		for _, point := range triangle.Points {
			if point.X > gapX-1 && point.X < gapX+1 {
				inside++
			}
		}
		if inside == 3 {
			t.Fatalf("line crosses the gap: %+v", triangle.Points)
		}
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("segments around the gap not painted")
	}
}

func TestPaintAreaChartEmptyAndDegenerate(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 300, Height: 200}, gfx.Item(1), AreaChart{}, nil)
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 300, Height: 200}, gfx.Item(1), AreaChart{Series: chartTestSeries(theme)[:1], Values: [][]float32{{5}}}, nil)
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 4, Height: 4}, gfx.Item(1), chartTestArea(theme), nil)
	PaintAreaChart(nil, theme, gfx.Rect{}, gfx.Item(1), chartTestArea(theme), nil)
}

func TestPaintAreaChartStackedExpandAxis(t *testing.T) {
	theme := gfx.LightTheme()
	chart := chartTestArea(theme)
	chart.Stack = ChartStackExpand
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, nil)
	percent := false
	for _, text := range canvas.Texts() {
		if text.Value == "100%" {
			percent = true
		}
	}
	if !percent {
		t.Fatal("expanded chart has no 100% tick")
	}
}

func TestPaintAreaChartSteadyStateAllocatesNothing(t *testing.T) {
	theme := gfx.LightTheme()
	for name, curve := range map[string]ChartCurve{"linear": ChartCurveLinear, "natural": ChartCurveNatural, "monotone": ChartCurveMonotone, "step": ChartCurveStep} {
		chart := chartTestArea(theme)
		chart.Curve, chart.Stack = curve, ChartStackStacked
		var state ChartState
		var canvas gfx.Canvas
		canvas.SetMeasurer(chartTestMeasure)
		state.Hover(2)
		paint := func() {
			canvas.Reset()
			PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
		}
		paint()
		paint()
		if allocs := testing.AllocsPerRun(20, paint); allocs != 0 {
			t.Fatalf("%s: steady paint allocated %v times", name, allocs)
		}
	}
}

func chartBenchData(points, series int) ([][]float32, []float32) {
	values := make([][]float32, series)
	x := make([]float32, points)
	for index := range x {
		x[index] = float32(index)
	}
	for k := range values {
		values[k] = make([]float32, points)
		for index := range values[k] {
			values[k][index] = 100 + 50*float32(math.Sin(float64(index)/float64(10+k*7))) + float32(k*20)
		}
	}
	return values, x
}

func BenchmarkAreaChart(b *testing.B) {
	theme := gfx.LightTheme()
	for _, series := range []int{1, 2, 4} {
		for _, points := range []int{100, 1000, 10000} {
			b.Run(chartBenchName(series, points), func(b *testing.B) {
				values, x := chartBenchData(points, series)
				chart := AreaChart{Series: make([]ChartSeries, series), X: x, Values: values, Curve: ChartCurveMonotone, Grid: ChartGridHorizontal}
				for k := range chart.Series {
					chart.Series[k] = ChartSeries{Label: "s", Color: theme.Chart[k%5]}
				}
				var state ChartState
				var canvas gfx.Canvas
				canvas.SetMeasurer(chartTestMeasure)
				paint := func() {
					canvas.Reset()
					PaintAreaChart(&canvas, theme, gfx.Rect{Width: 640, Height: 320}, gfx.Item(1), chart, &state)
				}
				paint()
				b.ReportAllocs()
				b.ResetTimer()
				for iteration := 0; iteration < b.N; iteration++ {
					paint()
				}
			})
		}
	}
}

func BenchmarkAreaChartHover(b *testing.B) {
	theme := gfx.LightTheme()
	values, x := chartBenchData(1000, 2)
	chart := AreaChart{Series: chartTestSeries(theme), X: x, Values: values, Curve: ChartCurveMonotone, Grid: ChartGridHorizontal}
	var state ChartState
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	paint := func(index int) {
		state.Hover(index)
		canvas.Reset()
		PaintAreaChart(&canvas, theme, gfx.Rect{Width: 640, Height: 320}, gfx.Item(1), chart, &state)
	}
	paint(10)
	b.ReportAllocs()
	b.ResetTimer()
	for iteration := 0; iteration < b.N; iteration++ {
		paint(10 + iteration%2)
	}
}

func chartBenchName(series, points int) string {
	digits := func(value int) string {
		out := ""
		for value > 0 {
			out = string(rune('0'+value%10)) + out
			value /= 10
		}
		return out
	}
	return digits(series) + "series/" + digits(points) + "points"
}

func TestChartDownsample(t *testing.T) {
	const n = 10000
	y := make([]float32, n)
	for i := range y {
		y[i] = float32(math.Sin(float64(i) / 200))
	}
	y[4321] = 50 // a spike LTTB must keep
	got := ChartDownsample(nil, nil, y, 200)
	if len(got) < 150 || len(got) > 200 {
		t.Fatalf("kept %d points", len(got))
	}
	if got[0] != 0 || got[len(got)-1] != n-1 {
		t.Fatalf("ends = %d, %d", got[0], got[len(got)-1])
	}
	spike := false
	for i, index := range got {
		if i > 0 && index <= got[i-1] {
			t.Fatalf("indexes not ascending at %d", i)
		}
		spike = spike || index == 4321
	}
	if !spike {
		t.Fatal("downsampling dropped the spike")
	}
	if whole := ChartDownsample(nil, nil, y[:50], 200); len(whole) != 50 {
		t.Fatalf("short series reduced to %d", len(whole))
	}
	if len(ChartDownsample(nil, nil, nil, 10)) != 0 {
		t.Fatal("empty series")
	}
	nan := float32(math.NaN())
	gaps := []float32{nan, nan, nan, nan, nan, nan, 1, 2, nan, nan, nan, nan}
	if out := ChartDownsample(nil, nil, gaps, 4); len(out) < 2 || out[0] != 0 || out[len(out)-1] != len(gaps)-1 {
		t.Fatalf("gapped series = %v", out)
	}
}

func TestChartDownsampleReusesDestination(t *testing.T) {
	y := make([]float32, 5000)
	for i := range y {
		y[i] = float32(i % 37)
	}
	dst := ChartDownsample(nil, nil, y, 300)
	if allocs := testing.AllocsPerRun(10, func() { dst = ChartDownsample(dst, nil, y, 300) }); allocs != 0 {
		t.Fatalf("allocated %v times with a reused destination", allocs)
	}
}

func TestPaintBarChart(t *testing.T) {
	theme := gfx.LightTheme()
	chart := BarChart{
		Series: chartTestSeries(theme), Values: [][]float32{{186, 305, 237}, {80, 200, 120}},
		XAxis: ChartAxis{Format: chartTestMonth}, Grid: ChartGridHorizontal, Legend: ChartLegendBottom,
	}
	var state ChartState
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	plot := PaintBarChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	if plot.Count() != 3 || plot.IndexAt(plot.PointX(1)) != 1 {
		t.Fatalf("plot count %d", plot.Count())
	}
	grouped := len(canvas.Rectangles())
	if grouped == 0 {
		t.Fatal("no bars painted")
	}
	chart.Stack = ChartStackStacked
	canvas.Reset()
	PaintBarChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("stacked bars painted nothing")
	}
	state.Hover(1)
	canvas.Reset()
	PaintBarChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	found := false
	for _, text := range canvas.Texts() {
		found = found || text.Value == "305"
	}
	if !found {
		t.Fatal("hover tooltip missing")
	}
	PaintBarChart(&canvas, theme, gfx.Rect{Width: 4, Height: 4}, gfx.Item(2), BarChart{}, nil)
}

func TestPaintBarChartSteadyStateAllocatesNothing(t *testing.T) {
	theme := gfx.LightTheme()
	chart := BarChart{Series: chartTestSeries(theme), Values: [][]float32{{186, 305, 237}, {80, 200, 120}}, Grid: ChartGridHorizontal}
	var state ChartState
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	state.Hover(1)
	paint := func() {
		canvas.Reset()
		PaintBarChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, &state)
	}
	paint()
	paint()
	if allocs := testing.AllocsPerRun(20, paint); allocs != 0 {
		t.Fatalf("steady bar paint allocated %v times", allocs)
	}
}

// A series that crosses the baseline must fill both sides of it, each clipped to
// its own side, instead of one self-intersecting outline.
func TestPaintAreaChartFillsBothSidesOfTheBaseline(t *testing.T) {
	theme := gfx.LightTheme()
	for _, curve := range []ChartCurve{ChartCurveLinear, ChartCurveMonotone, ChartCurveNatural, ChartCurveStep} {
		chart := AreaChart{
			Series: chartTestSeries(theme)[:1], Curve: curve, XAxis: ChartAxis{Hidden: true},
			Values: [][]float32{{40, -30, 20, -50, 10, 60}},
		}
		var canvas gfx.Canvas
		canvas.SetMeasurer(chartTestMeasure)
		plot := PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, nil)
		baseline := plot.ValueY(0)
		above, below := 0, 0
		for _, triangle := range canvas.Triangles() {
			if !triangle.HasClip {
				continue
			}
			switch {
			case triangle.ClipOf.Y+triangle.ClipOf.Height <= baseline+0.5 && triangle.ClipOf.Y < baseline:
				above++
			case triangle.ClipOf.Y >= baseline-0.5:
				below++
			}
		}
		if above == 0 || below == 0 {
			t.Fatalf("curve %d: fill triangles above/below the baseline = %d/%d", curve, above, below)
		}
	}
	// Staying on one side needs no clipping.
	chart := AreaChart{Series: chartTestSeries(theme)[:1], Values: [][]float32{{-5, -30, -20, -10}}, XAxis: ChartAxis{Hidden: true}}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	PaintAreaChart(&canvas, theme, gfx.Rect{Width: 400, Height: 240}, gfx.Item(1), chart, nil)
	if len(canvas.Triangles()) == 0 {
		t.Fatal("negative-only series painted nothing")
	}
}

func TestStackedNegativeValuesStackAsZero(t *testing.T) {
	spec := chartSpec{series: []ChartSeries{{}, {}}, values: [][]float32{{5}, {-3}}, stack: ChartStackStacked}
	if base, top := spec.span(1, 0); base != 5 || top != 5 {
		t.Fatalf("negative stacked span = %v %v", base, top)
	}
	if low, high := spec.domain(); low != 0 || high != 5 {
		t.Fatalf("domain = %v %v", low, high)
	}
}

func TestLinearXAxisTicksAreRoundValues(t *testing.T) {
	theme := gfx.LightTheme()
	x := []float32{0, 13, 27, 31, 50, 77, 99}
	chart := AreaChart{
		Series: chartTestSeries(theme)[:1], X: x, Values: [][]float32{{1, 3, 2, 5, 4, 6, 5}},
		XAxis: ChartAxis{Linear: true},
	}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	plot := PaintAreaChart(&canvas, theme, gfx.Rect{Width: 480, Height: 240}, gfx.Item(1), chart, nil)
	round := map[string]bool{"0": true, "20": true, "40": true, "60": true, "80": true, "100": true, "25": true, "50": true, "75": true, "10": true, "30": true, "70": true, "90": true}
	seen := 0
	for _, text := range canvas.Texts() {
		if text.Bounds.Y > plot.Plot.Y+plot.Plot.Height {
			if !round[text.Value] {
				t.Fatalf("x tick %q is not a round value", text.Value)
			}
			seen++
		}
	}
	if seen < 3 {
		t.Fatalf("only %d x ticks", seen)
	}
}

func TestXAxisTimeLabelsStayInsideChart(t *testing.T) {
	fontSize := gfx.LightTheme().Metrics.FontSizeS
	for _, width := range []float32{60, 90, 180, 300, 480} {
		for _, stamp := range []string{"20:33:25", "10-29 20:33", "2026-10-29 20:33:25"} {
			var canvas gfx.Canvas
			canvas.SetMeasurer(chartTestMeasure)
			theme := gfx.LightTheme()
			bounds := gfx.Rect{X: 120, Y: 30, Width: width, Height: 200}
			chart := AreaChart{Series: chartTestSeries(theme)[:1], X: []float32{0, .5, 1}, Values: [][]float32{{10, 30, 20}}, XAxis: ChartAxis{Linear: true, Fixed: true, Min: 0, Max: 1, Ticks: 3, Format: func(dst []byte, value float32) []byte { return append(dst, stamp...) }}}
			plot := PaintAreaChart(&canvas, theme, bounds, gfx.Item(1), chart, nil)
			seen := 0
			truncated := false
			for _, text := range canvas.Texts() {
				if text.Bounds.Y <= plot.Plot.Y+plot.Plot.Height {
					continue
				}
				seen++
				if text.Bounds.X < bounds.X || text.Bounds.X+text.Bounds.Width > bounds.X+bounds.Width+.001 {
					t.Fatalf("time label %q escaped width %v: %+v", stamp, width, text.Bounds)
				}
				if painted := canvas.TextWidth(text.Value, fontSize); painted > text.Bounds.Width+.001 {
					t.Fatalf("time label %q painted %.1f into %.1f at width %v", text.Value, painted, text.Bounds.Width, width)
				}
				truncated = truncated || strings.HasSuffix(text.Value, "…")
			}
			if seen < 2 {
				t.Fatal("time-axis labels disappeared")
			}
			// The widest stamps cannot fit a narrow chart, so at least one label
			// must reach the ellipsis path this test exists to protect.
			if width <= 90 && len(stamp) > 12 && !truncated {
				t.Fatalf("time label %q was not truncated to fit width %v", stamp, width)
			}
		}
	}
}

func TestFocusedChartDescribesItsLatestPoint(t *testing.T) {
	theme := gfx.LightTheme()
	bounds := gfx.Rect{X: 10, Y: 10, Width: 320, Height: 200}
	chart := AreaChart{Series: chartTestSeries(theme)[:1], X: []float32{0, .5, 1}, Values: [][]float32{{10, 30, 20}}, XAxis: ChartAxis{Linear: true, Fixed: true, Min: 0, Max: 1, Ticks: 3, Format: func(dst []byte, _ float32) []byte { return append(dst, 't') }}}
	for _, focused := range []bool{false, true} {
		var canvas gfx.Canvas
		canvas.SetMeasurer(chartTestMeasure)
		if focused {
			canvas.SetInteraction(gfx.ID{}, gfx.ID{}, gfx.Item(1), true)
		}
		PaintAreaChart(&canvas, theme, bounds, gfx.Item(1), chart, nil)
		hits := canvas.Hits()
		if len(hits) != 1 {
			t.Fatalf("chart hits = %d", len(hits))
		}
		if focused {
			if value := hits[0].Value; !strings.Contains(value, "Desktop") || !strings.Contains(value, "20") {
				t.Fatalf("focused chart value = %q, want the latest point", value)
			}
		} else if hits[0].Value != "" {
			t.Fatalf("unfocused chart value = %q, want nothing until a point is chosen", hits[0].Value)
		}
	}
}
