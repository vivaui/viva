package ui_test

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A form field's title labels the control its children declare, as an HTML
// label labels the control inside it: pressing the title focuses the field.
// A TextField given WithID edits a model string under the application's own identity.
func TestAFormFieldTitleLabelsItsControl(t *testing.T) {
	var row ui.FormFieldState
	email := ""
	id := ui.Identity{}
	d := openPage(t, func(c ui.C) {
		if id.IsZero() {
			id = ui.ID(c)
		}
		ui.FormField(c, &row, "Work email").FillWidth().Children(func() {
			ui.TextField(c, &email, "Email", "you@example.com", ui.WithID(id)).FillWidth()
		})
	})
	var title ui.Rect
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.IsLabel && hit.Label == "Work email" {
			title = hit.Bounds
		}
	}
	if title.Width <= 0 {
		t.Fatal("the title labels no control")
	}
	if err := d.driver.Click(title.X+2, title.Y+title.Height/2); err != nil {
		t.Fatal(err)
	}
	if d.Focused() != "Email" || d.driver.Snapshot().Focused != id {
		t.Fatalf("pressing the title put the keyboard on %q", d.Focused())
	}
	d.typeText("a@b.c")
	if email != "a@b.c" {
		t.Fatalf("the field edited %q into the model", email)
	}
}
