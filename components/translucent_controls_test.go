package components

import (
	"testing"

	"github.com/vivaui/viva/ui"
)

func TestTranslucentInputDoesNotPaintThePageBackgroundAgain(t *testing.T) {
	theme := ui.LightTheme()
	theme.Background.Alpha = .5
	var canvas ui.Canvas
	paintTextEntrySurface(&canvas, theme, ui.Rect{Width: 200, Height: 32}, textEntryState{})
	for _, rect := range canvas.Rectangles() {
		if rect.Color.Alpha > 0 && rect.Bounds.X <= 100 && rect.Bounds.X+rect.Bounds.Width >= 100 && rect.Bounds.Y <= 16 && rect.Bounds.Y+rect.Bounds.Height >= 16 {
			t.Fatalf("transparent input repaints the page: %+v", rect)
		}
	}
}

func TestTranslucentControlsDoNotFillTheInteriorOnFocus(t *testing.T) {
	theme := ui.DarkTheme()
	theme.Background.Alpha = .04
	theme.Muted = ui.Color{Red: .5, Green: .5, Blue: .5, Alpha: .055}
	theme.Card = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: .12}
	bounds := ui.Rect{X: 10, Y: 10, Width: 200, Height: 32}
	for _, name := range []string{"input", "button", "segments"} {
		var canvas ui.Canvas
		canvas.SetInteraction(ui.ID{}, ui.ID{}, ui.Item(7), true)
		switch name {
		case "input":
			PaintInput(&canvas, theme, bounds, InputSnapshot{ID: ui.Item(7), Value: "readable text"})
		case "button":
			BeginButton(&canvas, theme, ui.Item(7), bounds).Outline("Readable button")
		case "segments":
			BeginSegmentedControl(&canvas, theme, bounds, []SegmentOption{{ID: ui.Item(7), Label: "Selected"}}, ui.Item(7)).Paint()
		}
		for _, rect := range canvas.Rectangles() {
			coversCenter := rect.Bounds.X <= 110 && rect.Bounds.X+rect.Bounds.Width >= 110 && rect.Bounds.Y <= 26 && rect.Bounds.Y+rect.Bounds.Height >= 26
			if coversCenter && rect.Color.Alpha > .25 {
				t.Fatalf("%s: focus paints over the translucent content: %+v", name, rect)
			}
		}
		if len(canvas.Triangles()) == 0 {
			t.Fatalf("%s: missing focus outlines", name)
		}
	}
}

func TestTranslucentSegmentsPaintTheRestingTrackOnce(t *testing.T) {
	for _, theme := range []ui.Theme{ui.LightTheme(), ui.DarkTheme()} {
		theme.Background.Alpha = .5
		theme.Border = ui.Color{Alpha: .16}
		theme.Muted = ui.Color{Alpha: .075}
		theme.Card = ui.Color{Red: 1, Green: 1, Blue: 1, Alpha: .76}
		var canvas ui.Canvas
		BeginSegmentedControl(&canvas, theme, ui.Rect{Width: 200, Height: 32},
			[]SegmentOption{{ID: ui.Item(1), Label: "Resting"}, {ID: ui.Item(2), Label: "Selected"}}, ui.Item(2)).Paint()
		layers := 0
		for _, rect := range canvas.Rectangles() {
			if rect.Color.Alpha > 0 && rect.Bounds.X < 50 && rect.Bounds.X+rect.Bounds.Width > 50 && rect.Bounds.Y < 16 && rect.Bounds.Y+rect.Bounds.Height > 16 {
				layers++
				if rect.Color != theme.Muted {
					t.Fatalf("unexpected resting layer: %+v", rect.Color)
				}
			}
		}
		if layers != 1 {
			t.Fatalf("resting track painted %d times, want once", layers)
		}
	}
}
