package hierarchypicker_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/hierarchypicker"
)

type harness struct {
	t     *testing.T
	state *hierarchypicker.State
	pick  *hierarchypicker.Picker
	box   ui.Box
	page  *ui.Page
	drv   *app.Driver
	inv   *app.Invalidator
	opts  gfx.Scope
}

func open(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, state: hierarchypicker.NewState([]string{
		"School", "School/Class A", "School/Class B", "Office",
	})}
	h.pick = hierarchypicker.New(h.state)
	var tree gfx.Tree
	feature := tree.Feature("group")
	h.opts = tree.Sub(feature, tree.Name("options"))
	h.page = ui.NewPage(ui.PageConfig{Tree: &tree, View: func(c ui.C) {
		h.box = h.pick.View(c, hierarchypicker.Spec{
			Label: "Group", Placeholder: "Select a group", OptionIDs: h.opts,
			DismissID: feature.ID(tree.Name("dismiss")), Scope: feature.ID(tree.Name("scope")),
		}).Width(260)
	}})
	h.inv = new(app.Invalidator)
	var err error
	h.drv, err = app.NewDriver(ui.Configure(h.page, app.Config{Width: 480, Height: 480, Invalidator: h.inv}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.drv.Close)
	h.repaint()
	return h
}

func (h *harness) repaint() {
	h.t.Helper()
	h.inv.Invalidate()
	if err := h.drv.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) hit(id gfx.ID) gfx.Hit {
	h.t.Helper()
	for _, hit := range h.drv.Snapshot().Hits {
		if hit.ID == id {
			return hit
		}
	}
	h.t.Fatalf("no hit with identity %v", id)
	return gfx.Hit{}
}

// The picker opens from its field, chooses a path value and closes.
func TestPickerChoosesAPath(t *testing.T) {
	h := open(t)
	r := h.box.Rect()
	if err := h.drv.Click(r.X+r.Width/2, r.Y+r.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.state.Open {
		t.Fatal("the trigger did not open the menu")
	}
	row := h.hit(h.opts.At(0))
	if err := h.drv.Click(row.Bounds.X+row.Bounds.Width/2, row.Bounds.Y+row.Bounds.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if h.state.Value != "School" || h.state.Open {
		t.Fatalf("value %q open %v, want School and closed", h.state.Value, h.state.Open)
	}
}

// Escape closes the menu without reaching the page.
func TestPickerKeyCloses(t *testing.T) {
	h := open(t)
	r := h.box.Rect()
	if err := h.drv.Click(r.X+r.Width/2, r.Y+r.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.state.Open {
		t.Fatal("not open")
	}
	if err := h.drv.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if h.state.Open {
		t.Fatal("Escape left the menu open")
	}
}

func drawPickerPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 480}, view)
	canvas.EndFrame()
}

func BenchmarkHierarchyPickerPaint(b *testing.B) {
	state := hierarchypicker.NewState([]string{"School", "School/Class A", "School/Class B", "Office"})
	state.Open = true
	picker := hierarchypicker.New(state)
	var tree gfx.Tree
	feature := tree.Feature("group")
	opts := tree.Sub(feature, tree.Name("options"))
	view := func(c ui.C) {
		picker.View(c, hierarchypicker.Spec{
			Label: "Group", Placeholder: "Select a group", OptionIDs: opts,
			DismissID: feature.ID(tree.Name("dismiss")), Scope: feature.ID(tree.Name("scope")),
		}).Width(260)
	}
	page := ui.NewPage(ui.PageConfig{Tree: &tree, View: view})
	canvas := &gfx.Canvas{}
	canvas.SetMeasurer(func(Value string, Size, Weight float32) float32 { return float32(len(Value)) * Size / 2 })
	drawPickerPage(page, canvas, view)
	b.ReportAllocs()
	for b.Loop() {
		drawPickerPage(page, canvas, view)
	}
}
