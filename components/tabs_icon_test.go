package components

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTabsIconsCenterWithLabelAndFit(t *testing.T) {
	icons := []gfx.Icon{
		{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(2, 12), gfx.LineTo(22, 12)}},
		{ViewBox: 24, Layers: []gfx.IconLayer{{Fill: gfx.RGB(200, 0, 0), Commands: []gfx.PathCommand{gfx.MoveTo(2, 2), gfx.LineTo(22, 2), gfx.LineTo(12, 22), gfx.ClosePath()}}}},
	}
	for _, icon := range icons {
		for _, width := range []float32{240, 300, 600} {
			var canvas gfx.Canvas
			theme := gfx.LightTheme()
			theme.Metrics.FontSizeM = 12
			canvas.SetMeasurer(chartTestMeasure)
			options := []TabOption{{ID: gfx.Item(1), Label: "Overview", Icon: icon}, {ID: gfx.Item(2), Label: "Disk", Icon: icon}, {ID: gfx.Item(3), Label: "Network", Icon: icon}}
			paint := func() {
				canvas.Reset()
				BeginTabs(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: width, Height: 36}, options, gfx.Item(1)).Paint()
			}
			paint()
			paint()
			for i, text := range canvas.Texts() {
				trigger := canvas.Hits()[i].Bounds
				size, gap := float32(12), float32(6)
				groupLeft := text.Bounds.X - size - gap
				center := (groupLeft + text.Bounds.X + text.Bounds.Width) / 2
				if math.Abs(float64(center-(trigger.X+trigger.Width/2))) > .001 || text.Bounds.Y != trigger.Y || text.Bounds.Height != trigger.Height {
					t.Fatalf("tab %d not centered: text=%+v trigger=%+v", i, text.Bounds, trigger)
				}
			}
			if len(canvas.Triangles()) == 0 {
				t.Fatal("tab icon not painted")
			}
			if allocations := testing.AllocsPerRun(30, paint); allocations != 0 {
				t.Fatalf("icon tabs allocations=%v", allocations)
			}
			canvas.Reset()
			BeginTabs(&canvas, theme, gfx.Rect{}, options, gfx.Item(1)).Fit().Paint()
			want := canvas.TextWidthWeight(options[0].Label, 12, gfx.WeightMedium) + 12 + 6 + tabsTriggerPad*2
			if got := canvas.Hits()[0].Bounds.Width; got != want {
				t.Fatalf("Fit icon width=%v want%v", got, want)
			}
		}
	}
}

func TestTabIconWithoutTextIsCenteredEvenInAShortTrigger(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Layers: []gfx.IconLayer{{Fill: gfx.RGB(0, 100, 200), Commands: []gfx.PathCommand{gfx.MoveTo(2, 2), gfx.LineTo(22, 2), gfx.LineTo(22, 22), gfx.LineTo(2, 22), gfx.ClosePath()}}}}
	for _, height := range []float32{8, 26, 36} {
		var canvas gfx.Canvas
		theme := gfx.LightTheme()
		option := TabOption{ID: gfx.Item(1), Icon: icon}
		tabs := BeginTabs(&canvas, theme, gfx.Rect{}, nil, gfx.ID{})
		trigger := gfx.Rect{X: 30, Y: 45, Width: 100, Height: height}
		tabs.paintContent(option, trigger, gfx.TextStyle{Size: theme.Metrics.FontSizeM, Weight: gfx.WeightMedium, Color: theme.Foreground})
		lowX, lowY, highX, highY := float32(math.MaxFloat32), float32(math.MaxFloat32), float32(-math.MaxFloat32), float32(-math.MaxFloat32)
		for _, triangle := range canvas.Triangles() {
			for _, p := range triangle.Points {
				lowX = min(lowX, p.X)
				highX = max(highX, p.X)
				lowY = min(lowY, p.Y)
				highY = max(highY, p.Y)
			}
		}
		if math.Abs(float64((lowX+highX)/2-(trigger.X+trigger.Width/2))) > .001 || math.Abs(float64((lowY+highY)/2-(trigger.Y+trigger.Height/2))) > .001 {
			t.Fatal("icon-only tab is not centered")
		}
	}
}

// An icon-only tab still needs a name: the label is hidden, not dropped from
// the accessibility tree.
func TestIconOnlyTabKeepsItsAccessibleName(t *testing.T) {
	icon := gfx.Icon{ViewBox: 24, Commands: []gfx.PathCommand{gfx.MoveTo(2, 12), gfx.LineTo(22, 12)}}
	var canvas gfx.Canvas
	theme := gfx.LightTheme()
	canvas.SetMeasurer(chartTestMeasure)
	options := []TabOption{
		{ID: gfx.Item(1), Label: "Overview", Icon: icon, IconOnly: true},
		{ID: gfx.Item(2), Label: "Disk", Icon: icon, IconOnly: true},
	}
	BeginTabs(&canvas, theme, gfx.Rect{X: 10, Y: 20, Width: 300, Height: 36}, options, gfx.Item(1)).Paint()
	if texts := canvas.Texts(); len(texts) != 0 {
		t.Fatalf("icon-only tabs painted %d labels", len(texts))
	}
	hits := canvas.Hits()
	if len(hits) != 2 || hits[0].Label != "Overview" || hits[1].Label != "Disk" {
		t.Fatalf("icon-only tabs lost their names: %+v", hits)
	}
	if hits[0].Hint != hits[0].Label || hits[1].Hint != hits[1].Label {
		t.Fatalf("icon-only tabs lost their hover descriptions: %+v", hits)
	}
	canvas.Reset()
	BeginTabs(&canvas, theme, gfx.Rect{}, nil, gfx.ID{}).Fit()
	for index, option := range options {
		width := tabsTriggerPad*2 + theme.Metrics.FontSizeM
		if got := (Tabs{canvas: &canvas, theme: theme, options: options, fit: true}).contentWidth(option) + tabsTriggerPad*2; got != width {
			t.Fatalf("tab %d icon-only Fit width=%v want %v", index, got, width)
		}
	}
}
