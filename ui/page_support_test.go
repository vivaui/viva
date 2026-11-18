package ui_test

import (
	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"testing"
)

// pageDriver is a headless window that paints a Page.
type pageDriver struct {
	t           *testing.T
	driver      *app.Driver
	page        *ui.Page
	invalidator *app.Invalidator
}

var pageTheme = gfx.LightTheme()

func openPage(t *testing.T, view func(ui.C)) *pageDriver {
	t.Helper()
	return openPageWith(t, ui.PageConfig{View: view}, app.Config{})
}

func openPageWith(t *testing.T, config ui.PageConfig, Base app.Config) *pageDriver {
	t.Helper()
	page := ui.NewPage(config)
	invalidator := new(app.Invalidator)
	Base.Invalidator = invalidator
	if Base.Width == 0 {
		Base.Width, Base.Height = 480, 480
	}
	driver, err := app.NewDriver(page.Config(Base), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return &pageDriver{t: t, driver: driver, page: page, invalidator: invalidator}
}

// repaint paints again after the model changed outside an event, as background
// work published through an Invalidator does.
func (d *pageDriver) repaint() {
	d.t.Helper()
	d.invalidator.Invalidate()
	if err := d.driver.Pump(); err != nil {
		d.t.Fatal(err)
	}
}

// hit finds the control named label (its accessible name), the front-most first.
func (d *pageDriver) hit(Label string) gfx.Hit {
	d.t.Helper()
	hits := d.driver.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if hit := hits[i]; hit.Label == Label && !hit.IsLabel && !hit.Backdrop && !hit.ID.IsZero() {
			return hit
		}
	}
	d.t.Fatalf("no control labeled %q", Label)
	return gfx.Hit{}
}

func (d *pageDriver) has(Label string) bool {
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == Label && !hit.IsLabel && !hit.Backdrop {
			return true
		}
	}
	return false
}

func center(r gfx.Rect) (float32, float32) { return r.X + r.Width/2, r.Y + r.Height/2 }

func (d *pageDriver) click(Label string) {
	d.t.Helper()
	if err := d.driver.Click(center(d.hit(Label).Bounds)); err != nil {
		d.t.Fatal(err)
	}
}

func (d *pageDriver) key(key gfx.Key) {
	d.t.Helper()
	if err := d.driver.Key(key, 0); err != nil {
		d.t.Fatal(err)
	}
}

func (d *pageDriver) typeText(Value string) {
	d.t.Helper()
	if err := d.driver.Type(Value); err != nil {
		d.t.Fatal(err)
	}
}

func (d *pageDriver) Focused() string {
	snapshot := d.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused && !hit.IsLabel && !hit.Backdrop {
			return hit.Label
		}
	}
	return ""
}

func benchCanvas() *gfx.Canvas {
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	return canvas
}

func drawPage(page *ui.Page, Surface *gfx.Canvas, view func(ui.C)) {
	Surface.Reset()
	Surface.BeginFrame()
	page.Draw(Surface, gfx.Rect{Width: 600, Height: 400}, view)
	Surface.EndFrame()
}

type fleetHost struct {
	Name, addr string
	handled    bool
}

type fleetModel struct {
	hosts   []fleetHost
	filter  int
	scans   int
	labels  []string
	summary string
}

func newFleetModel() *fleetModel {
	m := &fleetModel{labels: []string{"All", "Open", "Handled"}, summary: "8 hosts"}
	for i := range 8 {
		m.hosts = append(m.hosts, fleetHost{Name: "host-" + string(rune('a'+i)), addr: "10.0.0.1", handled: i%2 == 0})
	}
	return m
}

// The fleet page written with a Frame and identities of its own, the baseline the
// Page version is measured against.
var (
	benchTree   = gfx.Tree{}
	benchPage   = benchTree.Feature("fleet")
	benchFilter = benchTree.Sub(benchPage, benchTree.Name("filter"))
	benchList   = benchTree.ID(benchPage, "list")
	benchScan   = benchTree.ID(benchPage, "scan")
	benchRows   = benchTree.Sub(benchPage, benchTree.Name("rows"))
)

func (m *fleetModel) frame(f *ui.Frame) {
	f.Column().Fill().Pad(22).Gap(14).Children(func() {
		f.Text("Fleet check").FontSize(26).Bold()
		if i, ok := f.Choice(benchFilter, m.labels, m.filter).ChosenIndex(); ok {
			m.filter = i
		}
		f.ScrollArea(benchList).Grow(1).Gap(8).Children(func() {
			for i := range m.hosts {
				h := &m.hosts[i]
				f.Row().FillWidth().Pad(12).Gap(12).Children(func() {
					f.Text(h.Name).Bold()
					f.Text(h.addr).Muted()
					if f.Checkbox(benchRows.At(i), h.handled).Clicked() {
						h.handled = !h.handled
					}
				})
			}
		})
		f.Row().Gap(10).Children(func() {
			f.Text(m.summary).Muted()
			if f.Button(benchScan, "Scan again").Clicked() {
				m.scans++
			}
		})
	})
}

func BenchmarkFrameFleet(b *testing.B) {
	canvas := benchCanvas()
	canvas.SetTree(&benchTree)
	m := newFleetModel()
	var view ui.View
	theme := gfx.LightTheme()
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.BeginFrame()
		view.Draw(canvas, theme, gfx.Rect{Width: 600, Height: 400}, m.frame)
		canvas.EndFrame()
	}
}
