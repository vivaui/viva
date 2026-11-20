package base

import (
	"testing"

	"github.com/vivaui/viva/gfx"
)

// A Button's label is drawn on the same baseline row as a Text, so Baseline
// alignment can sit a control and a label on one line.
func TestButtonBaselineMatchesItsLabelRow(t *testing.T) {
	theme := gfx.LightTheme()
	cases := []struct {
		Name      string
		declare   func(*Frame) Box
		labelSize float32
	}{
		{"default", func(f *Frame) Box { return f.Button(save, "Save") }, theme.Metrics.FontSizeM},
		{"xsmall", func(f *Frame) Box { return f.Button(save, "Save").XSmall() }, theme.Metrics.FontSizeS},
		{"font size", func(f *Frame) Box { return f.Button(save, "Save").FontSize(20) }, 20},
		{"height", func(f *Frame) Box { return f.Button(save, "Save").Height(48) }, theme.Metrics.FontSizeM},
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
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

// Caller-owned button content keeps its actual text baseline when a row asks
// for baseline alignment, rather than using the bottom edge of the button.
func TestButtonChildrenBaselineMatchesItsText(t *testing.T) {
	canvas := testCanvas()
	var view View
	var sibling, label Box
	f := view.Begin(canvas, gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80})
	f.Row().Height(80).Items(Baseline).Children(func() {
		sibling = f.Text("Text").FontSize(20)
		f.Button(save, "Save").FontSize(20).Children(func() {
			label = f.Text("Save")
		})
	})
	f.End()
	got := canvas.LabelBaseline(label.Rect(), gfx.TextStyle{Size: 20})
	want := canvas.LabelBaseline(sibling.Rect(), gfx.TextStyle{Size: 20})
	if d := got - want; d > 0.01 || d < -0.01 {
		t.Fatalf("caller-owned button text baseline %v, sibling baseline %v", got, want)
	}
}

func TestDisabledButtonCannotAutofocusDescendants(t *testing.T) {
	var view View
	f := view.Begin(testCanvas(), gfx.LightTheme(), gfx.Rect{Width: 400, Height: 80})
	blocked, available := gfx.Item(101), gfx.Item(102)
	row := f.Row().Children(func() {
		f.Button(save, "Disabled").Disabled(true).Children(func() {
			f.Button(blocked, "Nested")
		})
		f.Button(available, "Available")
	})
	if got := f.FirstControl(row.i); got != available {
		t.Fatalf("dialog autofocus candidate = %v, want enabled sibling %v", got, available)
	}
	f.End()
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

func (cellEngine) Width(Value string, Size, Weight float32) float32 {
	return float32(len(Value)) * Size / 2
}

func (cellEngine) Caret(Value string, Size, Weight float32, offset int) float32 {
	return float32(min(max(offset, 0), len(Value))) * Size / 2
}

func (cellEngine) Selection(dst []gfx.TextSpan, Value string, Size, Weight float32, start, end int) []gfx.TextSpan {
	start, end = min(max(start, 0), len(Value)), min(max(end, 0), len(Value))
	if start >= end {
		return dst
	}
	return append(dst, gfx.TextSpan{X: float32(start) * Size / 2, Width: float32(end-start) * Size / 2})
}

func (cellEngine) OffsetAt(Value string, Size, Weight, x float32) int {
	return min(max(int(x/(Size/2)+0.5), 0), len(Value))
}

func (cellEngine) MonoCell(Size float32, Face gfx.MonoFace) gfx.MonoCellMetrics {
	return gfx.MonoCellMetrics{Advance: 0.55 * Size, Ascent: 0.80 * Size, Descent: 0.25 * Size, LineGap: 0.08 * Size}
}
