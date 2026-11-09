package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A native editor takes over a focused field and draws where the hit says its
// text is. That is only right if the hit says what the component really paints
// when it is not focused, so each text field is checked against its own static
// rendering, in both themes and both densities.
func TestTextFieldsDeclareTheGeometryTheyPaint(t *testing.T) {
	type paint func(canvas *gfx.Canvas, theme gfx.Theme)
	fields := []struct {
		name  string
		paint paint
	}{
		{"input", func(canvas *gfx.Canvas, theme gfx.Theme) {
			PaintInput(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 240, Height: 30}, InputSnapshot{ID: gfx.Item(5), Label: "Name", Value: "hello"})
		}},
		{"textarea", func(canvas *gfx.Canvas, theme gfx.Theme) {
			PaintTextarea(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 240, Height: 80}, TextareaSnapshot{ID: gfx.Item(5), Label: "Notes", Value: "hello"})
		}},
		{"input group", func(canvas *gfx.Canvas, theme gfx.Theme) {
			PaintInputGroup(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 240, Height: 36}, InputGroupSnapshot{ID: gfx.Item(5), Label: "Search", Value: "hello"})
		}},
		{"multiline input group", func(canvas *gfx.Canvas, theme gfx.Theme) {
			PaintInputGroup(canvas, theme, gfx.Rect{X: 20, Y: 20, Width: 240, Height: 90}, InputGroupSnapshot{ID: gfx.Item(5), Label: "Search", Value: "hello", Multiline: true})
		}},
	}
	for _, field := range fields {
		for _, dark := range []bool{false, true} {
			for _, compact := range []bool{false, true} {
				theme := gfx.LightTheme()
				if dark {
					theme = gfx.DarkTheme()
				}
				if compact {
					theme.Metrics = theme.Metrics.WithDensity(gfx.DensityCompact)
				}
				var canvas gfx.Canvas
				field.paint(&canvas, theme)

				var hit gfx.Hit
				for _, candidate := range canvas.Hits() {
					if candidate.ID == gfx.Item(5) && candidate.Role == gfx.RoleTextField {
						hit = candidate
					}
				}
				var painted gfx.Text
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
var testInset = gfx.DefaultMetrics().TextInsetX()
