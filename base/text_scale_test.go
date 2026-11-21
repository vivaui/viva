package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

func TestScaledAndAbsoluteTextShareOneLayout(t *testing.T) {
	theme := gfx.LightTheme().WithTextScale(1.5)
	canvas := testCanvas()
	var view View
	var heading Box
	view.Draw(canvas, theme, gfx.Rect{Width: 300, Height: 180}, func(f *Frame) {
		f.Column().Children(func() {
			heading = f.Text("Heading").FontSize(theme.Metrics.ScaledText(20)).LineHeight(theme.Metrics.ScaledText(28))
			f.Text("Token").FontSize(theme.Metrics.FontSizeM)
			f.Text("Fixed").FontSize(12)
		})
	})
	if got := heading.Rect().Height; got != 42 {
		t.Fatalf("heading line height = %v, want 42", got)
	}
	labels := canvas.Texts()
	want := []float32{30, 21, 12}
	if len(labels) != len(want) {
		t.Fatalf("drew %d labels, want %d", len(labels), len(want))
	}
	for i, l := range labels {
		if l.Style.Size != want[i] {
			t.Errorf("%s font size = %v, want %v", l.Value, l.Style.Size, want[i])
		}
	}
}
