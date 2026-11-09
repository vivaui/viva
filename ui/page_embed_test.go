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
	renames    int
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
	if ui.Shortcut(c, ui.Primary, "r") {
		m.renames++
	}
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
			f.Page(m.page, m.pageView).Grow(1).FillHeight()
		})
	})
}

func openPane(t *testing.T, m *paneModel) *pageDriver {
	t.Helper()
	invalidator := new(app.Invalidator)
	config := app.Config{Width: 640, Height: 480, Tree: &m.tree, Paint: m.paint, Invalidator: invalidator,
		TextInput: m.page.TextInput, OnScroll: m.page.Wheel,
		OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
			if result, ok := m.page.Key(event); ok {
				return result
			}
			return gfx.EventResult{}
		}}
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
	if !m.page.Owns(d.hit("Inner").ID) || m.page.Owns(d.hit("Side").ID) {
		t.Fatal("Owns does not tell the page's controls from the window's")
	}
	if m.page.Overlay() {
		t.Fatal("Overlay with nothing open")
	}
	inner, side := d.hit("Inner").ID, d.hit("Side").ID
	if r, ok := m.page.Pointer(gfx.PointerEvent{Target: inner}); !ok || r.Handled {
		t.Fatal("Pointer does not leave the page's own press to it")
	}
	if _, ok := m.page.Pointer(gfx.PointerEvent{Target: side}); ok {
		t.Fatal("Pointer claims the window's press with no overlay open")
	}
	if _, ok := m.page.Key(gfx.KeyEvent{Key: gfx.KeyDelete}); ok {
		t.Fatal("Key claims a key with no overlay open")
	}
	d.click("More")
	if !m.menu || !m.page.Overlay() {
		t.Fatal("the popover did not open, or Overlay does not report it")
	}
	if r, ok := m.page.Pointer(gfx.PointerEvent{Target: side}); !ok || !r.Handled {
		t.Fatal("Pointer lets a press through to the window behind an open overlay")
	}
	if r, ok := m.page.Key(gfx.KeyEvent{Key: gfx.KeyDelete}); !ok || !r.Handled {
		t.Fatal("Key lets a shortcut through behind an open overlay")
	}
	if r, ok := m.page.Key(gfx.KeyEvent{Key: gfx.KeyEscape}); !ok || r.Handled {
		t.Fatal("Key takes Escape from the overlay")
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

// An open Select list in a pane holds the window behind it as a Popover does: the
// window's shortcuts and presses do not act while it is open, Escape reaches it.
func TestFramePageSelectIsAnOverlay(t *testing.T) {
	m := newPaneModel()
	d := openPane(t, m)
	var trigger gfx.Hit
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit
		}
	}
	if err := d.driver.Click(center(trigger.Bounds)); err != nil {
		t.Fatal(err)
	}
	if !d.has("L") || !m.page.Overlay() {
		t.Fatal("the select did not open, or Overlay does not report it")
	}
	side := d.hit("Side").ID
	if r, ok := m.page.Key(gfx.KeyEvent{Key: gfx.KeyDelete}); !ok || !r.Handled {
		t.Fatal("Key lets a shortcut through behind an open select")
	}
	if r, ok := m.page.Pointer(gfx.PointerEvent{Target: side}); !ok || !r.Handled {
		t.Fatal("Pointer lets a press through to the window behind an open select")
	}
	d.click("L")
	if m.size != "L" || m.page.Overlay() {
		t.Fatalf("size=%q overlay=%v after picking", m.size, m.page.Overlay())
	}
}

// A page is declared by being painted, so a Memo, whose copies are not, cannot hold one.
func TestFramePageInAMemoPanics(t *testing.T) {
	inner := ui.NewPage(ui.PageConfig{Root: "inner", Theme: &pageTheme})
	view := func(c ui.C) { ui.Text(c, "pane") }
	defer func() {
		if recover() == nil {
			t.Fatal("Frame.Page in a Memo did not panic")
		}
	}()
	drawPage(ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas(), func(c ui.C) {
		ui.Memo(c, 1, func() { c.Frame().Page(inner, view).Height(40) })
	})
}

// A pane's Shortcut acts with the focus in the pane, delivered through the window,
// and not with the focus on the window's own controls.
func TestFramePageShortcut(t *testing.T) {
	m := newPaneModel()
	d := openPane(t, m)
	d.click("Side")
	if _, err := d.driver.ShortcutChord(ui.Primary, 'r'); err != nil {
		t.Fatal(err)
	}
	if m.renames != 0 {
		t.Fatal("the pane's Shortcut acted with the focus outside it")
	}
	d.click("Inner")
	if _, err := d.driver.ShortcutChord(ui.Primary, 'r'); err != nil {
		t.Fatal(err)
	}
	if m.renames != 1 {
		t.Fatalf("renames %d with the focus in the pane", m.renames)
	}
}
