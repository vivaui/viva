package metriccardview_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/metriccardview"
)

var testIcon = gfx.Icon{ViewBox: 24, Stroke: 2, Cap: gfx.CapRound, Join: gfx.JoinRound, Commands: []gfx.PathCommand{
	gfx.MoveTo(4, 20), gfx.LineTo(20, 4),
}}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 360, Height: 240}, view)
	canvas.EndFrame()
}

func open(t testing.TB, view func(ui.C)) (*ui.Page, *gfx.Canvas) {
	t.Helper()
	canvas := new(gfx.Canvas)
	canvas.SetReduceMotion(true)
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	draw(page, canvas, view)
	return page, canvas
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func textColor(canvas *gfx.Canvas, value string) (gfx.Color, bool) {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return text.Style.Color, true
		}
	}
	return gfx.Color{}, false
}

// The view paints every part of the metric with the page's theme, including the
// description, the icon and the caller's content.
func TestViewPaintsTitleValueTrendDescriptionIconAndContent(t *testing.T) {
	theme := gfx.LightTheme()
	metric := metriccardview.New().
		Title("Active sessions").Value("12,840").
		Trend("+18.2% from last week").Up().
		Description("Compared with the previous 7 days").
		Icon(testIcon).
		Content(func(c ui.C) { ui.Text(c, "Extra note") })
	view := func(c ui.C) {
		metriccardview.View(c, "Active sessions", metric).FillWidth().Height(150)
	}
	_, canvas := open(t, view)

	for _, value := range []string{
		"Active sessions", "12,840", "+18.2% from last week",
		"Compared with the previous 7 days", "Extra note",
	} {
		if !hasText(canvas, value) {
			t.Fatalf("the card did not paint %q", value)
		}
	}
	if color, ok := textColor(canvas, "+18.2% from last week"); !ok || color != theme.Success {
		t.Fatalf("up trend color = %+v, want the theme's success color", color)
	}
	if len(canvas.Triangles()) == 0 {
		t.Fatal("the card did not paint its icon")
	}
}

// Up and Down are the caller's direction choice, and Neutral is the default.
func TestViewPaintsTrendDirection(t *testing.T) {
	for _, test := range []struct {
		name  string
		apply func(metriccardview.Metric) metriccardview.Metric
		want  gfx.Color
	}{
		{"up", func(m metriccardview.Metric) metriccardview.Metric { return m.Up() }, gfx.LightTheme().Success},
		{"down", func(m metriccardview.Metric) metriccardview.Metric { return m.Down() }, gfx.LightTheme().Danger},
		{"neutral", func(m metriccardview.Metric) metriccardview.Metric { return m.Neutral() }, gfx.LightTheme().MutedForeground},
	} {
		t.Run(test.name, func(t *testing.T) {
			metric := test.apply(metriccardview.New().Title("Metric").Value("1").Trend("delta"))
			view := func(c ui.C) {
				metriccardview.View(c, "Metric", metric).FillWidth().Height(120)
			}
			_, canvas := open(t, view)
			if color, ok := textColor(canvas, "delta"); !ok || color != test.want {
				t.Fatalf("trend color = %+v, want %+v", color, test.want)
			}
		})
	}
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	metric := metriccardview.New().
		Title("Active sessions").Value("12,840").
		Trend("+18.2% from last week").Up().
		Description("Compared with the previous 7 days").
		Icon(testIcon)
	view := func(c ui.C) {
		metriccardview.View(c, "Active sessions", metric).FillWidth().Height(150)
	}
	page, canvas := open(t, view)
	if allocations := testing.AllocsPerRun(100, func() { draw(page, canvas, view) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	metric := metriccardview.New().
		Title("Active sessions").Value("12,840").
		Trend("+18.2% from last week").Up().
		Description("Compared with the previous 7 days").
		Icon(testIcon)
	view := func(c ui.C) {
		metriccardview.View(c, "Active sessions", metric).FillWidth().Height(150)
	}
	page, canvas := open(b, view)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		draw(page, canvas, view)
	}
}
