package components

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestProgressIndicatorIsClippedToItsFractionAndNeverLeavesTheTrack(t *testing.T) {
	theme := gfx.LightTheme()
	track := gfx.Rect{X: 10, Y: 20, Width: 200, Height: 6}
	for _, fraction := range []float32{0, 0.01, 0.5, 1, 1.7, -1} {
		var canvas gfx.Canvas
		Progress(&canvas, theme, track, fraction)
		clamped := min(max(fraction, 0), 1)
		rectangles := canvas.Rectangles()
		if clamped == 0 {
			if len(rectangles) != 1 || rectangles[0].Color != theme.Muted {
				t.Fatalf("fraction %v painted %d rectangles, want only the track", fraction, len(rectangles))
			}
			continue
		}
		indicator := rectangles[len(rectangles)-1]
		if indicator.Color != theme.Primary || !indicator.HasClip {
			t.Fatalf("fraction %v indicator = %+v", fraction, indicator)
		}
		if got, want := indicator.ClipOf.Width, 200*clamped; got < want-0.01 || got > want+0.01 {
			t.Fatalf("fraction %v clips the indicator to %v, want %v", fraction, got, want)
		}
	}
}

func TestProgressIndeterminateSlidesInsideTheTrack(t *testing.T) {
	theme := gfx.LightTheme()
	track := gfx.Rect{Width: 100, Height: 6}
	for _, phase := range []float32{0, 0.3, 0.7, 1} {
		var canvas gfx.Canvas
		ProgressIndeterminate(&canvas, theme, track, phase)
		for _, rectangle := range canvas.Rectangles() {
			if rectangle.HasClip && (rectangle.ClipOf.X < 0 || rectangle.ClipOf.X+rectangle.ClipOf.Width > 100.01) {
				t.Fatalf("phase %v segment %+v leaves the track", phase, rectangle.ClipOf)
			}
		}
	}
	var middle gfx.Canvas
	ProgressIndeterminate(&middle, theme, track, 0.5)
	if len(middle.Rectangles()) != 2 {
		t.Fatalf("mid-cycle segment missing: %d rectangles", len(middle.Rectangles()))
	}
}

func TestProgressRowPutsLabelAndValueOnALineAboveTheTrack(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	track := PaintProgressRow(&canvas, theme, gfx.Rect{X: 4, Y: 8, Width: 240}, "Uploading", "64%", 0.64)
	texts := canvas.Texts()
	if len(texts) != 2 || texts[0].Value != "Uploading" || texts[1].Value != "64%" {
		t.Fatalf("row texts = %+v", texts)
	}
	if track.Y <= texts[0].Bounds.Y+texts[0].Bounds.Height-1 || track.Width != 240 || track.Y+track.Height != 8+ProgressRowHeight(theme) {
		t.Fatalf("track %+v not under the text line", track)
	}
}
