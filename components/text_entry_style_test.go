package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

// A text-entry control painted with its own radius or font size must look as
// that size says, whole: the surface, the text, the size the platform editor is
// told to type in, and what is drawn over the text (caret, selection, the
// composition underline), which has to stay the height of the text.

// textEntryMarks splits what an Input paints after its surface (the border and
// fill) into the selection, the composition underline and the caret.
func textEntryMarks(t *testing.T, canvas *ui.Canvas) (selection, underline, caret ui.Rect) {
	t.Helper()
	rectangles := canvas.Rectangles()
	if len(rectangles) != 5 {
		t.Fatalf("got %d rectangles, want the surface (2) and selection, underline, caret: %+v", len(rectangles), rectangles)
	}
	return rectangles[2].Bounds, rectangles[3].Bounds, rectangles[4].Bounds
}

func paintMarkedInput(theme ui.Theme, bounds ui.Rect) *ui.Canvas {
	canvas := &ui.Canvas{}
	PaintInput(canvas, theme, bounds, InputSnapshot{
		ID: ui.Item(1), Label: "Name", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3,
		SelectionStart: 0, SelectionEnd: 2, CompositionStart: 3, CompositionEnd: 5,
	})
	return canvas
}

// At the default font size nothing moved: the caret is 16px tall and the
// selection fills the 24px band of a 32px field, four from the border.
func TestInputMarksAtTheDefaultSizeKeepTheirGeometry(t *testing.T) {
	canvas := paintMarkedInput(ui.LightTheme(), ui.Rect{Width: 200})
	selection, underline, caret := textEntryMarks(t, canvas)
	if selection.Y != 4 || selection.Height != 24 {
		t.Errorf("selection = %+v, want the band 4..28", selection)
	}
	if underline.Y != 27 || underline.Height != 1 {
		t.Errorf("composition underline = %+v, want the foot of the band at 27", underline)
	}
	if caret.Y != 8 || caret.Height != 16 {
		t.Errorf("caret = %+v, want 16px centered in the field", caret)
	}
}

func TestInputWithAnotherFontSizeScalesWhatItPaintsOverTheText(t *testing.T) {
	const size = 20
	bounds := ui.Rect{Y: 10, Width: 200, Height: 48}
	canvas := paintMarkedInput(ui.LightTheme().WithFontSize(size), bounds)
	selection, underline, caret := textEntryMarks(t, canvas)

	if caret.Height != size+2 {
		t.Errorf("caret height = %v, want the text's %d plus two", caret.Height, size+2)
	}
	if selection.Height != size+10 {
		t.Errorf("selection height = %v, want %d: it follows the text, not the field", selection.Height, size+10)
	}
	middle := bounds.Y + bounds.Height/2
	for name, rect := range map[string]ui.Rect{"selection": selection, "caret": caret} {
		if got := rect.Y + rect.Height/2; got != middle {
			t.Errorf("%s is centered at %v, want the field's middle %v", name, got, middle)
		}
	}
	if foot := selection.Y + selection.Height - 1; underline.Y != foot {
		t.Errorf("composition underline at %v, want the foot of the selection band %v", underline.Y, foot)
	}
	if text := canvas.Texts()[0]; text.Style.Size != size {
		t.Errorf("text size = %v, want %d", text.Style.Size, size)
	}
	if hit := canvas.Hits()[0]; hit.TextSize != size {
		t.Errorf("the platform editor is told %v, want %d", hit.TextSize, size)
	}
}

// A field too short for its text keeps what it paints inside its border.
func TestInputMarksStayInsideAShortField(t *testing.T) {
	bounds := ui.Rect{Width: 200, Height: 24}
	canvas := paintMarkedInput(ui.LightTheme().WithFontSize(30), bounds)
	selection, underline, caret := textEntryMarks(t, canvas)
	for name, rect := range map[string]ui.Rect{"selection": selection, "underline": underline, "caret": caret} {
		if rect.Y < 4 || rect.Y+rect.Height > bounds.Height-4+0.001 {
			t.Errorf("%s = %+v leaves the 4px inside the border of a %v field", name, rect, bounds.Height)
		}
	}
}

func TestInputWithAnotherRadiusPaintsItAndLeavesTheThemeAlone(t *testing.T) {
	theme := ui.DarkTheme()
	var canvas ui.Canvas
	PaintInput(&canvas, theme.WithRadius(3), ui.Rect{Width: 200}, InputSnapshot{ID: ui.Item(1), Label: "x"})
	if got := canvas.Rectangles()[0].Radius; got != 3 {
		t.Errorf("border radius = %v, want 3", got)
	}
	canvas.Reset()
	PaintInput(&canvas, theme, ui.Rect{Width: 200}, InputSnapshot{ID: ui.Item(1), Label: "x"})
	if got := canvas.Rectangles()[0].Radius; got != theme.Metrics.RadiusControl {
		t.Errorf("the next field painted with theme radius %v, want %v: an override must not leak", got, theme.Metrics.RadiusControl)
	}
	canvas.Reset()
	PaintInput(&canvas, theme.WithRadius(ui.RadiusFull), ui.Rect{Width: 200}, InputSnapshot{ID: ui.Item(1), Label: "x"})
	if got := canvas.Rectangles()[0].Radius; got < 16 {
		t.Errorf("full radius = %v, want at least half the field's height", got)
	}
}

// The text of a text-entry control starts where the control's own padding puts
// it, so a denser or looser theme moves the text with the border.
func TestTextEntryInsetFollowsDensity(t *testing.T) {
	for name, density := range map[string]ui.Density{"compact": ui.DensityCompact, "regular": ui.DensityRegular, "large": ui.DensityLarge} {
		theme := ui.LightTheme()
		theme.Metrics = theme.Metrics.WithDensity(density)
		want := theme.Metrics.TextInsetX()

		var canvas ui.Canvas
		PaintInput(&canvas, theme, ui.Rect{Width: 200}, InputSnapshot{ID: ui.Item(1), Label: "x", Value: "v"})
		if got := canvas.Hits()[0].TextBounds.X; got != want {
			t.Errorf("%s: Input text starts at %v, want %v", name, got, want)
		}
		canvas.Reset()
		PaintTextarea(&canvas, theme, ui.Rect{Width: 200, Height: 80}, TextareaSnapshot{ID: ui.Item(1), Label: "x", Value: "v"})
		if got := canvas.Hits()[0].TextBounds.X; got != want {
			t.Errorf("%s: Textarea text starts at %v, want %v", name, got, want)
		}
		canvas.Reset()
		PaintInputGroup(&canvas, theme, ui.Rect{Width: 200}, InputGroupSnapshot{ID: ui.Item(1), Label: "x", Value: "v"})
		if got := canvas.Hits()[0].TextBounds.X; got != want {
			t.Errorf("%s: InputGroup text starts at %v, want %v", name, got, want)
		}
	}
}

func TestTextareaCaretFollowsTheFontSize(t *testing.T) {
	for _, size := range []float32{14, 18, 12} {
		var canvas ui.Canvas
		PaintTextarea(&canvas, ui.LightTheme().WithFontSize(size), ui.Rect{Width: 300, Height: 120}, TextareaSnapshot{
			ID: ui.Item(1), Value: "hi", Focused: true, ShowCaret: true, Caret: 1, HideEditorText: true,
		})
		var caret *ui.Rectangle
		for i, rectangle := range canvas.Rectangles() {
			if rectangle.Bounds.Width == 1 {
				caret = &canvas.Rectangles()[i]
			}
		}
		if caret == nil {
			t.Fatalf("size %v: no caret painted", size)
		}
		if caret.Bounds.Height != size+2 {
			t.Errorf("size %v: caret height = %v, want %v", size, caret.Bounds.Height, size+2)
		}
	}
}

// Every control of the family paints its text at the size it was given and its
// surface with the radius it was given, so one override reaches them all.
func TestTextEntryFamilyHonorsAnOverriddenRadiusAndFontSize(t *testing.T) {
	const size, radius = 19, 4
	theme := ui.LightTheme().WithFontSize(size).WithRadius(radius)
	controls := map[string]func(*ui.Canvas){
		"Input": func(c *ui.Canvas) {
			PaintInput(c, theme, ui.Rect{Width: 220}, InputSnapshot{ID: ui.Item(1), Label: "x", Value: "value"})
		},
		"Textarea": func(c *ui.Canvas) {
			PaintTextarea(c, theme, ui.Rect{Width: 220, Height: 80}, TextareaSnapshot{ID: ui.Item(1), Label: "x", Value: "value"})
		},
		"InputGroup": func(c *ui.Canvas) {
			PaintInputGroup(c, theme, ui.Rect{Width: 220}, InputGroupSnapshot{ID: ui.Item(1), Label: "x", Value: "value"})
		},
		"NativeSelect": func(c *ui.Canvas) {
			PaintNativeSelect(c, theme, ui.Rect{Width: 220}, NativeSelectSnapshot{ID: ui.Item(1), Label: "x", Display: "value"})
		},
		"InputOTP": func(c *ui.Canvas) {
			PaintInputOTP(c, theme, ui.Rect{}, InputOTPSnapshot{ID: ui.Item(1), Label: "x", Value: "123", Slots: 3})
		},
	}
	for name, paint := range controls {
		t.Run(name, func(t *testing.T) {
			var canvas ui.Canvas
			paint(&canvas)
			if len(canvas.Texts()) == 0 {
				t.Fatal("painted no text")
			}
			for _, text := range canvas.Texts() {
				if text.Style.Size != size {
					t.Errorf("text %q is %v, want %d", text.Value, text.Style.Size, size)
				}
			}
			if got := canvas.Rectangles()[0].Radius; got != radius {
				t.Errorf("surface radius = %v, want %d", got, radius)
			}
		})
	}
}

func TestInputPaintWithOverridesIsAllocationFree(t *testing.T) {
	var canvas ui.Canvas
	theme := ui.LightTheme()
	paint := func() {
		canvas.Reset()
		PaintInput(&canvas, theme.WithRadius(ui.RadiusFull).WithFontSize(16), ui.Rect{X: 2, Y: 3, Width: 160, Height: 40}, InputSnapshot{
			ID: ui.Item(9), Label: "Name", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3,
			SelectionStart: 1, SelectionEnd: 3, CompositionStart: 3, CompositionEnd: 6,
		})
	}
	paint()
	if allocations := testing.AllocsPerRun(100, paint); allocations != 0 {
		t.Fatalf("input paint with overrides allocated %v times", allocations)
	}
}

func BenchmarkPaintInputOverridden(b *testing.B) {
	var canvas ui.Canvas
	theme := ui.LightTheme()
	snapshot := InputSnapshot{ID: ui.Item(9), Label: "Name", Value: "Gopher", Focused: true, ShowCaret: true, Caret: 3, SelectionStart: 1, SelectionEnd: 3}
	bounds := ui.Rect{Width: 200}
	b.Run("theme", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			canvas.Reset()
			PaintInput(&canvas, theme, bounds, snapshot)
		}
	})
	b.Run("override", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			canvas.Reset()
			PaintInput(&canvas, theme.WithRadius(4).WithFontSize(16), bounds, snapshot)
		}
	})
}
