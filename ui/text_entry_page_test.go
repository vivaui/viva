package ui_test

import (
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

var textValue = "Gopher"
var textAreaValue = "line one"

// The calls a page makes on a text field reach its value and surface.
func TestTextFieldPagePropertiesReachTheField(t *testing.T) {
	ink, fill := gfx.Color{Red: 1, Alpha: 1}, gfx.Color{Blue: 1, Alpha: 1}
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.TextField(c, &textValue, "Name", "Type here").Width(220).Height(48).
				FontSize(20).Weight(gfx.WeightBold).Color(ink).Background(fill).Radius(6)
		})
	})
	snapshot := d.driver.Snapshot()
	var field gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleTextField {
			field = hit
		}
	}
	if field.Role != gfx.RoleTextField {
		t.Fatalf("no text-field hit: %+v", snapshot.Hits)
	}
	if field.Bounds.Height != 48 {
		t.Errorf("field height = %v, want 48", field.Bounds.Height)
	}
	var value gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "Gopher" {
			value = text
		}
	}
	if value.Value != "Gopher" {
		t.Fatalf("the value was not painted: %+v", snapshot.Texts)
	}
	if value.Style.Size != 20 || value.Style.Weight != gfx.WeightBold || value.Style.Color != ink {
		t.Errorf("value style = %+v, want 20, bold, red", value.Style)
	}
	found := false
	for _, primitive := range snapshot.Primitives {
		if primitive.Kind == app.RectanglePrimitive && primitive.Color == fill {
			found = true
		}
	}
	if !found {
		t.Errorf("no field fill %v in %+v", fill, snapshot.Primitives)
	}
}

// A text area takes the element's FontSize for its rows and its Height.
func TestTextAreaPagePropertiesReachTheRows(t *testing.T) {
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(20).Children(func() {
			ui.TextArea(c, &textAreaValue, "Notes", "Type here").Width(240).Height(120).FontSize(22).Weight(gfx.WeightBold)
		})
	})
	snapshot := d.driver.Snapshot()
	var field gfx.Hit
	for _, hit := range snapshot.Hits {
		if hit.Role == gfx.RoleTextField {
			field = hit
		}
	}
	if field.Bounds.Height != 120 {
		t.Errorf("textarea height = %v, want 120", field.Bounds.Height)
	}
	var row gfx.Text
	for _, text := range snapshot.Texts {
		if text.Value == "line one" {
			row = text
		}
	}
	if row.Value != "line one" {
		t.Fatalf("the row was not painted: %+v", snapshot.Texts)
	}
	if row.Style.Size != 22 || row.Style.Weight != gfx.WeightBold {
		t.Errorf("row style = %+v, want 22, bold", row.Style)
	}
}

// A steady page of text fields allocates nothing.
func TestPageOfTextFieldsAllocatesNothing(t *testing.T) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	for range 3 {
		drawPage(page, canvas, denseTextFields)
	}
	if allocs := testing.AllocsPerRun(50, func() { drawPage(page, canvas, denseTextFields) }); allocs != 0 {
		t.Fatalf("a steady page of text fields allocates %v times", allocs)
	}
}

// BenchmarkPageTextFields is the page-level cost of themed text fields, for the
// stage-2 regression budget.
func BenchmarkPageTextFields(b *testing.B) {
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, denseTextFields)
	}
}

var fieldValue = "value"
var areaValue = "two\nrows"

// denseTextFields is a settings form of text fields and text areas.
func denseTextFields(c ui.C) {
	ui.Column(c).Fill().Pad(12).Gap(6).Children(func() {
		for row := 0; row < 8; row++ {
			ui.TextField(c, &fieldValue, "Name", "Type here").FillWidth()
			ui.TextArea(c, &areaValue, "Notes", "Type here").FillWidth().Height(96)
		}
	})
}
