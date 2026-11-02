package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A native editor takes over a focused field and draws where the hit says its
// text is. That is only right if the hit says what the component really paints
// when it is not focused, so each text field is checked against its own static
// rendering, in both themes and both densities.
func TestTextFieldsDeclareTheGeometryTheyPaint(t *testing.T) {
	type paint func(canvas *ui.Canvas, theme ui.Theme)
	fields := []struct {
		name  string
		paint paint
	}{
		{"input", func(canvas *ui.Canvas, theme ui.Theme) {
			PaintInput(canvas, theme, ui.Rect{X: 20, Y: 20, Width: 240, Height: 30}, InputSnapshot{ID: ui.Item(5), Label: "Name", Value: "hello"})
		}},
		{"textarea", func(canvas *ui.Canvas, theme ui.Theme) {
			PaintTextarea(canvas, theme, ui.Rect{X: 20, Y: 20, Width: 240, Height: 80}, TextareaSnapshot{ID: ui.Item(5), Label: "Notes", Value: "hello"})
		}},
		{"input group", func(canvas *ui.Canvas, theme ui.Theme) {
			PaintInputGroup(canvas, theme, ui.Rect{X: 20, Y: 20, Width: 240, Height: 36}, InputGroupSnapshot{ID: ui.Item(5), Label: "Search", Value: "hello"})
		}},
		{"multiline input group", func(canvas *ui.Canvas, theme ui.Theme) {
			PaintInputGroup(canvas, theme, ui.Rect{X: 20, Y: 20, Width: 240, Height: 90}, InputGroupSnapshot{ID: ui.Item(5), Label: "Search", Value: "hello", Multiline: true})
		}},
	}
	for _, field := range fields {
		for _, dark := range []bool{false, true} {
			for _, compact := range []bool{false, true} {
				theme := ui.LightTheme()
				if dark {
					theme = ui.DarkTheme()
				}
				if compact {
					theme.Metrics = theme.Metrics.WithDensity(ui.DensityCompact)
				}
				var canvas ui.Canvas
				field.paint(&canvas, theme)

				var hit ui.Hit
				for _, candidate := range canvas.Hits() {
					if candidate.ID == ui.Item(5) && candidate.Role == ui.RoleTextField {
						hit = candidate
					}
				}
				var painted ui.Text
				for _, candidate := range canvas.Texts() {
					if candidate.Value == "hello" {
						painted = candidate
					}
				}
				if hit.ID.IsZero() || painted.Value == "" {
					t.Fatalf("%s: hit=%+v painted=%+v", field.name, hit, painted)
				}
				if hit.TextSize != painted.Style.Size || hit.TextSize != theme.Metrics.FontSizeM {
					t.Errorf("%s dark=%t compact=%t: declared size %v, painted %v, theme %v", field.name, dark, compact, hit.TextSize, painted.Style.Size, theme.Metrics.FontSizeM)
				}
				if hit.TextColor != painted.Style.Color {
					t.Errorf("%s dark=%t compact=%t: declared color %+v, painted %+v", field.name, dark, compact, hit.TextColor, painted.Style.Color)
				}
				// A multiline row may reach into the padding at its right so that platform
				// advances a little wider than the layout's do not cut its last letter;
				// where it starts, and so where the native editor's text lands, is the same.
				widening := painted.Bounds.Width - hit.TextBounds.Width
				if hit.TextBounds.X != painted.Bounds.X || widening < 0 || widening > 0 && (!hit.Multiline || widening >= testInset) {
					t.Errorf("%s dark=%t compact=%t: declared x/width %v/%v, painted %v/%v", field.name, dark, compact, hit.TextBounds.X, hit.TextBounds.Width, painted.Bounds.X, painted.Bounds.Width)
				}
				// A single line is vertically centered in the same box; a
				// multiline field starts its first row at the box's top.
				if hit.TextBounds.Y != painted.Bounds.Y {
					t.Errorf("%s dark=%t compact=%t: declared y %v, painted %v", field.name, dark, compact, hit.TextBounds.Y, painted.Bounds.Y)
				}
			}
		}
	}
}

// testInset is where the text of a text-entry control starts at the default
// density.
var testInset = ui.DefaultMetrics().TextInsetX()
