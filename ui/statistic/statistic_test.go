package statistic_test

import (
	"math"
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/statistic"
)

func TestFormat(t *testing.T) {
	for _, c := range []struct {
		v    float64
		prec int
		sep  rune
		want string
	}{
		{0, 0, 0, "0"}, {1234567, 0, ',', "1,234,567"}, {999, 0, ',', "999"}, {1000, 0, ',', "1,000"},
		{-1234.5, 1, ',', "-1,234.5"}, {3.14159, 2, 0, "3.14"}, {2, 2, 0, "2.00"}, {-0.001, 2, 0, "0.00"},
		{12345.678, 1, '.', "12.345.7"}, {math.NaN(), 0, 0, "—"}, {math.Inf(1), 0, 0, "—"}, {-4, -3, 0, "-4"},
	} {
		if got := statistic.Format(c.v, c.prec, c.sep); got != c.want {
			t.Errorf("Format(%v, %d, %q) = %q, want %q", c.v, c.prec, c.sep, got, c.want)
		}
	}
}

func TestTextAddsPrefixAndSuffixAndKeepsItWhileUnchanged(t *testing.T) {
	s := statistic.New()
	spec := statistic.Spec{Value: 1234.5, Precision: 1, Group: true, Prefix: "$", Suffix: " ms"}
	if got := s.Text(spec); got != "$1,234.5 ms" {
		t.Fatalf("Text = %q", got)
	}
	nan := statistic.Spec{Value: math.NaN()}
	s.Text(nan)
	if n := testing.AllocsPerRun(100, func() { s.Text(nan) }); n != 0 {
		t.Fatalf("an unchanged NaN reading allocates %v times", n)
	}
}

func TestWarmPaintDoesNotAllocate(t *testing.T) {
	s := statistic.New()
	view := func(c ui.C) {
		ui.Column(c).Fill().Pad(16).Children(func() {
			s.View(c, statistic.Spec{Title: "Sessions", Value: 12840, Group: true, Status: ui.StatusSuccess})
		})
	}
	theme := gfx.LightTheme()
	page := ui.NewPage(ui.PageConfig{View: view, Theme: &theme})
	canvas := &gfx.Canvas{}
	canvas.SetReduceMotion(true)
	draw := func() {
		canvas.BeginFrame()
		canvas.Reset()
		page.Draw(canvas, gfx.Rect{Width: 320, Height: 200}, view)
		canvas.EndFrame()
	}
	draw()
	draw()
	if n := testing.AllocsPerRun(100, draw); n != 0 {
		t.Fatalf("a warm paint allocates %v times", n)
	}
}

func BenchmarkFormat(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		statistic.Format(1234567.891, 2, ',')
	}
}
