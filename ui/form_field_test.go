package ui_test

import (
	"testing"

	"github.com/vivaui/viva/field"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

type formFieldModel struct {
	state   ui.FormFieldState
	field   ui.Box
	control ui.Box
}

func (m *formFieldModel) view(c ui.C) {
	m.field = ui.FormField(c, &m.state, "Email").Width(220).Children(func() {
		m.control = ui.Slot(c)
	})
}

// A page FormField and a painted components.BeginField are the same field: the
// control sits in the same slot and the title is painted in the same row. The
// two go through one internal layout, so this compares the page against the
// component it replaces, not against a second copy of the formula.
func TestFormFieldMatchesBeginField(t *testing.T) {
	m := &formFieldModel{}
	page := ui.NewPage(ui.PageConfig{View: m.view})
	canvas := benchCanvas()
	drawPage(page, canvas, m.view)

	layout := field.Measure(pageTheme, m.field.Rect(), field.Spec{Title: "Email"})
	if got := m.control.Rect(); got != layout.Control {
		t.Fatalf("control slot %+v, shared layout says %+v", got, layout.Control)
	}
	auto := field.Measure(pageTheme, gfx.Rect{Width: m.field.Rect().Width}, field.Spec{Title: "Email"})
	if m.field.Rect().Height != auto.Bounds.Height {
		t.Fatalf("field height %v, BeginField's auto height is %v", m.field.Rect().Height, auto.Bounds.Height)
	}
	var label *gfx.Text
	for i := range canvas.Texts() {
		if canvas.Texts()[i].Value == "Email" {
			label = &canvas.Texts()[i]
			break
		}
	}
	if label == nil {
		t.Fatal("the field painted no title")
	}
	if label.Bounds.X != layout.Label.X || label.Bounds.Y != layout.Label.Y || label.Bounds.Height != layout.Label.Height {
		t.Fatalf("title at %+v, shared layout says %+v", label.Bounds, layout.Label)
	}
	if len(canvas.Hits()) != 0 {
		t.Fatalf("a field without For declared %d hits", len(canvas.Hits()))
	}
}

type fixedFormFieldModel struct {
	state   ui.FormFieldState
	field   ui.Box
	control ui.Box
}

func (m *fixedFormFieldModel) view(c ui.C) {
	ui.Column(c).Fill().Pad(10).Children(func() {
		ui.Row(c).Height(70).Width(240).Children(func() {
			m.field = ui.FormField(c, &m.state, "Proxy").Grow(1).Children(func() {
				m.control = ui.Row(c).Fill()
			})
		})
	})
}

// A fixed row gives the field its height: the field fills the row and keeps the
// shared control slot, instead of taking BeginField's self-measured height
// (which reserves a description and an error) and overflowing the row.
func TestFormFieldFillsAFixedRow(t *testing.T) {
	m := &fixedFormFieldModel{}
	page := ui.NewPage(ui.PageConfig{View: m.view})
	canvas := benchCanvas()
	drawPage(page, canvas, m.view)

	bounds := m.field.Rect()
	if bounds.Height != 70 {
		t.Fatalf("field height = %v, want the row's 70", bounds.Height)
	}
	layout := field.Measure(pageTheme, bounds, field.Spec{Title: "Proxy"})
	if got := m.control.Rect(); got != layout.Control {
		t.Fatalf("control slot %+v, shared layout says %+v", got, layout.Control)
	}
	if m.control.Rect().Y+m.control.Rect().Height > bounds.Y+bounds.Height {
		t.Fatalf("control %+v overflows the field %+v", m.control.Rect(), bounds)
	}
}

type popupFieldModel struct {
	state ui.FormFieldState
	field ui.Box
	zone  string
}

var fieldZones = []string{"Asia/Shanghai", "Asia/Tokyo", "Europe/Berlin"}

func (m *popupFieldModel) view(c ui.C) {
	ui.Column(c).Fill().Pad(10).Items(ui.Start).Children(func() {
		m.field = ui.FormField(c, &m.state, "Zone").Width(240).Children(func() {
			ui.Combobox(c, &m.zone, fieldZones, nil, "Time zone")
		})
	})
}

// The field does not clip its control: the Combobox's list is free to leave the
// field's bounds, as the field only lays out the control slot and paints the
// title around it.
func TestFormFieldDoesNotClipAControlPopup(t *testing.T) {
	m := &popupFieldModel{}
	d := openPage(t, m.view)
	d.click("Time zone")
	field := m.field.Rect()
	found := false
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleOption && hit.Bounds.Bottom() > field.Bottom() {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no option extends past the field %+v; the field clipped its control", field)
	}
}

// A warm page with a FormField allocates nothing: the state holds the layout and
// the paint callback, and the shared fieldlayout is value math.
func TestFormFieldAllocatesNothing(t *testing.T) {
	m := &formFieldModel{}
	page := ui.NewPage(ui.PageConfig{View: m.view})
	canvas := benchCanvas()
	drawPage(page, canvas, m.view)
	run := func() { drawPage(page, canvas, m.view) }
	if allocs := testing.AllocsPerRun(100, run); allocs != 0 {
		t.Fatalf("a warm page with a FormField allocates %v times a frame, want 0", allocs)
	}
}

func BenchmarkFormField(b *testing.B) {
	m := &formFieldModel{}
	page := ui.NewPage(ui.PageConfig{View: m.view})
	canvas := benchCanvas()
	b.ReportAllocs()
	for b.Loop() {
		drawPage(page, canvas, m.view)
	}
}
