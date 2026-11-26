package overflowlist_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/overflowlist"
)

var glyph = gfx.Icon{ViewBox: 24, Stroke: 2, Commands: []gfx.PathCommand{gfx.MoveTo(4, 12), gfx.LineTo(20, 12)}}

type model struct {
	o       *overflowlist.OverflowList
	items   []overflowlist.Item
	min     int
	clicked []int
}

func newModel(n int) *model {
	m := &model{o: overflowlist.New()}
	for i := 0; i < n; i++ {
		m.items = append(m.items, overflowlist.Item{Label: fmt.Sprintf("Tool %d", i), Icon: glyph})
	}
	return m
}

func (m *model) page(c ui.C) {
	ui.Column(c).Fill().Pad(8).Children(func() {
		m.o.View(c, "Actions", overflowlist.Spec{Items: m.items, Min: m.min}).FillWidth()
		if i, ok := m.o.Clicked(); ok {
			m.clicked = append(m.clicked, i)
		}
	})
}

type driver struct {
	t *testing.T
	*app.Driver
}

func open(t *testing.T, m *model, width int) driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	d, err := app.NewDriver(ui.Configure(page, app.Config{Width: width, Height: 200, Invalidator: new(app.Invalidator)}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.Close)
	return driver{t, d}
}

// row is the tool buttons in the row, left to right, and whether More is there.
func (d driver) row() (tools []string, more bool) {
	for _, h := range d.Snapshot().Hits {
		if h.IsLabel || h.Role != gfx.RoleButton {
			continue
		}
		if h.Label == "More" {
			more = true
		} else {
			tools = append(tools, h.Label)
		}
	}
	return tools, more
}

func (d driver) click(label string) {
	d.t.Helper()
	hits := d.Snapshot().Hits
	for i := len(hits) - 1; i >= 0; i-- {
		if h := hits[i]; h.Label == label && !h.IsLabel && !h.ID.IsZero() {
			if err := d.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
				d.t.Fatal(err)
			}
			return
		}
	}
	d.t.Fatalf("no control labeled %q", label)
}

func TestEverythingFitsThenThereIsNoMore(t *testing.T) {
	m := newModel(4)
	d := open(t, m, 600)
	tools, more := d.row()
	if len(tools) != 4 || more {
		t.Fatalf("tools %v more %v", tools, more)
	}
}

func TestNarrowingMovesTheLastItemsIntoMoreAndWideningBringsThemBack(t *testing.T) {
	m := newModel(6)
	d := open(t, m, 600)
	previous := 6
	for _, width := range []int{240, 180, 120, 80} {
		if err := d.Resize(width, 200); err != nil {
			t.Fatal(err)
		}
		if err := d.Pump(); err != nil {
			t.Fatal(err)
		}
		tools, more := d.row()
		if len(tools) >= 6 || !more {
			t.Fatalf("width %d: %d tools, more %v; want fewer than all and a menu", width, len(tools), more)
		}
		if len(tools) > previous {
			t.Fatalf("width %d: %d tools, more than at the wider width (%d)", width, len(tools), previous)
		}
		for i, label := range tools {
			if want := fmt.Sprintf("Tool %d", i); label != want {
				t.Fatalf("width %d: row has %v; items must leave from the end", width, tools)
			}
		}
		previous = len(tools)
	}
	if err := d.Resize(600, 200); err != nil {
		t.Fatal(err)
	}
	if err := d.Pump(); err != nil {
		t.Fatal(err)
	}
	if tools, more := d.row(); len(tools) != 6 || more {
		t.Fatalf("widened: %v more %v", tools, more)
	}
}

func TestNeverFewerThanTheMinimum(t *testing.T) {
	m := newModel(6)
	m.min = 3
	d := open(t, m, 40)
	if tools, _ := d.row(); len(tools) < 3 {
		t.Fatalf("only %d tools in a row that keeps at least 3", len(tools))
	}
}

func TestAnOverflowedItemIsChosenFromTheMenuAndKeepsItsDisabledState(t *testing.T) {
	m := newModel(6)
	m.items[5].Disabled = true
	d := open(t, m, 160)
	tools, more := d.row()
	if !more || len(tools) >= 6 {
		t.Fatalf("not overflowed: %v %v", tools, more)
	}
	d.click("More")
	var hidden, disabled bool
	for _, h := range d.Snapshot().Hits {
		if h.Label == "Tool 5" && !h.IsLabel {
			hidden, disabled = true, h.Disabled
		}
	}
	if !hidden || !disabled {
		t.Fatalf("the last item in the menu: present %v disabled %v", hidden, disabled)
	}
	d.click("Tool 4")
	if len(m.clicked) != 1 || m.clicked[0] != 4 {
		t.Fatalf("clicked %v, want [4]", m.clicked)
	}
}

func TestClickingARowButtonReportsIt(t *testing.T) {
	m := newModel(3)
	d := open(t, m, 600)
	d.click("Tool 1")
	if len(m.clicked) != 1 || m.clicked[0] != 1 {
		t.Fatalf("clicked %v", m.clicked)
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	m := newModel(6)
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: m.page, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 180, Height: 200}, m.page)
		canvas.EndFrame()
	}
	draw()
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}
