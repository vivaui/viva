package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// The Bases without a test of their own are still controls without a look:
// they paint no surface, and declare the role and state assistive technology
// needs. ButtonBase, the item Bases, InputBase and TextareaBase have their own
// tests; this table keeps the rest from drifting.
func TestEveryRemainingBaseIsItsControlWithoutALook(t *testing.T) {
	theme := gfx.LightTheme()
	scope := tree.Sub(page, tree.Name("base-contract"))
	type base struct {
		name  string
		role  gfx.Role
		build func(f *Frame, id gfx.ID) Box
		state func(gfx.Hit) bool
	}
	bases := []base{
		{"checkbox", gfx.RoleCheckbox, func(f *Frame, id gfx.ID) Box { return f.CheckboxBase(id, true) },
			func(h gfx.Hit) bool { return h.Checked }},
		{"switch", gfx.RoleSwitch, func(f *Frame, id gfx.ID) Box { return f.SwitchBase(id, true) },
			func(h gfx.Hit) bool { return h.Checked }},
		{"slider", gfx.RoleSlider, func(f *Frame, id gfx.ID) Box {
			return f.SliderBase(id, &SliderState{}).Size(120, 16)
		}, func(h gfx.Hit) bool { return h.Range.Set }},
		{"select", gfx.RoleCombobox, func(f *Frame, id gfx.ID) Box {
			return f.SelectBase(scope, &SelectState{}, []string{"Small", "Large"}, 0).Size(120, 32)
		}, func(h gfx.Hit) bool { return h.Value == "Small" && !h.Expanded }},
	}
	for _, b := range bases {
		t.Run(b.name, func(t *testing.T) {
			var view View
			canvas := testCanvas()
			view.Draw(canvas, theme, gfx.Rect{Width: 240, Height: 100}, func(f *Frame) {
				b.build(f, tree.ID(page, "base-contract-"+b.name))
			})
			if len(canvas.Rectangles()) != 0 || len(canvas.Shadows()) != 0 {
				t.Errorf("a Base painted a look: %d rectangles, %d shadows", len(canvas.Rectangles()), len(canvas.Shadows()))
			}
			hits := canvas.Hits()
			if len(hits) != 1 {
				t.Fatalf("hits = %+v, want one", hits)
			}
			if hit := hits[0]; hit.Role != b.role || !hit.Focusable || !b.state(hit) {
				t.Errorf("hit = %+v, want a focusable %v in its state", hit, b.role)
			}
		})
	}
}
