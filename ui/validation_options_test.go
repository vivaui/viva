package ui_test

import (
	"fmt"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// A page Checkbox, Toggle or Select marked Invalid paints with the official
// destructive border, and its valid twin does not.
func TestInvalidPageControlsPaintTheDestructiveBorder(t *testing.T) {
	for _, kind := range []string{"checkbox", "toggle", "select"} {
		t.Run(kind, func(t *testing.T) {
			look := func(invalid bool) (string, bool) {
				var on, active bool
				size := "M"
				d := openPage(t, page(func(c ui.C) {
					switch kind {
					case "checkbox":
						ui.Checkbox(c, &on, "Accept").Invalid(invalid)
					case "toggle":
						ui.Toggle(c, &active, "Bold").Outline().Invalid(invalid)
					case "select":
						ui.Select(c, &size, []string{"S", "M"}, nil).Placeholder("Size").Invalid(invalid)
					}
				}))
				primitives := d.driver.Snapshot().Primitives
				danger := false
				for _, primitive := range primitives {
					danger = danger || primitive.Color == pageTheme.Danger
				}
				return fmt.Sprint(primitives), danger
			}
			valid, validDanger := look(false)
			invalid, invalidDanger := look(true)
			if valid == invalid {
				t.Fatal("the invalid control is painted like the valid one")
			}
			if validDanger {
				t.Fatal("the valid control painted the destructive border")
			}
			if !invalidDanger {
				t.Fatal("the invalid control painted no destructive border")
			}
		})
	}
}

// A page control marked Invalid also publishes that state on its hit, so the
// runtime announces it to assistive technology.
func TestInvalidPageControlsCarryTheSemanticState(t *testing.T) {
	for _, kind := range []string{"checkbox", "switch", "toggle", "select", "text field", "text area", "combobox"} {
		t.Run(kind, func(t *testing.T) {
			var on, active bool
			size, framework := "M", "Go"
			var field ui.Field
			field.Invalid = true
			d := openPage(t, page(func(c ui.C) {
				switch kind {
				case "checkbox":
					ui.Checkbox(c, &on, "Accept").Invalid(true)
				case "switch":
					ui.Switch(c, &on, "Notifications").Invalid(true)
				case "toggle":
					ui.Toggle(c, &active, "Bold").Outline().Invalid(true)
				case "select":
					ui.Select(c, &size, []string{"S", "M"}, nil).Placeholder("Size").Invalid(true)
				case "text field":
					ui.TextField(c, &field, "Email", "", ui.WithID(gfx.Item(1)))
				case "text area":
					ui.TextArea(c, &field, "Notes", "", ui.WithID(gfx.Item(1)))
				case "combobox":
					ui.Combobox(c, &framework, []string{"Go", "Rust"}, func(value string) string { return value }, "Framework").Invalid(true)
				}
			}))
			hits := d.driver.Snapshot().Hits
			if len(hits) == 0 {
				t.Fatal("no hit was declared")
			}
			for _, hit := range hits {
				if hit.Backdrop || hit.IsLabel {
					continue
				}
				if !hit.Invalid {
					t.Fatalf("hit %d is not invalid: %+v", hit.ID, hit)
				}
			}
		})
	}
}

// DisabledOptions on a page Choice marks just those options unavailable, and a
// whole-control Disabled makes every option inert.
func TestChoiceDisablesOptionsFromThePage(t *testing.T) {
	view := "List"
	d := openPage(t, page(func(c ui.C) {
		ui.Choice(c, &view, []string{"List", "Board", "Timeline"}, nil).DisabledOptions([]bool{false, false, true})
	}))
	if hit := d.hit("Timeline"); !hit.Disabled || hit.Focusable {
		t.Fatalf("Timeline hit = %+v, want disabled and not focusable", hit)
	}
	if hit := d.hit("Board"); hit.Disabled || !hit.Focusable {
		t.Fatalf("Board hit = %+v, want enabled and focusable", hit)
	}
	d.click("Timeline")
	if view != "List" {
		t.Fatalf("a disabled option was chosen: %q", view)
	}
	d.click("Board")
	if view != "Board" {
		t.Fatalf("an enabled option was not chosen: %q", view)
	}

	locked := "List"
	d = openPage(t, page(func(c ui.C) {
		ui.Choice(c, &locked, []string{"List", "Board"}, nil).Disabled(true)
	}))
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleRadio && (!hit.Disabled || hit.Focusable) {
			t.Fatalf("a disabled control's option is live: %+v", hit)
		}
	}
}

// Declaring the chained validation options allocates nothing once the page is warm.
func TestValidationOptionsPaintWithoutAllocating(t *testing.T) {
	var on, active bool
	size, view := "M", "List"
	labels := []string{"S", "M"}
	views := []string{"List", "Board", "Timeline"}
	disabled := []bool{false, false, true}
	page, canvas := ui.NewPage(ui.PageConfig{Theme: &pageTheme}), benchCanvas()
	draw := func(c ui.C) {
		ui.Column(c).Pad(16).Gap(8).Children(func() {
			ui.Checkbox(c, &on, "Accept").Invalid(!on)
			ui.Toggle(c, &active, "Bold").Outline().Invalid(true)
			ui.Select(c, &size, labels, nil).Placeholder("Size").Invalid(true)
			ui.Choice(c, &view, views, nil).DisabledOptions(disabled)
		})
	}
	drawPage(page, canvas, draw)
	if allocations := testing.AllocsPerRun(50, func() { drawPage(page, canvas, draw) }); allocations != 0 {
		t.Fatalf("a warm paint of the validation options allocated %v times", allocations)
	}
}

// A Disabled page Select is reported and painted inert.
func TestDisabledPageSelectIsInert(t *testing.T) {
	size := "M"
	d := openPage(t, page(func(c ui.C) {
		ui.Select(c, &size, []string{"S", "M"}, nil).Placeholder("Size").Disabled(true)
	}))
	for _, hit := range d.driver.Snapshot().Hits {
		if hit.Role == gfx.RoleCombobox {
			if !hit.Disabled || hit.Focusable {
				t.Fatalf("disabled select hit = %+v", hit)
			}
			return
		}
	}
	t.Fatal("no select trigger")
}
