package anchor_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/anchor"
)

var items = []anchor.Item{{Title: "Overview"}, {Title: "Install", Level: 0}, {Title: "From source", Level: 1}, {Title: "Usage"}}

type model struct {
	a       *anchor.Anchor
	active  int
	clicked []int
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(16).Children(func() {
		m.a.View(c, "On this page", anchor.Spec{Items: items, Active: m.active})
		if i, ok := m.a.Clicked(); ok {
			m.clicked = append(m.clicked, i)
			m.active = i
		}
	})
}

func TestClickReportsTheLink(t *testing.T) {
	m := &model{a: anchor.New()}
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: 320, Height: 240, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	for _, want := range []int{2, 0} {
		var hit gfx.Hit
		for _, h := range d.Snapshot().Hits {
			if h.Label == items[want].Title && h.Role == gfx.RoleLink {
				hit = h
			}
		}
		if hit.ID.IsZero() {
			t.Fatalf("no link %q", items[want].Title)
		}
		if err := d.Click(hit.Bounds.X+hit.Bounds.Width/2, hit.Bounds.Y+hit.Bounds.Height/2); err != nil {
			t.Fatal(err)
		}
	}
	if len(m.clicked) != 2 || m.clicked[0] != 2 || m.clicked[1] != 0 || m.active != 0 {
		t.Fatalf("clicked %v, active %d", m.clicked, m.active)
	}
}

func warm() func() {
	m := &model{a: anchor.New(), active: 1}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	return func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 240}, m.page)
		canvas.EndFrame()
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	draw := warm()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}

func BenchmarkViewPaint(b *testing.B) {
	draw := warm()
	draw()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		draw()
	}
}
