package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

// The calls a page makes on a Toggle reach its surface and label: FontSize,
// Weight and Color the text, Height and Radius the surface.
func TestTogglePagePropertiesReachTheSurface(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	on := true
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Toggle(c, &on, "Bold").FontSize(18).Weight(gfx.WeightBold).Color(ink).Height(40).Radius(5)
		})
	})
	snapshot := d.driver.Snapshot()
	var target gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Label == "Bold" {
			target = hit
		}
	}
	if target.Label != "Bold" {
		t.Fatalf("the toggle declared no hit: %+v", snapshot.Hits)
	}
	if target.Bounds.Height != 40 {
		t.Errorf("toggle height = %v, want 40", target.Bounds.Height)
	}
	if !target.Checked || !target.Selected {
		t.Errorf("toggle hit = %+v, want checked and selected", target)
	}
	var label gfx.Text
	for _, painted := range snapshot.Texts {
		if painted.Value == "Bold" {
			label = painted
		}
	}
	if label.Value != "Bold" {
		t.Fatalf("the toggle label was not painted: %+v", snapshot.Texts)
	}
	if label.Style.Size != 18 || label.Style.Weight != gfx.WeightBold || label.Style.Color != ink {
		t.Errorf("label style = %+v, want 18, bold, red", label.Style)
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == target.Bounds {
			found = true
			if primitive.Radius != 5 {
				t.Errorf("surface radius = %v, want the explicit 5", primitive.Radius)
			}
		}
	}
	if !found {
		t.Errorf("no surface for %v", target.Bounds)
	}
}

// Children replace the label; the toggle keeps its pressed state and hit.
func TestTogglePageChildrenReplaceTheLabel(t *testing.T) {
	on := false
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Toggle(c, &on, "Bold").Children(func() {
				ui.Text(c, "one")
				ui.Text(c, "two")
			})
		})
	})
	snapshot := d.driver.Snapshot()
	for _, text := range snapshot.Texts {
		if text.Value == "Bold" {
			t.Errorf("the label was painted over the children: %+v", snapshot.Texts)
		}
	}
	var target gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Label == "Bold" {
			target = hit
		}
	}
	if target.Role != gfx.RoleButton || target.Checked {
		t.Errorf("hit = %+v, want one unpressed button named Bold", target)
	}
	children := 0
	for _, text := range snapshot.Texts {
		if text.Value == "one" || text.Value == "two" {
			children++
			if text.Bounds.X < target.Bounds.X || text.Bounds.X+text.Bounds.Width > target.Bounds.X+target.Bounds.Width {
				t.Errorf("child %q at %v is outside the toggle %v", text.Value, text.Bounds, target.Bounds)
			}
		}
	}
	if children != 2 {
		t.Errorf("painted %d children, want 2: %+v", children, snapshot.Texts)
	}
}

// A page's Styles.Toggle dresses every toggle of the page; the state the page
// recipe asks about includes Checked.
func TestPageStylesDressToggles(t *testing.T) {
	fill := gfx.Color{Red: 0.2, Green: 0.6, Blue: 0.3, Alpha: 1}
	checked := false
	styles := &ui.Styles{Toggle: func(_ gfx.Theme, _ style.Variant, state gfx.State) style.ButtonLook {
		if state.Checked {
			checked = true
		}
		return style.ButtonLook{Background: fill, Foreground: fill, Filled: true, Radius: 2}
	}}
	on := true
	d := openPageWith(t, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() { ui.Toggle(c, &on, "Bold") })
		},
		Styles: styles,
	}, app.Config{})
	if !checked {
		t.Error("the page recipe did not see the checked state")
	}
	snapshot := d.driver.Snapshot()
	var target gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Label == "Bold" {
			target = hit
		}
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == target.Bounds {
			found = true
			if primitive.Color != fill || primitive.Radius != 2 {
				t.Errorf("surface = %v radius %v, want the page recipe's %v radius 2", primitive.Color, primitive.Radius, fill)
			}
		}
	}
	if !found {
		t.Errorf("no surface authored by the page recipe for %v", target.Bounds)
	}
}

// A steady page of toggles allocates nothing.
func TestPageOfTogglesAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseToggles)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseToggles) }); allocs != 0 {
		t.Fatalf("a steady page of toggles allocates %v times", allocs)
	}
}

// BenchmarkPageToggles is the page-level cost of themed toggles, for the
// stage-2 regression budget.
func BenchmarkPageToggles(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseToggles)
	}
}

var toggleLabels = [...]string{"Bold", "Italic", "Underline", "Code", "Link"}

// toggleState is the model of denseToggles, kept out of the declaration so a
// steady paint allocates nothing.
var toggleState [20 * len(toggleLabels)]bool

// denseToggles is a page made of toggles, as a formatting toolbar is: every
// label, the outline treatment and the sizes, in a few rows.
func denseToggles(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for row := 0; row < 20; row++ {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				for i, label := range toggleLabels {
					index := row*len(toggleLabels) + i
					value := &toggleState[index]
					*value = index%2 == 0
					toggle := ui.Toggle(c, value, label)
					switch index % 4 {
					case 1:
						toggle.Outline()
					case 2:
						toggle.Small()
					case 3:
						toggle.Large()
					}
				}
			})
		}
	})
}
