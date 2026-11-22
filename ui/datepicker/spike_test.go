package datepicker_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// spikeModel is the smallest thing shaped like a picker: one Control whose
// trigger is the control itself, with two panel cells declared as parts and a
// backdrop/dismiss hit while it is open. It exists to settle, before the real
// component is written, whether a canvas-declared panel can take the pointer,
// keep Tab inside the control's parts and close on Escape.
type spikeModel struct {
	handlers ui.Handlers
	drawFn   func(*ui.DrawContext)
	open     bool
	chosen   int
	dismiss  gfx.ID
}

func (m *spikeModel) key(event gfx.KeyEvent) gfx.EventResult {
	parts := m.handlers.Parts()
	if index, ok := event.Target.In(parts); ok {
		if event.Key == gfx.KeyEnter || event.Key == gfx.KeySpace {
			m.chosen = index
			m.open = false
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		return gfx.EventResult{}
	}
	switch event.Key {
	case gfx.KeyEnter, gfx.KeySpace, gfx.KeyDown:
		m.open = true
		return gfx.EventResult{Handled: true, Repaint: true, Focus: parts.At(1), FocusVisible: true}
	}
	return gfx.EventResult{}
}

func (m *spikeModel) pointer(event gfx.PointerEvent) gfx.EventResult {
	if event.Kind != gfx.PointerDown {
		return gfx.EventResult{}
	}
	if index, ok := event.Target.In(m.handlers.Parts()); ok {
		if index == 0 {
			m.open = !m.open
			return gfx.EventResult{Handled: true, Repaint: true}
		}
		m.chosen = index
		m.open = false
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	return gfx.EventResult{}
}

func (m *spikeModel) draw(d *ui.DrawContext) {
	canvas := d.Canvas
	parts := m.handlers.Parts()
	if m.open && canvas.Clicked(m.dismiss) {
		m.open = false
	}
	canvas.Fill(d.Rect, d.Theme.Input)
	canvas.HitWithOptions(parts.At(0), d.Rect, "Date", gfx.HitOptions{Role: gfx.RoleCombobox, Expanded: m.open})
	if !m.open {
		return
	}
	panel := gfx.Rect{X: d.Rect.X, Y: d.Rect.Y + 40, Width: 220, Height: 90}
	scope := parts.At(0)
	canvas.PushLayer(d.Frame.WidgetLayer() + 2)
	canvas.PushFocusScope(scope)
	canvas.PushOverlayClip(d.Frame.GetBounds())
	canvas.HitWithOptions(m.dismiss, d.Frame.GetBounds(), "Dismiss spike", gfx.HitOptions{
		Role: gfx.RoleButton, Backdrop: true, PassSecondary: true, SkipFocus: true,
	})
	canvas.Block(panel)
	canvas.Fill(panel, d.Theme.Popover)
	for index := 1; index <= 2; index++ {
		cell := gfx.Rect{X: panel.X + 8 + float32(index-1)*80, Y: panel.Y + 8, Width: 72, Height: 28}
		label := "cell one"
		if index == 2 {
			label = "cell two"
		}
		canvas.HitWithOptions(parts.At(index), cell, label, gfx.HitOptions{Role: gfx.RoleButton})
	}
	canvas.TrapFocus(scope)
	canvas.SetInitialFocus(parts.At(1))
	canvas.SetEscapeTarget(m.dismiss)
	canvas.PopOverlayClip()
	canvas.PopFocusScope()
	canvas.PopLayer()
}

type spikeHarness struct {
	t      *testing.T
	model  *spikeModel
	page   *ui.Page
	driver *app.Driver
	inv    *app.Invalidator
	tree   gfx.Tree
}

func openSpike(t *testing.T) *spikeHarness {
	t.Helper()
	h := &spikeHarness{t: t}
	h.model = &spikeModel{}
	feature := h.tree.Feature("spike")
	h.model.dismiss = feature.ID(h.tree.Name("dismiss"))
	h.model.handlers.Role = gfx.RoleCombobox
	h.model.handlers.Key = h.model.key
	h.model.handlers.Pointer = h.model.pointer
	h.model.drawFn = h.model.draw
	h.page = ui.NewPage(ui.PageConfig{Tree: &h.tree, View: func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Control(c, "Date", &h.model.handlers).Size(200, 32).Draw(h.model.drawFn)
			ui.Button(c, "after")
		})
	}})
	h.inv = new(app.Invalidator)
	driver, err := app.NewDriver(ui.Configure(h.page, app.Config{Width: 480, Height: 480, Invalidator: h.inv}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	h.driver = driver
	h.repaint()
	return h
}

func (h *spikeHarness) repaint() {
	h.t.Helper()
	h.inv.Invalidate()
	if err := h.driver.Pump(); err != nil {
		h.t.Fatal(err)
	}
}

func (h *spikeHarness) focused() string {
	snapshot := h.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == snapshot.Focused {
			return hit.Label
		}
	}
	return ""
}

func (h *spikeHarness) key(key gfx.Key) {
	h.t.Helper()
	if err := h.driver.Key(key, 0); err != nil {
		h.t.Fatal(err)
	}
}

func (h *spikeHarness) hit(label string) gfx.Hit {
	h.t.Helper()
	for _, hit := range h.driver.Snapshot().Hits {
		if hit.Label == label && !hit.Backdrop && !hit.ID.IsZero() {
			return hit
		}
	}
	h.t.Fatalf("no hit labeled %q", label)
	return gfx.Hit{}
}

// These tests settle the mechanics the picker's panel depends on, before the
// component is written:
//
//   - a canvas-declared panel is reachable outside the Control's own rectangle,
//     because its parts are ordinary hits under a pushed layer;
//   - TrapFocus works for those parts only when the same identity given to
//     TrapFocus is pushed as the focus scope the panel's hits are declared in
//     (a bare part identity is a usable scope, as the color picker does);
//   - SetInitialFocus moves the keyboard into the panel when the trap opens, and
//     closing the trap hands it back to the trigger without extra code;
//   - Escape arrives as a click on the dismiss target, read in Draw, not as a
//     key the control handles.
//
// They stay as a regression test for that wiring when the real component lands.
func TestSpikeCanvasPartsTakePointerAndEscape(t *testing.T) {
	h := openSpike(t)
	trigger := h.hit("Date").Bounds
	if err := h.driver.Click(trigger.X+10, trigger.Y+10); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.model.open {
		t.Fatal("a press on the trigger did not open the panel")
	}
	cell := h.hit("cell two").Bounds
	if err := h.driver.Click(cell.X+4, cell.Y+4); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if h.model.open || h.model.chosen != 2 {
		t.Fatalf("a press outside the control's rect left open=%v chosen=%d", h.model.open, h.model.chosen)
	}

	// Opening again focuses the panel's initial cell; Escape reaches the
	// dismiss target instead of the page, and gives the keyboard back.
	if err := h.driver.Click(trigger.X+10, trigger.Y+10); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	if !h.model.open {
		t.Fatal("a second press on the trigger did not reopen the panel")
	}
	if focus := h.focused(); focus != "cell one" {
		t.Fatalf("opening focused %q, want the panel's initial cell", focus)
	}
	h.key(gfx.KeyEscape)
	h.repaint()
	if h.model.open {
		t.Fatal("Escape did not close the panel")
	}
	if focus := h.focused(); focus != "Date" {
		t.Fatalf("closing left the focus on %q, want the trigger", focus)
	}
}

// Tab stays inside the open panel's parts, and never reaches the button after it.
func TestSpikeTabStaysInTheControlWhileOpen(t *testing.T) {
	h := openSpike(t)
	trigger := h.hit("Date").Bounds
	if err := h.driver.Click(trigger.X+10, trigger.Y+10); err != nil {
		t.Fatal(err)
	}
	h.repaint()
	seen := map[string]bool{}
	for range 6 {
		h.key(gfx.KeyTab)
		focus := h.focused()
		seen[focus] = true
		if focus == "after" {
			t.Fatalf("Tab left the open panel for %q", focus)
		}
	}
	for _, want := range []string{"cell one", "cell two"} {
		if !seen[want] {
			t.Fatalf("Tab never reached %q; seen %v", want, seen)
		}
	}
}
