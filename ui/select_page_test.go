package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/style"
	"github.com/vivaui/viva/ui"
)

var selectChoices = []int{0, 1, 2}

func selectName(index int) string { return [...]string{"Small", "Medium", "Large"}[index] }

// The calls a page makes on a Select reach its trigger and value.
func TestSelectPagePropertiesReachTheTrigger(t *testing.T) {
	choice, ink, fill := 1, gfx.Color{Red: 1, Alpha: 1}, gfx.Color{Blue: 1, Alpha: 1}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Select(c, &choice, selectChoices, selectName).Placeholder("Size").
				FontSize(18).Weight(gfx.WeightBold).Color(ink).Height(48).Radius(5).Background(fill)
		})
	})
	snapshot := d.driver.Snapshot()
	var trigger gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit
		}
	}
	if trigger.Role != gfx.RoleCombobox || trigger.Value != "Medium" {
		t.Fatalf("trigger = %+v, want a combobox showing Medium", trigger)
	}
	if trigger.Bounds.Height != 48 {
		t.Errorf("trigger height = %v, want 48", trigger.Bounds.Height)
	}
	var value gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "Medium" {
			value = text
		}
	}
	if value.Value != "Medium" {
		t.Fatalf("the value was not painted: %+v", snapshot.Texts)
	}
	if value.Style.Size != 18 || value.Style.Weight != gfx.WeightBold || value.Style.Color != ink {
		t.Errorf("value style = %+v, want 18, bold, red", value.Style)
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill {
			found = true
		}
	}
	if !found {
		t.Errorf("no trigger fill %v in %+v", fill, snapshot.Primitives)
	}
	// The chevron stays the control's: triangles are painted at the trigger's end.
	triangles := 0
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.TrianglePrimitive {
			triangles++
		}
	}
	if triangles == 0 {
		t.Error("the trigger painted no chevron")
	}
}

// Children replace the chosen value while the combobox hit and the chevron stay.
func TestSelectPageChildrenReplaceTheValue(t *testing.T) {
	choice := 0
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.Select(c, &choice, selectChoices, selectName).Placeholder("Size").Children(func() {
				ui.Text(c, "own")
			})
		})
	})
	snapshot := d.driver.Snapshot()
	for _, text := range snapshot.Texts {
		if text.Value == "Small" || text.Value == "Size" {
			t.Errorf("the value or placeholder was painted over the children: %+v", snapshot.Texts)
		}
	}
	var trigger gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleCombobox {
			trigger = hit
		}
	}
	if trigger.Role != gfx.RoleCombobox {
		t.Fatalf("children dropped the trigger's combobox hit: %+v", snapshot.Hits)
	}
	triangles := 0
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.TrianglePrimitive {
			triangles++
		}
	}
	if triangles == 0 {
		t.Error("children dropped the trigger's chevron")
	}
}

// A page's Styles.Select dresses the trigger: its surface fill and radius.
func TestPageStylesDressSelects(t *testing.T) {
	choice := 0
	fill := gfx.Color{Red: 0.2, Green: 0.6, Blue: 0.3, Alpha: 1}
	styles := &ui.Styles{Select: func(gfx.Theme, gfx.State, bool) (style.TextEntryLook, gfx.Color, gfx.Color) {
		surface := style.TextEntryLook{Fill: fill, Radius: 9, BorderWidth: 1, Border: gfx.Color{Alpha: 1}}
		return surface, gfx.Color{Alpha: 1}, gfx.Color{Alpha: 1}
	}}
	d := openPageWith(t, ui.PageConfig{
		View: func(c ui.C) {
			ui.Column(c).Pad(20).Children(func() { ui.Select(c, &choice, selectChoices, selectName).Placeholder("Size") })
		},
		Styles: styles,
	}, app.Config{})
	snapshot := d.driver.Snapshot()
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill {
			found = true
		}
	}
	if !found {
		t.Errorf("no trigger authored by the page recipe with fill %v", fill)
	}
}

var selectState = 0

// A steady page of selects allocates nothing.
func TestPageOfSelectsAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseSelects)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseSelects) }); allocs != 0 {
		t.Fatalf("a steady page of selects allocates %v times", allocs)
	}
}

// BenchmarkPageSelects is the page-level cost of select triggers, for the
// stage-2 regression budget.
func BenchmarkPageSelects(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseSelects)
	}
}

// denseSelects is a page of closed select triggers, as a settings form is.
func denseSelects(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for row := 0; row < 20; row++ {
			ui.Row(c).FillWidth().Gap(6).Children(func() {
				ui.Select(c, &selectState, selectChoices, selectName).Placeholder("Size").Width(120)
				ui.Select(c, &selectState, selectChoices, selectName).Placeholder("Size").Width(160).Small()
			})
		}
	})
}
