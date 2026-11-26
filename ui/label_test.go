package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page Label paints the field title's rules: the body size at medium weight,
// a danger-colored marker when Required, and the muted treatment when Disabled.
func TestPageLabelRequiredAndDisabled(t *testing.T) {
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Label(c, "Display name")
			ui.Label(c, "Organization name").Required()
			ui.Label(c, "Disabled field").Required().Disabled(true)
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	drawPage(page, canvas, view)

	normal := canvasText(canvas, "Display name")
	if normal == nil {
		t.Fatal("the plain label was not painted")
	}
	if normal.Style.Size != pageTheme.Metrics.FontSizeM || normal.Style.Weight != gfx.WeightMedium {
		t.Fatalf("plain label style = %+v, want the field title's size and weight", normal.Style)
	}
	if normal.Style.Color != pageTheme.Foreground {
		t.Fatalf("plain label color = %+v, want the foreground", normal.Style.Color)
	}
	required := canvasText(canvas, "Organization name")
	marker := canvasText(canvas, "*")
	if required == nil || marker == nil {
		t.Fatal("the required label or its marker was not painted")
	}
	if marker.Style.Color != pageTheme.Danger {
		t.Fatalf("required marker color = %+v, want danger %+v", marker.Style.Color, pageTheme.Danger)
	}
	if bounds := marker.Bounds; bounds.X < required.Bounds.X+required.Bounds.Width {
		t.Fatalf("marker at %+v is not after the words at %+v", bounds, required.Bounds)
	}
	disabled := canvasText(canvas, "Disabled field")
	if disabled == nil {
		t.Fatal("the disabled label was not painted")
	}
	wantDisabled := pageTheme.Foreground
	wantDisabled.Alpha *= 0.50
	if disabled.Style.Color != wantDisabled {
		t.Fatalf("disabled label color = %+v, want %+v", disabled.Style.Color, wantDisabled)
	}
}

// LabelFor makes the Label the control's label without making the words a node
// of their own: the hit names the control and carries the label's words.
func TestPageLabelLabelsForItsControl(t *testing.T) {
	control := gfx.Item(7)
	var field ui.Field
	field.SetText("team@viva.dev")
	d := openPage(t, func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Label(c, "Work email").Required().LabelFor(control)
			ui.TextField(c, &field, "Work email", "", ui.WithID(control))
		})
	})
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.ID == control && hit.IsLabel {
			if hit.Label != "Work email" {
				t.Fatalf("label hit names %q, want the label's words", hit.Label)
			}
			return
		}
	}
	t.Fatal("the Label declared no LabelFor hit")
}

// A warm page Label paints without allocating: the field title's paint is the
// same code the canvas component uses.
func TestPageLabelAllocatesNothingOnceWarm(t *testing.T) {
	view := func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Label(c, "Display name").Required()
			ui.Label(c, "Disabled field").Required().Disabled(true)
		})
	}
	page := ui.NewPage(ui.PageConfig{View: view})
	canvas := benchCanvas()
	draw := func() { drawPage(page, canvas, view) }
	draw()
	if allocations := testing.AllocsPerRun(100, draw); allocations != 0 {
		t.Fatalf("a warm page label allocates %v times a frame, want 0", allocations)
	}
}
