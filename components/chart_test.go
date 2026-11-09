package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func chartTestMeasure(value string, _, _ float32) float32 {
	return float32(len(value) * 5)
}

func TestChartContainerReservesLegendAndReturnsPlotBounds(t *testing.T) {
	theme := gfx.LightTheme()
	series := []ChartSeries{
		{Key: "desktop", Label: "Desktop", Color: theme.Chart[0]},
		{Key: "mobile", Label: "Mobile", Color: theme.Chart[1]},
		{Key: "hidden", Label: "Hidden", Color: theme.Chart[2], Hidden: true},
	}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	layout := PaintChartContainer(&canvas, theme, gfx.Rect{X: 4, Y: 8, Width: 320, Height: 200}, series, ChartContainerOptions{Legend: ChartLegendBottom})
	if layout.Bounds != (gfx.Rect{X: 4, Y: 8, Width: 320, Height: 200}) {
		t.Fatalf("container bounds = %+v", layout.Bounds)
	}
	if layout.PlotBounds != (gfx.Rect{X: 4, Y: 8, Width: 320, Height: 172}) {
		t.Fatalf("plot bounds = %+v", layout.PlotBounds)
	}
	if layout.LegendBounds != (gfx.Rect{X: 4, Y: 180, Width: 320, Height: 28}) {
		t.Fatalf("legend bounds = %+v", layout.LegendBounds)
	}
	if len(canvas.Rectangles()) != 2 || canvas.Rectangles()[0].Color != theme.Chart[0] || canvas.Rectangles()[1].Color != theme.Chart[1] {
		t.Fatalf("legend swatches = %+v", canvas.Rectangles())
	}
	if len(canvas.Texts()) != 2 || canvas.Texts()[0].Value != "Desktop" || canvas.Texts()[1].Value != "Mobile" {
		t.Fatalf("legend labels = %+v", canvas.Texts())
	}
	if canvas.Rectangles()[0].Bounds.Width != 8 || canvas.Rectangles()[1].Bounds.X-(canvas.Texts()[0].Bounds.X+canvas.Texts()[0].Bounds.Width) != 16 {
		t.Fatalf("legend spacing = %+v, %+v", canvas.Rectangles(), canvas.Texts())
	}
}

func TestChartContainerUsesOfficialInitialDimensionsAndTopLegend(t *testing.T) {
	var canvas gfx.Canvas
	layout := PaintChartContainer(&canvas, gfx.LightTheme(), gfx.Rect{X: 3, Y: 5}, []ChartSeries{{Label: "Value"}}, ChartContainerOptions{Legend: ChartLegendTop})
	if layout.Bounds != (gfx.Rect{X: 3, Y: 5, Width: 320, Height: 200}) {
		t.Fatalf("initial bounds = %+v", layout.Bounds)
	}
	if layout.PlotBounds != (gfx.Rect{X: 3, Y: 33, Width: 320, Height: 172}) {
		t.Fatalf("top plot bounds = %+v", layout.PlotBounds)
	}
}

func TestChartTooltipPaintsDotLineAndDashedIndicators(t *testing.T) {
	theme := gfx.LightTheme()
	series := []ChartSeries{{Label: "Desktop", Color: theme.Chart[1]}}
	rows := []ChartTooltipRow{{Series: 0, Value: "1,234"}}
	tests := []struct {
		name       string
		indicator  ChartTooltipIndicator
		wantWidth  float32
		wantHeight float32
		wantRects  int
	}{
		{name: "dot", indicator: ChartTooltipDot, wantWidth: 128, wantHeight: 50, wantRects: 3},
		{name: "line", indicator: ChartTooltipLine, wantWidth: 128, wantHeight: 50, wantRects: 3},
		{name: "dashed", indicator: ChartTooltipDashed, wantWidth: 128, wantHeight: 50, wantRects: 9},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var canvas gfx.Canvas
			canvas.SetMeasurer(chartTestMeasure)
			bounds := PaintChartTooltip(&canvas, theme, gfx.Rect{X: 10, Y: 20}, series, rows, ChartTooltipOptions{
				Label: "June", Indicator: test.indicator, Layer: 7,
			})
			if bounds.Width != test.wantWidth || bounds.Height != test.wantHeight {
				t.Fatalf("tooltip bounds = %+v", bounds)
			}
			if len(canvas.Rectangles()) != test.wantRects {
				t.Fatalf("tooltip rectangles = %+v", canvas.Rectangles())
			}
			for _, rectangle := range canvas.Rectangles() {
				if rectangle.Layer != 7 {
					t.Fatalf("rectangle layer = %d", rectangle.Layer)
				}
			}
			texts := canvas.Texts()
			if len(texts) != 3 || texts[0].Value != "June" || texts[1].Value != "Desktop" || texts[2].Value != "1,234" {
				t.Fatalf("tooltip texts = %+v", texts)
			}
			if test.indicator != ChartTooltipDot && texts[0].Bounds.X != texts[1].Bounds.X {
				t.Fatalf("nested label x positions = %v and %v", texts[0].Bounds.X, texts[1].Bounds.X)
			}
		})
	}
}

func TestChartTooltipRowsUseSeriesMetadataAndOverrides(t *testing.T) {
	theme := gfx.DarkTheme()
	series := []ChartSeries{
		{Label: "Desktop", Color: theme.Chart[0]},
		{Label: "Mobile", Color: theme.Chart[4]},
	}
	rows := []ChartTooltipRow{
		{Series: 0, Value: "80"},
		{Series: 1, Label: "Phone", Value: "20"},
		{Series: 1, Label: "Ignored", Value: "0", Hidden: true},
	}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	bounds := PaintChartTooltip(&canvas, theme, gfx.Rect{X: 1, Y: 2}, series, rows, ChartTooltipOptions{Label: "Traffic", Indicator: ChartTooltipDot})
	if bounds != (gfx.Rect{X: 1, Y: 2, Width: 128, Height: 72}) {
		t.Fatalf("tooltip bounds = %+v", bounds)
	}
	texts := canvas.Texts()
	if len(texts) != 5 || texts[1].Value != "Desktop" || texts[3].Value != "Phone" {
		t.Fatalf("resolved labels = %+v", texts)
	}
	rectangles := canvas.Rectangles()
	if rectangles[2].Color != theme.Chart[0] || rectangles[3].Color != theme.Chart[4] {
		t.Fatalf("indicator colors = %+v", rectangles)
	}
}

func TestChartTooltipEmptyPayloadPaintsNothing(t *testing.T) {
	var canvas gfx.Canvas
	if bounds := PaintChartTooltip(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20}, nil, nil, ChartTooltipOptions{Label: "June"}); bounds != (gfx.Rect{}) {
		t.Fatalf("empty tooltip bounds = %+v", bounds)
	}
	if len(canvas.Rectangles()) != 0 || len(canvas.Texts()) != 0 {
		t.Fatalf("empty tooltip painted rectangles=%d texts=%d", len(canvas.Rectangles()), len(canvas.Texts()))
	}
}

func TestChartPaintIsAllocationFreeAfterWarmup(t *testing.T) {
	theme := gfx.LightTheme()
	series := []ChartSeries{
		{Key: "desktop", Label: "Desktop", Color: theme.Chart[0]},
		{Key: "mobile", Label: "Mobile", Color: theme.Chart[1]},
	}
	rows := []ChartTooltipRow{{Series: 0, Value: "80"}, {Series: 1, Value: "20"}}
	var canvas gfx.Canvas
	canvas.SetMeasurer(chartTestMeasure)
	paint := func() {
		canvas.Reset()
		PaintChartContainer(&canvas, theme, gfx.Rect{Width: 320, Height: 200}, series, ChartContainerOptions{Legend: ChartLegendBottom})
		PaintChartTooltip(&canvas, theme, gfx.Rect{X: 40, Y: 30}, series, rows, ChartTooltipOptions{Label: "Traffic", Indicator: ChartTooltipDashed, Layer: 4})
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("chart paint allocations = %v", allocations)
	}
}
