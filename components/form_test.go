package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestFormFieldReturnsDeterministicControlAndMetadataGeometry(t *testing.T) {
	var canvas gfx.Canvas
	layout := PaintFormField(&canvas, gfx.LightTheme(), gfx.Rect{X: 10, Y: 20, Width: 200}, FormFieldSnapshot{
		Label: "Email", Description: "Used for receipts", Message: "Fallback message",
		Error: "Enter a valid address", Required: true,
	})
	wantControl := gfx.Rect{X: 10, Y: 42, Width: 200, Height: 32}
	if layout.Control != wantControl {
		t.Fatalf("control = %+v, want %+v", layout.Control, wantControl)
	}
	if layout.Description != (gfx.Rect{X: 10, Y: 78, Width: 200, Height: 21}) {
		t.Fatalf("description = %+v", layout.Description)
	}
	if layout.Message != (gfx.Rect{X: 10, Y: 103, Width: 200, Height: 21}) {
		t.Fatalf("message = %+v", layout.Message)
	}
	if layout.Bounds != (gfx.Rect{X: 10, Y: 20, Width: 200, Height: 104}) {
		t.Fatalf("bounds = %+v", layout.Bounds)
	}
	if len(canvas.Rectangles()) != 0 || len(canvas.Hits()) != 0 {
		t.Fatal("form recipe must not paint or register the caller-owned control")
	}
	if len(canvas.Texts()) != 4 || canvas.Texts()[0].Value != "Email" || canvas.Texts()[3].Value != "Enter a valid address" {
		t.Fatalf("texts = %+v", canvas.Texts())
	}
}

func TestFormFieldErrorStateAndTypedControlSemantics(t *testing.T) {
	theme := gfx.DarkTheme()
	var canvas gfx.Canvas
	layout := PaintFormField(&canvas, theme, gfx.Rect{Width: 180}, FormFieldSnapshot{
		Label: "Framework", Description: "Choose one", Message: "Selection required",
		Invalid: true, Disabled: true, Required: true,
	})
	semantics := layout.Semantics
	if semantics.Label != "Framework" || semantics.Hint != "Choose one" || semantics.Error != "Selection required" {
		t.Fatalf("semantics = %+v", semantics)
	}
	if !semantics.Required || !semantics.Disabled || !semantics.Invalid || semantics.AssistiveText() != "Selection required" {
		t.Fatalf("semantic state = %+v", semantics)
	}
	PaintNativeSelect(&canvas, theme, layout.Control, NativeSelectSnapshot{
		ID: gfx.Item(7), Label: semantics.Label, Hint: semantics.AssistiveText(),
		Disabled: semantics.Disabled, Invalid: semantics.Invalid,
	})
	hit := canvas.Hits()[0]
	if hit.Label != "Framework" || hit.Hint != "Selection required" || !hit.Disabled || hit.Focusable {
		t.Fatalf("control hit = %+v", hit)
	}
	if canvas.Texts()[0].Style.Color != formHalfAlpha(theme.Danger) || canvas.Texts()[3].Style.Color != formHalfAlpha(theme.Danger) {
		t.Fatalf("disabled invalid colors = %+v", canvas.Texts())
	}
}

func TestFormFieldOmitsAbsentDescriptionAndMessage(t *testing.T) {
	var canvas gfx.Canvas
	layout := PaintFormField(&canvas, gfx.LightTheme(), gfx.Rect{X: 3, Y: 5, Width: 140}, FormFieldSnapshot{Label: "Name"})
	if layout.Description != (gfx.Rect{}) || layout.Message != (gfx.Rect{}) {
		t.Fatalf("optional rows = description %+v, message %+v", layout.Description, layout.Message)
	}
	if layout.Bounds.Height != 54 || len(canvas.Texts()) != 1 {
		t.Fatalf("layout = %+v texts = %+v", layout, canvas.Texts())
	}
	if layout.Semantics.AssistiveText() != "" {
		t.Fatalf("assistive text = %q", layout.Semantics.AssistiveText())
	}
}

func TestFormFieldPaintIsDeterministicAndAllocationFreeAfterWarmup(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		PaintFormField(&canvas, gfx.LightTheme(), gfx.Rect{X: 2, Y: 4, Width: 220}, FormFieldSnapshot{
			Label: "Email", Description: "Work address", Error: "Invalid email", Required: true,
		})
	}
	paint()
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical form snapshots produced different text streams")
	}
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("form paint allocations = %v", allocations)
	}
}

func formHalfAlpha(color gfx.Color) gfx.Color {
	color.Alpha *= 0.5
	return color
}
