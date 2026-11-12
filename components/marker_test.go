package components

import (
	"slices"
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestMarkerMirrorsDefaultSeparatorAndBorderVariants(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	Marker(&canvas, theme, gfx.Rect{Width: 240, Height: 24}, MarkerDefault, &checkIcon, "Default")
	Marker(&canvas, theme, gfx.Rect{Y: 28, Width: 240, Height: 24}, MarkerSeparator, nil, "Today")
	Marker(&canvas, theme, gfx.Rect{Y: 56, Width: 240, Height: 24}, MarkerBorder, nil, "Archived")
	if len(canvas.Rectangles()) != 3 {
		t.Fatalf("marker rules = %+v", canvas.Rectangles())
	}
	if canvas.Rectangles()[2].Bounds.Y != 79 {
		t.Fatalf("border marker y = %f", canvas.Rectangles()[2].Bounds.Y)
	}
	for _, text := range canvas.Texts() {
		if text.Style.Color != theme.MutedForeground {
			t.Fatalf("marker color = %+v", text.Style.Color)
		}
	}
}

func TestMarkerCommandStreamIsStable(t *testing.T) {
	var canvas gfx.Canvas
	paint := func() {
		canvas.Reset()
		Marker(&canvas, gfx.DarkTheme(), gfx.Rect{Width: 200}, MarkerSeparator, nil, "Processing")
	}
	paint()
	rectangles := slices.Clone(canvas.Rectangles())
	texts := slices.Clone(canvas.Texts())
	paint()
	if !slices.Equal(rectangles, canvas.Rectangles()) || !slices.Equal(texts, canvas.Texts()) {
		t.Fatal("identical markers produced different command streams")
	}
}

func TestMarkerProgressPaintsASpinnerInTheIconSlot(t *testing.T) {
	theme := gfx.LightTheme()
	var canvas gfx.Canvas
	MarkerProgress(&canvas, theme, gfx.Rect{Width: 240}, MarkerDefault, 2, "Thinking")
	if len(canvas.Rectangles()) == 0 {
		t.Fatal("no spinner spokes")
	}
	spoke, text := canvas.Rectangles()[0], canvas.Texts()[0]
	if spoke.Bounds.X+spoke.Bounds.Width > text.Bounds.X {
		t.Fatalf("the spinner %+v runs into the text %+v", spoke.Bounds, text.Bounds)
	}
}

func TestMarkerWithoutAHeightIsAsTallAsItsText(t *testing.T) {
	theme := gfx.LightTheme()
	var plain, bordered gfx.Canvas
	Marker(&plain, theme, gfx.Rect{Width: 240}, MarkerDefault, nil, "Today")
	Marker(&bordered, theme, gfx.Rect{Width: 240}, MarkerBorder, nil, "Today")
	if got := plain.Texts()[0].Bounds.Height; got >= theme.Metrics.ControlHeight {
		t.Fatalf("default marker row is %v tall, as tall as a control", got)
	}
	rule := bordered.Rectangles()[0].Bounds
	if rule.Y <= plain.Texts()[0].Bounds.Height {
		t.Fatalf("the border rule at %v leaves no padding under the text", rule.Y)
	}
}
