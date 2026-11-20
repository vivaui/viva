package base

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestHighlightUsesUTF8ByteOffsetsAndPreservesClip(t *testing.T) {
	var view View
	canvas := testCanvas()
	tint := gfx.RGB(220, 190, 80)
	value := "上个月的设计资料 DESIGN"
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, func(f *Frame) {
		f.Column().Clip().Width(350).Children(func() {
			f.Text(value).FontSize(14).Highlight("设计", tint)
			f.Text("following label").FontSize(14)
		})
	})
	var label gfx.Text
	for _, text := range canvas.Texts() {
		if text.Value == value {
			label = text
			break
		}
	}
	spans := canvas.TextSelectionStyle(nil, value, label.Style, len("上个月的"), len("上个月的设计"))
	found := false
	for _, rect := range canvas.Rectangles() {
		if rect.Color == tint {
			found = true
			if math.Abs(float64(rect.Bounds.X-label.Bounds.X-spans[0].X)) > .01 || math.Abs(float64(rect.Bounds.Width-spans[0].Width)) > .01 {
				t.Fatalf("highlight is not on the matching Chinese text: %+v", rect.Bounds)
			}
		}
	}
	if !found {
		t.Fatal("missing highlight")
	}
	for _, text := range canvas.Texts() {
		if text.Value == "following label" && text.HasClip && text.ClipOf.Width < 300 {
			t.Fatal("highlight leaked a clip into the next label")
		}
	}
}

func TestHighlightCanBeReplacedRemovedAndDoesNotSurviveTheNextFrame(t *testing.T) {
	var view View
	canvas := testCanvas()
	tint := gfx.RGB(220, 190, 80)
	var box Box
	draw := func(enabled bool) {
		canvas.Reset()
		view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, func(f *Frame) {
			box = f.Text("Design DESIGN")
			if enabled {
				box.Highlight("absent", tint).Highlight("design", tint)
			}
		})
	}
	draw(true)
	if len(canvas.Rectangles()) != 2 {
		t.Fatal("replacement query did not highlight both matches")
	}
	draw(false)
	if len(canvas.Rectangles()) != 0 {
		t.Fatal("query leaked into the next frame")
	}
	canvas.Reset()
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, func(f *Frame) {
		f.Text("design").Highlight("design", tint).Highlight("", tint)
		f.Text("design").Highlight("design", tint).Highlight("design", Color{})
	})
	if len(canvas.Rectangles()) != 0 {
		t.Fatal("empty query or transparent color did not remove highlighting")
	}
	defer func() {
		if recover() == nil {
			t.Fatal("Highlight accepted a stale Box")
		}
	}()
	box.Highlight("design", tint)
}

func TestHighlightParagraphKeepsWrappingAndInheritedOpacity(t *testing.T) {
	var highlighted, plain View
	canvas, reference := runeCanvas(), runeCanvas()
	tint := gfx.RGB(220, 190, 80)
	draw := func(view *View, surface *gfx.Canvas, enabled bool) {
		view.Draw(surface, gfx.LightTheme(), gfx.Rect{Width: 100, Height: 200}, func(f *Frame) {
			f.Column().Opacity(.5).Children(func() {
				box := f.Wrap("design one design two").FontSize(10)
				if enabled {
					box.Highlight("design", tint)
				}
			})
		})
	}
	draw(&highlighted, canvas, true)
	draw(&plain, reference, false)
	if len(canvas.Texts()) != len(reference.Texts()) {
		t.Fatal("highlight changed wrapping")
	}
	for i, text := range canvas.Texts() {
		want := reference.Texts()[i]
		if text.Value != want.Value || text.Bounds != want.Bounds || text.Style != want.Style {
			t.Fatal("highlight changed paragraph layout or text opacity")
		}
	}
	matches := 0
	for _, rect := range canvas.Rectangles() {
		if rect.Color == fade(tint, .5) {
			matches++
		}
	}
	if matches != 2 {
		t.Fatalf("faded highlights = %d, want 2", matches)
	}
}

func BenchmarkTextHighlights(b *testing.B) {
	for _, query := range []string{"", "设计", "design", "missing"} {
		name := query
		if name == "" {
			name = "disabled"
		}
		b.Run(name, func(b *testing.B) {
			var view View
			canvas := runeCanvas()
			theme := gfx.LightTheme()
			tint := gfx.RGB(220, 190, 80)
			draw := func() {
				canvas.Reset()
				view.Draw(canvas, theme, gfx.Rect{Width: 800, Height: 800}, func(f *Frame) {
					f.Column().Children(func() {
						for range 20 {
							f.Text("项目设计资料 Design DESIGN").Highlight(query, tint)
						}
					})
				})
			}
			draw()
			b.ReportAllocs()
			for b.Loop() {
				draw()
			}
		})
	}
}

func TestHighlightCaseFoldPreservesTheOriginalText(t *testing.T) {
	var view View
	canvas := testCanvas()
	tint := gfx.RGB(220, 190, 80)
	view.Draw(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80}, func(f *Frame) { f.Text("Design DESIGN").Highlight("design", tint) })
	matches := 0
	for _, rect := range canvas.Rectangles() {
		if rect.Color == tint {
			matches++
		}
	}
	if matches != 2 {
		t.Fatalf("case-insensitive highlights = %d, want 2", matches)
	}
	if canvas.Texts()[0].Value != "Design DESIGN" {
		t.Fatal("highlight changed the original label")
	}
}
