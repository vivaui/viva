package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// paneModel is a window laid out with a Frame (a sidebar of its own controls) whose
// pane is a page, as an application moving one pane at a time to the page API has.
type paneModel struct {
	tree     gfx.Tree
	sidebar  gfx.Scope
	view     ui.View
	page     *ui.Page
	pageView func(ui.C)

	sideClicks int
	clicks     int
	on         bool
	name       ui.Field
	menu       bool
	size       string
	rows       []string
}

func newPaneModel() *paneModel {
	m := &paneModel{rows: []string{"a", "b", "c"}}
	m.sidebar = m.tree.Feature("sidebar")
	m.page = ui.NewPage(ui.PageConfig{Root: "pane", Theme: &pageTheme}) // no Tree: it takes the canvas's
	m.pageView = m.pane
	return m
}

func (m *paneModel) pane(c ui.C) {
	ui.Column(c).Pad(8).Gap(8).Children(func() {
		if ui.Button(c, "Inner").Clicked() {
			m.clicks++
		}
		ui.Switch(c, &m.on, "Toggle")
		ui.TextField(c, &m.name, "Name", "")
		more := ui.Button(c, "More")
		if more.Clicked() {
			m.menu = !m.menu
		}
		ui.Popover(c, more, &m.menu, func() { ui.Text(c, "Menu item") })
		ui.Select(c, &m.size, []string{"S", "M", "L"}, nil).Placeholder("Size").Width(120)
		for _, row := range m.rows {
			ui.Row(c.Key(row)).Gap(8).Children(func() {
				ui.Text(c, row)
				if ui.Button(c, "Delete").Clicked() {
					c.After(func() { m.remove(row) })
				}
			})
		}
	})
}

func (m *paneModel) remove(row string) {
	for i, r := range m.rows {
		if r == row {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return
		}
	}
}

// pageSlot paints page into the slot the Frame laid out for it, as a pane of a
// Frame-laid-out window is written as a page. The page is drawn on the host
// canvas so its controls keep the host's identities.
func pageSlot(f *ui.Frame, page *ui.Page, view func(ui.C)) ui.Box {
	return f.Slot().Draw(func(d *ui.DrawContext) {
		page.Draw(d.Canvas, d.Rect, view)
	})
}

func (m *paneModel) paint(canvas *gfx.Canvas, size gfx.Size) {
	m.view.Draw(canvas, pageTheme, gfx.Rect{Width: size.Width, Height: size.Height}, func(f *ui.Frame) {
		f.Row().Fill().Children(func() {
			// The sidebar is as tall as the window, under where an unplaced page would go.
			f.Column().Width(200).FillHeight().Children(func() {
				for i := range 12 {
					if f.Button(m.sidebar.ID(gfx.Index(i)), "Side").FillWidth().Clicked() {
						m.sideClicks++
					}
				}
			})
			pageSlot(f, m.page, m.pageView).Grow(1).FillHeight()
		})
	})
}

func openPane(t *testing.T, m *paneModel) *pageDriver {
	t.Helper()
	invalidator := new(app.Invalidator)
	config := app.Config{Width: 640, Height: 480, Tree: &m.tree, Paint: m.paint, Invalidator: invalidator,
		TextInput: m.page.TextInput, OnScroll: m.page.Wheel}
	driver, err := app.NewDriver(config, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return &pageDriver{t: t, driver: driver, page: m.page, invalidator: invalidator}
}

// A page in a pane of a Frame takes its clicks, keys and typing as a window's page
// does, and the window's own controls beside it keep theirs.
func TestFramePageIsAPaneThatTakesInput(t *testing.T) {
	m := newPaneModel()
	d := openPane(t, m)
	if hit := d.hit("Inner"); hit.Bounds.X < 200 {
		t.Fatalf("the page was painted at %v, not in its pane", hit.Bounds)
	}
	d.click("Inner")
	d.click("Toggle")
	if m.clicks != 1 || !m.on || m.sideClicks != 0 {
		t.Fatalf("clicks=%d on=%v side=%d", m.clicks, m.on, m.sideClicks)
	}
	d.click("Name")
	d.typeText("go")
	if m.name.Text() != "go" {
		t.Fatalf("typing reached %q", m.name.Text())
	}
	d.click("More")
	if !m.menu || !m.page.Overlay() {
		t.Fatal("the popover did not open")
	}
	d.key(gfx.KeyEscape)
	if m.menu || m.page.Overlay() {
		t.Fatal("Escape did not close the popover")
	}
	// The second row's Delete removes that row.
	var deletes []gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Label == "Delete" && !hit.IsLabel {
			deletes = append(deletes, hit)
		}
	}
	if len(deletes) != 3 {
		t.Fatalf("%d Delete buttons", len(deletes))
	}
	if err := d.driver.Click(center(deletes[1].Bounds)); err != nil {
		t.Fatal(err)
	}
	if len(m.rows) != 2 || m.rows[0] != "a" || m.rows[1] != "c" {
		t.Fatalf("rows = %v", m.rows)
	}
	d.click("Side")
	if m.sideClicks != 1 {
		t.Fatal("the window's own control lost its click")
	}
}

func BenchmarkFramePage(b *testing.B) {
	m := newPaneModel()
	canvas := benchCanvas()
	canvas.SetTree(&m.tree)
	b.ReportAllocs()
	for b.Loop() {
		canvas.Reset()
		canvas.BeginFrame()
		m.paint(canvas, gfx.Size{Width: 640, Height: 480})
		canvas.EndFrame()
	}
}
