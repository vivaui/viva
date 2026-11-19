package ui_test

import (
	"testing"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// paintHits is the hits one page view declares, for comparing a page control
// with the component it replaces.
func paintHits(theme gfx.Theme, viewport gfx.Rect, view func(ui.C)) []gfx.Hit {
	canvas := benchCanvas()
	page := ui.NewPage(ui.PageConfig{Theme: &theme, View: view})
	canvas.Reset()
	canvas.BeginFrame()
	page.Draw(canvas, viewport, view)
	canvas.EndFrame()
	return append([]gfx.Hit(nil), canvas.Hits()...)
}

// requireSameControlHit checks that both paths declare the same control hit:
// its identity, bounds, role, label and state.
func requireSameControlHit(t *testing.T, page, component []gfx.Hit, id gfx.ID) {
	t.Helper()
	find := func(hits []gfx.Hit) gfx.Hit {
		for _, hit := range hits {
			if hit.ID == id {
				return hit
			}
		}
		t.Fatalf("no hit %d", id)
		return gfx.Hit{}
	}
	a, b := find(page), find(component)
	if a.Bounds != b.Bounds || a.Role != b.Role || a.Label != b.Label || a.Focusable != b.Focusable || a.Checked != b.Checked {
		t.Fatalf("page hit %+v, component hit %+v", a, b)
	}
}

// A page control declared inside a slot at the same rectangle as the component
// it replaces paints the same picture. An application that keeps a custom slot
// layout can move from components.BeginCheckbox or BeginSwitch to ui.Checkbox
// or ui.Switch without changing a pixel; the hit keeps the same identity and
// bounds, and the page control adds its own click and key binding.
func TestSlotHostedCheckboxPaintsLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	side := components.CheckboxExtent(theme)
	for _, checked := range []bool{false, true} {
		state := checked
		var slot ui.Box
		pageView := func(c ui.C) {
			ui.Column(c).Pad(16).Children(func() {
				slot = ui.Slot(c).Size(22, 24).Shrink(0)
				slot.Children(func() {
					ui.Checkbox(c, &state, "").Absolute().Left(0).Top(0).Size(side, side).Hit(gfx.Item(1), "label")
				})
			})
		}
		page := paintOnPage(theme, pageViewport, pageView)
		rect := slot.Rect()
		componentView := slotView(rect, func(d *ui.DrawContext) {
			components.BeginCheckbox(d.Canvas, *d.Theme, gfx.Item(1), d.Rect, checked).Label("label").Paint()
		})
		requirePaintParity(t, "slot checkbox", page, paintOnPage(theme, pageViewport, componentView))
		requireSameControlHit(t, paintHits(theme, pageViewport, pageView), paintHits(theme, pageViewport, componentView), gfx.Item(1))
	}
}

func TestSlotHostedSwitchPaintsLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	width, height := components.SwitchExtent(theme, false)
	for _, on := range []bool{false, true} {
		state := on
		var slot ui.Box
		pageView := func(c ui.C) {
			ui.Column(c).Pad(16).Children(func() {
				slot = ui.Slot(c).Size(52, 32).Shrink(0)
				slot.Children(func() {
					ui.Switch(c, &state, "").Absolute().Left(10).Top(7).Size(width, height).Hit(gfx.Item(1), "label")
				})
			})
		}
		page := paintOnPage(theme, pageViewport, pageView)
		rect := slot.Rect()
		componentView := slotView(rect, func(d *ui.DrawContext) {
			components.BeginSwitch(d.Canvas, *d.Theme, gfx.Item(1), gfx.Rect{X: d.Rect.X + 10, Y: d.Rect.Y + 7}, on).Label("label").Paint()
		})
		requirePaintParity(t, "slot switch", page, paintOnPage(theme, pageViewport, componentView))
		requireSameControlHit(t, paintHits(theme, pageViewport, pageView), paintHits(theme, pageViewport, componentView), gfx.Item(1))
	}
}

// A page FormField and a painted components.BeginField paint the same title,
// error and reserved rows when the field is given the same rectangle.
func TestSlotHostedFormFieldPaintsLikeTheComponent(t *testing.T) {
	theme := gfx.LightTheme()
	var state ui.FormFieldState
	state.For, state.Error = gfx.Item(1), "Required"
	state.Invalid = true
	var field ui.Box
	pageView := func(c ui.C) {
		ui.Column(c).Pad(16).Children(func() {
			field = ui.FormField(c, &state, "Email").Width(240).Height(90).Children(func() {
				ui.Slot(c).Fill()
			})
		})
	}
	page := paintOnPage(theme, pageViewport, pageView)
	rect := field.Rect()
	componentView := slotView(rect, func(d *ui.DrawContext) {
		components.BeginField(d.Canvas, *d.Theme, d.Rect).For(gfx.Item(1)).Invalid().Paint("Email", "", "Required")
	})
	requirePaintParity(t, "slot form field", page, paintOnPage(theme, pageViewport, componentView))
}
