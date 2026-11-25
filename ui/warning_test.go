package ui_test

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// hasColor reports whether any primitive of the page is painted in color.
func hasColor(d *pageDriver, color gfx.Color) bool {
	for _, primitive := range d.driver.Snapshot().Primitives {
		if primitive.Color == color {
			return true
		}
	}
	return false
}

// A TextField or TextArea marked Warning takes the warning border; a field that is
// also Invalid brings the destructive one, which wins; Warning(false) takes it away.
func TestWarningBorderOfTextControls(t *testing.T) {
	warning := pageTheme.Warning
	for _, kind := range []string{"text field", "text area"} {
		t.Run(kind, func(t *testing.T) {
			var field ui.Field
			warn, invalid := false, false
			d := openPage(t, func(c ui.C) {
				field.Invalid = invalid
				var box ui.Box
				if kind == "text field" {
					box = ui.TextField(c, &field, "Name", "")
				} else {
					box = ui.TextArea(c, &field, "Name", "").Height(80)
				}
				box.Warning(warn)
			})
			if hasColor(d, warning) {
				t.Fatal("a control that is not marked paints the warning color")
			}
			warn = true
			d.repaint()
			if !hasColor(d, warning) || hasColor(d, pageTheme.Danger) {
				t.Fatalf("a warned control: warning painted %v, danger painted %v; want true, false", hasColor(d, warning), hasColor(d, pageTheme.Danger))
			}
			invalid = true
			d.repaint()
			if hasColor(d, warning) || !hasColor(d, pageTheme.Danger) {
				t.Fatalf("a control both warned and invalid: warning painted %v, danger painted %v; want false, true", hasColor(d, warning), hasColor(d, pageTheme.Danger))
			}
			warn, invalid = false, false
			d.repaint()
			if hasColor(d, warning) || hasColor(d, pageTheme.Danger) {
				t.Fatal("Warning(false) left a warning border")
			}
		})
	}
}

// A *Field takes Warning where the model is made, as it takes Invalid.
func TestFieldWarning(t *testing.T) {
	var field ui.Field
	field.Warning = true
	d := openPage(t, func(c ui.C) { ui.TextField(c, &field, "Name", "") })
	if !hasColor(d, pageTheme.Warning) {
		t.Fatal("a Field with Warning set paints no warning border")
	}
}

// A Progress takes the color of its status for its fill, and the primary without one.
func TestProgressStatusColorsTheFill(t *testing.T) {
	for name, test := range map[string]struct {
		status ui.Status
		want   gfx.Color
	}{
		"default": {ui.StatusDefault, pageTheme.Primary},
		"success": {ui.StatusSuccess, pageTheme.Success},
		"warning": {ui.StatusWarning, pageTheme.Warning},
		"danger":  {ui.StatusDanger, pageTheme.Danger},
	} {
		d := openPage(t, func(c ui.C) { ui.Progress(c, 0.5).Status(test.status).Width(200) })
		if !hasColor(d, test.want) {
			t.Fatalf("%s: the fill is not painted in %+v", name, test.want)
		}
		if test.status != ui.StatusDefault && hasColor(d, pageTheme.Primary) {
			t.Fatalf("%s: the fill kept the primary color", name)
		}
	}
}
