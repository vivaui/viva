package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

// The calls a page makes on a Badge reach its chip and label: FontSize, Weight
// and Color the text, Width and Height the chip, Radius its corners.
func TestBadgePagePropertiesReachTheChip(t *testing.T) {
	ink := gfx.Color{Red: 1, Alpha: 1}
	var target gfx.Hit
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Badge(c, "beta").FontSize(18).Weight(gfx.WeightBold).Color(ink).
				Width(120).Height(40).Radius(7).Hit(gfx.Item(3), "beta")
		})
	})
	snapshot := d.driver.Snapshot()
	for _, hit := range snapshot.Hits {
		if hit.ID == gfx.Item(3) {
			target = hit
		}
	}
	if target.ID != gfx.Item(3) {
		t.Fatalf("the badge declared no hit: %+v", snapshot.Hits)
	}
	if target.Bounds.Width != 120 || target.Bounds.Height != 40 {
		t.Errorf("chip = %v, want 120x40", target.Bounds)
	}
	var text gfx.Text
	for _, painted := range snapshot.Texts {
		if painted.Value == "beta" {
			text = painted
		}
	}
	if text.Value != "beta" {
		t.Fatalf("the badge label was not painted: %+v", snapshot.Texts)
	}
	if text.Style.Size != 18 || text.Style.Weight != gfx.WeightBold || text.Style.Color != ink {
		t.Errorf("label style = %+v, want size 18, bold, red", text.Style)
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == target.Bounds {
			found = true
			if primitive.Radius != 7 {
				t.Errorf("chip radius = %v, want the explicit 7", primitive.Radius)
			}
		}
	}
	if !found {
		t.Errorf("no chip rectangle for %v", target.Bounds)
	}
}

// Children replace the label; the badge stays one button for the pointer.
func TestBadgePageChildrenReplaceTheLabel(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Badge(c, "beta").Hit(gfx.Item(4), "beta").Children(func() {
				ui.Text(c, "one")
				ui.Text(c, "two")
			})
		})
	})
	snapshot := d.driver.Snapshot()
	for _, text := range snapshot.Texts {
		if text.Value == "beta" {
			t.Errorf("the label was painted over the children: %+v", snapshot.Texts)
		}
	}
	var target gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.ID == gfx.Item(4) {
			target = hit
		}
	}
	if target.Role != gfx.RoleButton || target.Label != "beta" {
		t.Errorf("hit = %+v, want one button named beta", target)
	}
	for _, text := range snapshot.Texts {
		if text.Value != "one" && text.Value != "two" {
			continue
		}
		if text.Bounds.X < target.Bounds.X || text.Bounds.X+text.Bounds.Width > target.Bounds.X+target.Bounds.Width {
			t.Errorf("child %q at %v is outside the chip %v", text.Value, text.Bounds, target.Bounds)
		}
	}
}

// A page's Styles.Badge dresses every badge of the page: the chip takes the
// page's fill, radius and label color.
func TestPageStylesDressBadges(t *testing.T) {
	fill := gfx.Color{Red: 0.1, Green: 0.4, Blue: 0.2, Alpha: 1}
	ink := gfx.Color{Red: 0.9, Green: 0.9, Blue: 0.1, Alpha: 1}
	styles := &ui.Styles{Badge: func(gfx.Theme, style.Variant, gfx.State) style.ButtonLook {
		return style.ButtonLook{Background: fill, Foreground: ink, Filled: true, Radius: 4}
	}}
	d := openPageWith(t, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() { ui.Badge(c, "beta").Hit(gfx.Item(5), "beta") })
		},
		Styles: styles,
	}, app.Config{})
	snapshot := d.driver.Snapshot()
	var target gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.ID == gfx.Item(5) {
			target = hit
		}
	}
	chip := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Bounds == target.Bounds {
			chip = true
			if primitive.Color != fill || primitive.Radius != 4 {
				t.Errorf("chip = %v radius %v, want the page recipe's %v radius 4", primitive.Color, primitive.Radius, fill)
			}
		}
	}
	if !chip {
		t.Fatalf("no chip authored by the page recipe for %v", target.Bounds)
	}
	label := false
	for _, text := range snapshot.Texts {
		if text.Value == "beta" {
			label = true
			if text.Style.Color != ink {
				t.Errorf("label color = %v, want the page recipe's %v", text.Style.Color, ink)
			}
		}
	}
	if !label {
		t.Error("the badge label was not painted")
	}
}

// A steady page of badges allocates nothing: the recipe defaults and the node
// layout reuse the frame's arena.
func TestPageOfBadgesAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseBadges)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseBadges) }); allocs != 0 {
		t.Fatalf("a steady page of badges allocates %v times", allocs)
	}
}

// BenchmarkPageBadges is the page-level cost of themed badges, for the stage-2
// regression budget.
func BenchmarkPageBadges(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseBadges)
	}
}

var badgeLabels = [...]string{"beta", "new", "deprecated", "stable", "linked"}

var badgeVariants = [...]func(ui.Box) ui.Box{
	func(b ui.Box) ui.Box { return b },
	func(b ui.Box) ui.Box { return b.Secondary() },
	func(b ui.Box) ui.Box { return b.Outline() },
	func(b ui.Box) ui.Box { return b.Ghost() },
	func(b ui.Box) ui.Box { return b.Link() },
}

// denseBadges is a page made of badges, as a status-heavy workbench row is:
// every variant, with and without Hit, in several rows.
func denseBadges(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for row := 0; row < 20; row++ {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				for i, label := range badgeLabels {
					badge := badgeVariants[(row+i)%len(badgeVariants)](ui.Badge(c, label))
					if (row+i)%2 == 0 {
						badge.Hit(gfx.Item(row*len(badgeLabels)+i), label)
					}
				}
			})
		}
	})
}
