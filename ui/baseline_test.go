package ui

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A Button's label is drawn on the same baseline row as a Text, so Baseline
// alignment can sit a control and a label on one line.
func TestButtonBaselineMatchesItsLabelRow(t *testing.T) {
	theme := gfx.LightTheme()
	cases := []struct {
		name      string
		declare   func(*frame) Box
		labelSize float32
	}{
		{"default", func(f *Frame) Box { return f.Button(save, "Save") }, theme.Metrics.FontSizeM},
		{"xsmall", func(f *Frame) Box { return f.Button(save, "Save").XSmall() }, theme.Metrics.FontSizeS},
		{"font size", func(f *Frame) Box { return f.Button(save, "Save").FontSize(20) }, 20},
		{"height", func(f *Frame) Box { return f.Button(save, "Save").Height(48) }, theme.Metrics.FontSizeM},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var view View
			var text, button Box
			canvas := testCanvas()
			frame := view.Begin(canvas, theme, gfx.Rect{Width: 400, Height: 80})
			frame.Row().Height(80).Items(Baseline).Children(func() {
				text = frame.Text("Text").FontSize(16)
				button = tc.declare(frame)
			})
			frame.End()

			got := canvas.LabelBaseline(button.Rect(), gfx.TextStyle{Size: tc.labelSize})
			want := canvas.LabelBaseline(text.Rect(), gfx.TextStyle{Size: 16})
			if d := got - want; d > 0.01 || d < -0.01 {
				t.Fatalf("button baseline %v, text baseline %v: differ by %v", got, want, d)
			}
		})
	}
}

// A Mono text stands on its own font's baseline and a proportional one on
// LabelBaseline's row; Baseline alignment lines the two rows up anyway, so a
// code line and the prose beside it share one baseline.
func TestBaselineAlignmentMixesMonoAndProportionalText(t *testing.T) {
	canvas := testCanvas()
	canvas.SetTextEngine(cellEngine{})
	var view View
	var code, prose Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80})
	f.Row().Height(80).Items(Baseline).Children(func() {
		code = f.Text("x1").Mono().FontSize(20)
		prose = f.Text("a label").FontSize(20)
	})
	f.End()

	monoRow := canvas.MonoBaseline(20, gfx.MonoSystem, code.Rect().Height)
	if monoRow <= 0 {
		t.Fatalf("the engine measured no row of its own (%v)", monoRow)
	}
	got := code.Rect().Y + monoRow
	want := canvas.LabelBaseline(prose.Rect(), gfx.TextStyle{Size: 20})
	if d := got - want; d > 0.01 || d < -0.01 {
		t.Errorf("the mono line stands at %v, the prose at %v: rows differ by %v", got, want, d)
	}
}

// cellEngine is a TextEngine whose fixed-pitch face is a font like gfx's test
// one: 0.55 of the size wide, a 1.13 line box with its baseline 0.8 down. Its
// proportional widths are the measurer testCanvas installs, so the engine changes
// only the mono cell.
type cellEngine struct{}

func (cellEngine) Width(value string, size, weight float32) float32 {
	return float32(len(value)) * size / 2
}

func (cellEngine) Caret(value string, size, weight float32, offset int) float32 {
	return float32(min(max(offset, 0), len(value))) * size / 2
}

func (cellEngine) Selection(dst []gfx.TextSpan, value string, size, weight float32, start, end int) []gfx.TextSpan {
	start, end = min(max(start, 0), len(value)), min(max(end, 0), len(value))
	if start >= end {
		return dst
	}
	return append(dst, gfx.TextSpan{X: float32(start) * size / 2, Width: float32(end-start) * size / 2})
}

func (cellEngine) OffsetAt(value string, size, weight, x float32) int {
	return min(max(int(x/(size/2)+0.5), 0), len(value))
}

func (cellEngine) MonoCell(size float32, face gfx.MonoFace) gfx.MonoCellMetrics {
	return gfx.MonoCellMetrics{Advance: 0.55 * size, Ascent: 0.80 * size, Descent: 0.25 * size, LineGap: 0.08 * size}
}
