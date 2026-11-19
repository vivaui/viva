package aspectratioview_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/aspectratioview"
)

type model struct {
	box      aspectratioview.Box
	page     *ui.Page
	canvas   *gfx.Canvas
	maxWidth float32
	title    string
	draws    int
}

func (m *model) viewPage(c ui.C) {
	aspectratioview.View(c, m.title, &m.box, func() {
		m.draws++
		ui.Column(c).Center().Children(func() {
			ui.Text(c, m.title)
		})
	}).FillWidth().MaxWidth(m.maxWidth)
}

func draw(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 400, Height: 300}, view)
	canvas.EndFrame()
}

func open(t testing.TB, m *model) {
	t.Helper()
	theme := gfx.LightTheme()
	m.page = ui.NewPage(ui.PageConfig{View: m.viewPage, Theme: &theme})
	m.canvas = new(gfx.Canvas)
	draw(m.page, m.canvas, m.viewPage)
}

func hit(t *testing.T, canvas *gfx.Canvas, label string) gfx.Hit {
	t.Helper()
	hits := canvas.Hits()
	for index := len(hits) - 1; index >= 0; index-- {
		if hits[index].Label == label && !hits[index].ID.IsZero() {
			return hits[index]
		}
	}
	t.Fatalf("no hit labelled %q was painted", label)
	return gfx.Hit{}
}

func hasText(canvas *gfx.Canvas, value string) bool {
	for _, text := range canvas.Texts() {
		if text.Value == value {
			return true
		}
	}
	return false
}

func closeTo(got, want float32) bool { return math.Abs(float64(got-want)) < .001 }

// The box takes the caller's width and derives its height from the chained
// ratio; the content is the page's own children inside it.
func TestRatioDerivesHeightAndLaysOutContent(t *testing.T) {
	for _, test := range []struct {
		name     string
		width    float32
		height   float32
		ratio    float32
		maxWidth float32
	}{
		{name: "16:9", width: 16, height: 9, ratio: 16.0 / 9.0, maxWidth: 240},
		{name: "1:1", width: 1, height: 1, ratio: 1, maxWidth: 240},
		{name: "4:3", width: 4, height: 3, ratio: 4.0 / 3.0, maxWidth: 300},
	} {
		t.Run(test.name, func(t *testing.T) {
			m := &model{box: *aspectratioview.New(), maxWidth: test.maxWidth, title: test.name + " preview"}
			m.box.Ratio(test.width, test.height)
			open(t, m)

			control := hit(t, m.canvas, m.title)
			if control.Role != gfx.RoleGroup {
				t.Fatalf("role = %v, want group", control.Role)
			}
			bounds := components.AspectRatio(gfx.Rect{Width: test.maxWidth}, test.ratio)
			if !closeTo(control.Bounds.Width, bounds.Width) || !closeTo(control.Bounds.Height, bounds.Height) {
				t.Fatalf("box = %+v, want %+v", control.Bounds, bounds)
			}
			if !hasText(m.canvas, m.title) {
				t.Fatalf("content %q was not painted inside the box", m.title)
			}
			if m.draws != 1 {
				t.Fatalf("content declared %d times, want 1", m.draws)
			}
		})
	}
}

// Square is the same call as Ratio(1, 1); there is no constructor per ratio.
func TestSquare(t *testing.T) {
	m := &model{box: *aspectratioview.New(), maxWidth: 240, title: "Square"}
	m.box.Square()
	open(t, m)
	control := hit(t, m.canvas, m.title)
	if !closeTo(control.Bounds.Width, 240) || !closeTo(control.Bounds.Height, 240) {
		t.Fatalf("square = %+v, want 240x240", control.Bounds)
	}
}

// An invalid ratio gives a zero-height box instead of a non-finite layout.
func TestInvalidRatioIsDeterministic(t *testing.T) {
	for _, ratio := range [][2]float32{{0, 9}, {-16, 9}, {16, 0}} {
		m := &model{box: *aspectratioview.New(), maxWidth: 240, title: "Invalid"}
		m.box.Ratio(ratio[0], ratio[1])
		open(t, m)
		control := hit(t, m.canvas, m.title)
		if control.Bounds.Height > 0 {
			t.Fatalf("ratio %v produced height %g, want 0", ratio, control.Bounds.Height)
		}
	}
}

// The content is laid out again on every paint; the box state is the page's.
func TestContentDeclaredOnEveryPaint(t *testing.T) {
	m := &model{box: *aspectratioview.New(), maxWidth: 240, title: "Repaint"}
	m.box.Ratio(16, 9)
	open(t, m)
	draw(m.page, m.canvas, m.viewPage)
	if m.draws != 2 {
		t.Fatalf("content declared %d times over two paints, want 2", m.draws)
	}
}

// A warm repaint allocates nothing: the layout and box are kept in the model.
func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := &model{box: *aspectratioview.New(), maxWidth: 240, title: "Warm"}
	m.box.Ratio(16, 9)
	open(t, m)
	if allocations := testing.AllocsPerRun(100, func() { draw(m.page, m.canvas, m.viewPage) }); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := &model{box: *aspectratioview.New(), maxWidth: 240, title: "Benchmark"}
	m.box.Ratio(16, 9)
	open(b, m)
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		draw(m.page, m.canvas, m.viewPage)
	}
}
