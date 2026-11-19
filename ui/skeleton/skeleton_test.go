package skeleton_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/skeleton"
)

type model struct {
	block, text, circle, faded skeleton.Skeleton
	page                       *ui.Page
	canvas                     *gfx.Canvas
	theme                      gfx.Theme
}

func (m *model) viewPage(c ui.C) {
	// A page reconfigures the placeholders every paint; the draw functions and
	// any storage stay on the values in the model.
	m.block.Reset().Block().Size(100, 10).Opacity(1)
	m.text.Reset().Text().Size(120, 14).Opacity(1)
	m.circle.Reset().Circle().Size(40, 56).Opacity(1)
	m.faded.Reset().Block().Size(80, 8).Opacity(0.5)
	ui.Column(c).Gap(6).Children(func() {
		skeleton.View(c, "Block placeholder", &m.block)
		skeleton.View(c, "Text placeholder", &m.text)
		skeleton.View(c, "Circle placeholder", &m.circle)
		skeleton.View(c, "Faded placeholder", &m.faded)
	})
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 300}, view)
	canvas.EndFrame()
}

func open(t testing.TB, m *model) {
	t.Helper()
	m.theme = gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &m.theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

// Block, Text and Circle paint one neutral surface each with the sizes and
// radii the page declared; the colored surface carries the explicit opacity.
func TestPrimitivesPaintDeclaredSizeRadiusAndOpacity(t *testing.T) {
	m := new(model)
	open(t, m)

	rectangles := m.canvas.Rectangles()
	if len(rectangles) != 4 {
		t.Fatalf("painted %d rectangles, want 4", len(rectangles))
	}
	block, text, circle, faded := rectangles[0], rectangles[1], rectangles[2], rectangles[3]

	if block.Bounds.Width != 100 || block.Bounds.Height != 10 {
		t.Fatalf("block bounds = %+v, want 100x10", block.Bounds)
	}
	if block.Radius != m.theme.Metrics.RadiusControl {
		t.Fatalf("block radius = %g, want the control radius", block.Radius)
	}
	if text.Radius != 4 {
		t.Fatalf("text radius = %g, want 4", text.Radius)
	}
	if text.Bounds.Width != 120 || text.Bounds.Height != 14 {
		t.Fatalf("text bounds = %+v, want 120x14", text.Bounds)
	}
	if circle.Bounds.Width != 40 || circle.Bounds.Height != 40 || circle.Radius != 20 {
		t.Fatalf("circle = %+v radius %g, want 40x40 radius 20", circle.Bounds, circle.Radius)
	}
	if !closeTo(faded.Color.Alpha, m.theme.Muted.Alpha*0.5) {
		t.Fatalf("faded alpha = %g, want %g", faded.Color.Alpha, m.theme.Muted.Alpha*0.5)
	}
	if block.Color.Alpha != m.theme.Muted.Alpha {
		t.Fatalf("block alpha = %g, want %g", block.Color.Alpha, m.theme.Muted.Alpha)
	}
	if hits := m.canvas.Hits(); len(hits) != 0 {
		t.Fatalf("skeleton declared %d hits, want none", len(hits))
	}
	if m.canvas.Animating() {
		t.Fatal("static placeholder requested another frame")
	}
}

// Radius overrides the circle's radius without changing its square bounds.
func TestRadiusOverridesCircle(t *testing.T) {
	m := &model{theme: gfx.LightTheme()}
	m.circle.Reset().Circle().Size(40, 56).Radius(7)
	m.page = ui.NewPage(ui.PageConfig{Theme: &m.theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, func(c ui.C) {
		skeleton.View(c, "Circle", &m.circle)
	})
	rectangles := m.canvas.Rectangles()
	if len(rectangles) != 1 {
		t.Fatalf("painted %d rectangles, want 1", len(rectangles))
	}
	if rectangles[0].Bounds.Width != 40 || rectangles[0].Bounds.Height != 40 || rectangles[0].Radius != 7 {
		t.Fatalf("circle = %+v radius %g, want 40x40 radius 7", rectangles[0].Bounds, rectangles[0].Radius)
	}
}

// Reset returns a placeholder to an opaque Block so a reconfigured page does
// not keep a stale circle, radius or opacity.
func TestResetClearsVisualOptions(t *testing.T) {
	var placeholder skeleton.Skeleton
	placeholder.Circle().Radius(9).Opacity(0.25)
	placeholder.Reset().Block().Size(30, 12)
	m := &model{theme: gfx.LightTheme()}
	m.block = placeholder
	m.page = ui.NewPage(ui.PageConfig{Theme: &m.theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, func(c ui.C) {
		skeleton.View(c, "Block", &m.block)
	})
	rectangles := m.canvas.Rectangles()
	if len(rectangles) != 1 {
		t.Fatalf("painted %d rectangles, want 1", len(rectangles))
	}
	if rectangles[0].Radius != m.theme.Metrics.RadiusControl || rectangles[0].Color.Alpha != m.theme.Muted.Alpha {
		t.Fatalf("reset placeholder = %+v, want the default block", rectangles[0])
	}
}

// A warm repaint allocates nothing: the placeholders live in the model.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := new(model)
	open(t, m)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func closeTo(got, want float32) bool { return math.Abs(float64(got-want)) < .001 }

func BenchmarkViewPaint(b *testing.B) {
	m := new(model)
	open(b, m)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
