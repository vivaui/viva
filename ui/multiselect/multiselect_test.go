package multiselect_test

import (
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/base"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/multiselect"
	"github.com/vivaui/viva/window"
)

func TestEllipsizedChipKeepsItsFullLabelAndRemoveButton(t *testing.T) {
	h := open(t)
	const full = "Development-with-a-complete-long-name"
	h.state.SetOptions([]base.MultiSelectOption{{Value: "dev", Label: full}})
	h.state.SetValues([]string{"dev"})
	h.repaint()
	var text gfx.Rect
	for _, label := range h.drv.Snapshot().Labels {
		if strings.HasSuffix(label.Value, "…") {
			text = label.Bounds
			break
		}
	}
	if text.Width == 0 {
		t.Fatal("the selected chip did not ellipsize")
	}
	if err := h.drv.Move(text.X+2, text.Y+text.Height/2); err != nil {
		t.Fatal(err)
	}
	var tip string
	for _, label := range h.drv.Snapshot().Labels {
		if label.Layer == ui.TooltipLayer {
			tip += label.Value
		}
	}
	if tip != full {
		t.Fatalf("chip tooltip %q, want %q", tip, full)
	}
	if err := h.drv.Click(center(h.hit(h.rem.At(0)).Bounds)); err != nil {
		t.Fatal(err)
	}
	if len(h.state.Values()) != 0 {
		t.Fatal("the text hint intercepted the remove button")
	}
}

type harness struct {
	t     *testing.T
	state *base.MultiSelectState
	pick  *multiselect.Picker
	box   ui.Box
	page  *ui.Page
	drv   *app.Driver
	inv   *app.Invalidator
	opts  gfx.Scope
	rem   gfx.Scope
}

func open(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, state: &base.MultiSelectState{AllowCreate: true}}
	h.state.SetOptions([]base.MultiSelectOption{{Value: "dev", Label: "Development"}, {Value: "ops", Label: "Operations"}})
	h.pick = multiselect.New(h.state)
	var tree gfx.Tree
	feature := tree.Feature("tags")
	h.opts = tree.Sub(feature, tree.Name("options"))
	h.rem = tree.Sub(feature, tree.Name("removes"))
	h.page = ui.NewPage(ui.PageConfig{Tree: &tree, View: func(c ui.C) {
		h.box = h.pick.View(c, multiselect.Spec{
			Label: "Tags", Placeholder: "Add a tag", Options: h.opts, Removes: h.rem,
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

func center(r gfx.Rect) (float32, float32) { return r.X + r.Width/2, r.Y + r.Height/2 }

// The opt-in picker is a page control: its trigger opens the popup, a choice
// becomes a chip, and the chip's remove button takes it back.
func TestPickerChoosesAndRemovesValues(t *testing.T) {
	h := open(t)
	r := h.box.Rect()
	if err := h.drv.Click(r.X+r.Width-10, r.Y+r.Height/2); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.state.Open {
		t.Fatal("the trigger did not open the popup")
	}
	choice := h.hit(h.opts.At(0))
	if err := h.drv.Click(center(choice.Bounds)); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if got := h.state.Values(); len(got) != 1 || got[0] != "dev" {
		t.Fatalf("values = %v, want [dev]", got)
	}
	remove := h.hit(h.rem.At(0))
	if err := h.drv.Click(center(remove.Bounds)); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if got := h.state.Values(); len(got) != 0 {
		t.Fatalf("values = %v after remove, want none", got)
	}
}

// The query editor is the platform's: the control hands it the state's editor.
func TestPickerHandsTheQueryToTheTextService(t *testing.T) {
	h := open(t)
	id := h.box.N().ID
	input := h.page.TextInput(window.TextInputContext{ID: id})
	if input == nil || input.Editor != &h.state.Query {
		t.Fatal("the picker did not hand its query editor to the text service")
	}
}

// Escape keeps the popup's navigation on the control and away from the page.
func TestPickerKeyClosesThePopup(t *testing.T) {
	h := open(t)
	r := h.box.Rect()
	if err := h.drv.Click(r.X+20, r.Y+r.Height/2); err != nil {
		t.Fatal(err)
	}
	h.state.Open = true
	h.repaint()
	if err := h.drv.Key(gfx.KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if h.state.Open {
		t.Fatal("Escape did not close the popup")
	}
}

func drawPickerPage(page *ui.Page, canvas *gfx.Canvas, view func(ui.C)) {
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, gfx.Rect{Width: 480, Height: 480}, view)
	canvas.EndFrame()
}

func BenchmarkPickerPaint(b *testing.B) {
	state := &base.MultiSelectState{Open: true}
	state.SetOptions([]base.MultiSelectOption{{Value: "dev", Label: "Development"}, {Value: "ops", Label: "Operations"}})
	state.SetValues([]string{"dev"})
	picker := multiselect.New(state)
	var tree gfx.Tree
	feature := tree.Feature("tags")
	opts := tree.Sub(feature, tree.Name("options"))
	rem := tree.Sub(feature, tree.Name("removes"))
	view := func(c ui.C) {
		picker.View(c, multiselect.Spec{Label: "Tags", Placeholder: "Add a tag", Options: opts, Removes: rem}).Width(260)
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

// Wheel scrolls the open popup, for a page whose wheel handler reaches it
// before the page itself.
func TestPickerWheelScrollsTheOpenPopup(t *testing.T) {
	h := open(t)
	h.state.SetOptions([]base.MultiSelectOption{
		{Value: "a", Label: "Alpha"}, {Value: "b", Label: "Beta"}, {Value: "c", Label: "Gamma"},
		{Value: "d", Label: "Delta"}, {Value: "e", Label: "Epsilon"}, {Value: "f", Label: "Zeta"},
		{Value: "g", Label: "Eta"}, {Value: "h", Label: "Theta"}, {Value: "i", Label: "Iota"},
		{Value: "j", Label: "Kappa"},
	})
	h.state.Open = true
	h.repaint()
	before := h.state.ScrollArea().Offset
	if !h.pick.Wheel(gfx.ScrollEvent{DeltaY: 24}) {
		t.Fatal("the wheel did not reach the open popup")
	}
	if h.state.ScrollArea().Offset <= before {
		t.Fatalf("popup offset %v after the wheel, was %v", h.state.ScrollArea().Offset, before)
	}
	if h.pick.Wheel(gfx.ScrollEvent{DeltaY: 24}) {
		// At the end the popup reports no movement; the page may then scroll.
		if h.state.ScrollArea().Offset < before {
			t.Fatal("the popup scrolled backwards")
		}
	}
}
