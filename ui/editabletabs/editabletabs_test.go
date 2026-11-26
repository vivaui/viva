package editabletabs_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/editabletabs"
)

type model struct {
	strip  *editabletabs.EditableTabs
	tabs   []editabletabs.Tab
	active int
	closed []int
	added  int
}

func newModel(titles ...string) *model {
	m := &model{strip: editabletabs.New()}
	for _, title := range titles {
		m.tabs = append(m.tabs, editabletabs.Tab{Title: title})
	}
	return m
}

func TestWarmPaintAllocatesNothing(t *testing.T) {
	m := newModel("alpha", "beta", "gamma")
	m.tabs[0].Pinned = true
	view := func(c ui.C) {
		m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 600, Height: 160}, view)
		canvas.EndFrame()
	}
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("warm paint allocated %v times, want 0", allocations)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	m := newModel("alpha", "beta", "gamma", "delta")
	view := func(c ui.C) {
		m.strip.View(c, "Sessions", &m.active, editabletabs.Spec{Tabs: m.tabs, Addable: true}).FillWidth()
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	b.ReportAllocs()
	for range b.N {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 600, Height: 160}, view)
		canvas.EndFrame()
	}
}
