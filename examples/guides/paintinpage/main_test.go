package main

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

func open(t *testing.T, m *model) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(page.Config(app.Config{Width: 480, Height: 300}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func hit(t *testing.T, driver *app.Driver, name string) gfx.Hit {
	t.Helper()
	for _, h := range driver.Snapshot().Hits {
		if strings.HasPrefix(h.Label, name) && !h.IsLabel {
			return h
		}
	}
	t.Fatalf("no control named %q on screen", name)
	return gfx.Hit{}
}

// The chart is an element of the page: it has the room the page laid out for it, takes
// the pointer and is named for assistive technology, and what is drawn in it follows the
// model.
func TestTheChartIsAnElementOfThePageDrawnFromTheModel(t *testing.T) {
	m := &model{samples: []float32{0.3, 0.6}}
	driver := open(t, m)
	chart := hit(t, driver, "Samples chart")
	if chart.Bounds.Width < 300 || chart.Bounds.Height != 120 {
		t.Fatalf("the chart's room is %vx%v, want the width of the page and 120 high", chart.Bounds.Width, chart.Bounds.Height)
	}
	bars := func() (n int) {
		// What the chart painted: filled shapes in the primary color inside its room.
		for _, p := range driver.Snapshot().Primitives {
			if p.Color == gfx.LightTheme().Primary && p.Bounds.X >= chart.Bounds.X && p.Bounds.Y >= chart.Bounds.Y &&
				p.Bounds.X+p.Bounds.Width <= chart.Bounds.X+chart.Bounds.Width &&
				p.Bounds.Y+p.Bounds.Height <= chart.Bounds.Y+chart.Bounds.Height {
				n++
			}
		}
		return n
	}
	before := bars()
	add := hit(t, driver, "Add a sample")
	if err := driver.Click(add.Bounds.X+add.Bounds.Width/2, add.Bounds.Y+add.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	if len(m.samples) != 3 {
		t.Fatalf("samples = %d, want 3", len(m.samples))
	}
	if after := bars(); before == 0 || after <= before {
		t.Fatalf("a new sample painted no new bar in the chart: %d before, %d after", before, after)
	}
}
