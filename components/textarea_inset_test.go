package components

import (
	"fmt"
	"strings"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestTextareaTopInsetKeepsTextAndNativeCaretBelowControls(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 1.75, 2, 3} {
		t.Run(fmt.Sprint(scale), func(t *testing.T) {
			var canvas gfx.Canvas
			canvas.SetPixelScale(scale)
			var layout TextareaLayout
			bounds := gfx.Rect{X: 8, Y: 9, Width: 260, Height: 180}
			PaintTextarea(&canvas, gfx.LightTheme(), bounds, TextareaSnapshot{
				ID: gfx.Item(1), Value: "中文输入\nsecond row", Focused: true,
				HideEditorText: true, ShowCaret: true, Layout: &layout,
				InsetTop: 32, InsetBottom: 40,
			})
			hit := canvas.Hits()[0]
			if hit.TextBounds.Y != bounds.Y+textareaPadY+32 || hit.TextBounds.Height != bounds.Height-2*textareaPadY-72 {
				t.Fatalf("native bounds = %+v", hit.TextBounds)
			}
			caret, ok := hit.Placement.CaretBounds()
			if !ok || caret.Y < hit.TextBounds.Y || caret.Y+caret.Height > hit.TextBounds.Y+hit.TextBounds.Height+1 {
				t.Fatalf("native caret = %+v, %v", caret, ok)
			}
		})
	}
}

func TestTextareaTopInsetClampsAndKeepsTheDefaultLayout(t *testing.T) {
	for _, theme := range []gfx.Theme{gfx.LightTheme(), gfx.DarkTheme()} {
		for _, inset := range []float32{-32, 0, 32, 1000} {
			var canvas gfx.Canvas
			bounds := gfx.Rect{X: 8, Y: 9, Width: 260, Height: 120}
			PaintTextarea(&canvas, theme, bounds, TextareaSnapshot{ID: gfx.Item(1), Value: "hello", InsetTop: inset})
			content := canvas.Hits()[0].TextBounds
			if content.Height < 0 || content.Y < bounds.Y+textareaPadY || content.Y+content.Height > bounds.Y+bounds.Height-textareaPadY {
				t.Fatalf("inset %g put the editor outside its surface: %+v", inset, content)
			}
			if inset <= 0 && content.Y != bounds.Y+textareaPadY {
				t.Fatal("zero or negative inset changed the existing content origin")
			}
		}
	}
}

func textareaWithTopControls(canvas *gfx.Canvas, layout *TextareaLayout, value string) {
	canvas.Reset()
	PaintTextarea(canvas, gfx.LightTheme(), gfx.Rect{Width: 360, Height: 232}, TextareaSnapshot{
		ID: gfx.Item(1), Value: value, Focused: true, HideEditorText: true, ShowCaret: true,
		Caret: 300, Layout: layout, InsetTop: 32, InsetBottom: 40,
	})
}

func TestTextareaTopControlsPaintAllocatesNothingOnceWarm(t *testing.T) {
	var canvas gfx.Canvas
	var layout TextareaLayout
	value := strings.Repeat(wrapTestText+"\n", 8)
	textareaWithTopControls(&canvas, &layout, value)
	if allocations := testing.AllocsPerRun(100, func() { textareaWithTopControls(&canvas, &layout, value) }); allocations != 0 {
		t.Fatalf("top-controls paint allocated %g times", allocations)
	}
}

func BenchmarkPaintTextareaTopControls(b *testing.B) {
	var canvas gfx.Canvas
	var layout TextareaLayout
	value := strings.Repeat(wrapTestText+"\n", 8)
	textareaWithTopControls(&canvas, &layout, value)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		textareaWithTopControls(&canvas, &layout, value)
	}
}
