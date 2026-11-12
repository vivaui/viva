package gfx

import (
	"math"
	"testing"
)

// The canvas uses the row the rasterizers use, so layout can line text up with
// what is drawn: half the line box plus 0.378 of the font size, on a device row.
func TestCanvasLabelBaselineMatchesTheBackendRow(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	for size, want := range map[float32]float32{
		11: 13, 12: 14, 13: 14, 14: 15, 16: 17, 18: 19, 20: 21,
	} {
		if got := canvas.LabelBaseline(Rect{Height: size + 7}, TextStyle{Size: size}); got != want {
			t.Errorf("size %v: baseline row %v, want %v", size, got, want)
		}
	}

	canvas.SetPixelScale(2)
	if got := canvas.LabelBaseline(Rect{Height: 27}, TextStyle{Size: 20}); got != 21 {
		t.Errorf("size 20 at 2x: baseline %v, want 21", got)
	}
	if got := canvas.LabelBaseline(Rect{Y: 8, Height: 27}, TextStyle{Size: 20}); got != 29 {
		t.Errorf("bounds at y 8: baseline %v, want 29", got)
	}

	canvas.SetPixelScale(1.25)
	got := canvas.LabelBaseline(Rect{Height: 23}, TextStyle{Size: 16})
	if want := float32(22) / 1.25; math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("size 16 at 1.25x: baseline %v, want %v", got, want)
	}
}

func TestLabelOnBaselinePutsTheBaselineAtTheGivenRow(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(2)
	canvas.LabelOnBaseline("Big", Rect{X: 10, Width: 100, Height: 31}, 100, TextStyle{Size: 24})
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %+v", texts)
	}
	bounds := texts[0].Bounds
	if bounds.X != 10 || bounds.Height != 31 {
		t.Errorf("bounds = %+v, want x 10 and height 31", bounds)
	}
	if bounds.Y != 75.5 {
		t.Errorf("text top = %v, want 75.5", bounds.Y)
	}
	if got := canvas.LabelBaseline(bounds, TextStyle{Size: 24}); got != 100 {
		t.Errorf("baseline of the drawn line = %v, want 100", got)
	}
}

func TestLabelAlignedOnBaselineKeepsHorizontalPlacement(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	bounds := Rect{X: 100, Width: 100, Height: 14}
	canvas.LabelAlignedOnBaseline("x", bounds, 20, TextStyle{Size: 14}, AlignCenter)
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %+v", texts)
	}
	if got := texts[0].Bounds.X; got <= 100 {
		t.Errorf("centered line x = %v, want right of the box start", got)
	}
	if got := canvas.LabelBaseline(texts[0].Bounds, TextStyle{Size: 14}); got != 20 {
		t.Errorf("baseline = %v, want 20", got)
	}
}

// A Mono line placed on a baseline sits on its own font's row (MonoBaseline), not
// the label row, so a code line and the interface text around it share a baseline.
func TestLabelOnBaselineUsesTheMonoFontsOwnRow(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	canvas.SetTextEngine(&monoCellEngine{MonoCellMetrics: MonoCellMetrics{Advance: 0.55, Ascent: 0.80, Descent: 0.25, LineGap: 0.08}})
	bounds := Rect{X: 5, Width: 100, Height: 23}
	canvas.LabelOnBaseline("hi", bounds, 40, TextStyle{Size: 20, Mono: true})
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %+v", texts)
	}
	if got := texts[0].Bounds.Y + texts[0].Baseline; got != 40 {
		t.Errorf("a mono line on baseline 40 stands at %v, want 40", got)
	}
	if texts[0].Bounds.Y != 40-17 {
		t.Errorf("box top = %v, want %v: the cell's own baseline row 17", texts[0].Bounds.Y, 40-17)
	}
}

// A canvas that could not measure the face has no row of its own for it: the pair
// places the line from the label row, the closest guess, and the renderer centers
// the line by the document's own metrics.
func TestLabelOnBaselineWithoutAnEngineUsesTheLabelRow(t *testing.T) {
	var canvas Canvas
	canvas.SetPixelScale(1)
	bounds := Rect{X: 5, Width: 100, Height: 20}
	canvas.LabelOnBaseline("hi", bounds, 33, TextStyle{Size: 16, Mono: true})
	texts := canvas.Texts()
	if len(texts) != 1 {
		t.Fatalf("texts = %+v", texts)
	}
	if texts[0].Baseline != 0 {
		t.Errorf("baseline = %v, want none: no engine measured the face", texts[0].Baseline)
	}
	if want := 33 - canvas.LabelBaseline(bounds, TextStyle{Size: 16}); texts[0].Bounds.Y != want {
		t.Errorf("box top = %v, want %v from the label row", texts[0].Bounds.Y, want)
	}
}
