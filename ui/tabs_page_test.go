package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

var tabsChoices = []int{0, 1, 2}

func tabsLabel(index int) string { return [...]string{"General", "Keys", "Advanced"}[index] }

// The calls a page makes on a Tabs strip reach the strip and its labels.
func TestTabsPagePropertiesReachTheStrip(t *testing.T) {
	current, ink, fill := 1, gfx.Color{Red: 1, Alpha: 1}, gfx.Color{Blue: 1, Alpha: 1}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Tabs(c, &current, tabsChoices, tabsLabel).FontSize(18).Weight(gfx.WeightBold).
				Color(ink).Pad(7).Radius(4).Background(fill).Height(44)
		})
	})
	snapshot := d.driver.Snapshot()
	// The strip holds three tab-role hits; its own rectangle is the painted fill.
	tabHits := 0
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleTab {
			tabHits++
		}
	}
	if tabHits != 3 {
		t.Fatalf("%d tab hits, want 3", tabHits)
	}
	// Find the strip's surface: it spans all triggers and has the explicit fill.
	list := gfx.Rect{}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill && primitive.Radius == 4 {
			list, found = primitive.Bounds, true
		}
	}
	if !found {
		t.Fatalf("no strip painted with the explicit fill %v radius 4", fill)
	}
	if list.Height != 44 {
		t.Errorf("strip height = %v, want 44", list.Height)
	}
	dimmed := ink
	dimmed.Alpha *= 0.60
	seen := 0
	for _, text := range snapshot.Texts {
		label := text.Value
		if label != "General" && label != "Keys" && label != "Advanced" {
			continue
		}
		seen++
		if text.Style.Size != 18 || text.Style.Weight != gfx.WeightBold {
			t.Errorf("%s style = %+v, want 18, bold", label, text.Style)
		}
		want := dimmed
		if label == "Keys" {
			want = ink // the selected tab keeps the ink whole
		}
		if text.Style.Color != want {
			t.Errorf("%s color = %v, want %v", label, text.Style.Color, want)
		}
	}
	if seen != 3 {
		t.Errorf("%d labels painted, want 3", seen)
	}
}

// A page's Styles.Tabs dresses the strip: its fill and radius.
func TestPageStylesDressTabs(t *testing.T) {
	current := 0
	fill := gfx.Color{Red: 0.1, Green: 0.5, Blue: 0.2, Alpha: 1}
	styles := &ui.Styles{Tabs: func(gfx.Theme, bool) style.ButtonLook {
		return style.ButtonLook{Background: fill, Filled: true, Radius: 8}
	}}
	d := openPageWith(t, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() { ui.Tabs(c, &current, tabsChoices, tabsLabel) })
		},
		Styles: styles,
	}, app.Config{})
	snapshot := d.driver.Snapshot()
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill && primitive.Radius == 8 {
			found = true
		}
	}
	if !found {
		t.Errorf("no strip authored by the page recipe with fill %v radius 8", fill)
	}
}

var tabsState = 0

// A steady page of tab strips allocates nothing.
func TestPageOfTabsAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseTabs)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseTabs) }); allocs != 0 {
		t.Fatalf("a steady page of tabs allocates %v times", allocs)
	}
}

// BenchmarkPageTabs is the page-level cost of tab strips, for the stage-2
// regression budget.
func BenchmarkPageTabs(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseTabs)
	}
}

// denseTabs is a page of tab strips: the default and line variants, horizontal
// and vertical, at the sizes a settings page uses.
func denseTabs(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(8).Children(func() {
		for row := 0; row < 12; row++ {
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				tabs := ui.Tabs(c, &tabsState, tabsChoices, tabsLabel)
				switch row % 4 {
				case 1:
					tabs.Line()
				case 2:
					tabs.Vertical().Height(108)
				case 3:
					tabs.Line().Vertical().Height(108)
				}
			})
		}
	})
}
