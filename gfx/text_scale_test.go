package gfx

import (
	"math"
	"testing"
)

func TestThemeTextScale(t *testing.T) {
	base := LightTheme().WithFontSize(16)
	scaled := base.WithTextScale(1.5)
	want := base.WithFontSize(24).Metrics
	want.textScale = 1.5
	if scaled.Metrics != want || scaled.Metrics.ScaledText(20) != 30 || scaled.Metrics.ScaledText(28) != 42 {
		t.Fatalf("scaled metrics = %+v", scaled.Metrics)
	}
	if scaled.WithTextScale(1.5) != scaled || scaled.WithTextScale(1) != base {
		t.Fatal("reapplying or resetting the preference changed the base theme")
	}
	for _, invalid := range []float32{0, -1, float32(math.Inf(1)), float32(math.NaN())} {
		if scaled.WithTextScale(invalid) != scaled {
			t.Errorf("invalid scale %v changed the theme", invalid)
		}
	}
	if got := base.WithTextScale(math.MaxFloat32); got != base {
		t.Fatal("a scale that overflows a font size changed the theme")
	}
	defaultTheme := LightTheme()
	tiny := defaultTheme.WithTextScale(math.SmallestNonzeroFloat32)
	if tiny.Metrics.ScaledText(1) != math.SmallestNonzeroFloat32 || tiny.WithTextScale(1) != defaultTheme {
		t.Fatal("a representable tiny font scale could not be reset")
	}
	custom := defaultTheme
	custom.Metrics.FontSizeS = 0.25
	if got := custom.WithTextScale(math.SmallestNonzeroFloat32); got != custom {
		t.Fatal("a scale that rounds a nonzero font size to zero changed the theme")
	}
	for _, density := range []Density{DensityCompact, DensityLarge} {
		first := base
		first.Metrics = first.Metrics.WithDensity(density)
		if got := scaled.Metrics.WithDensity(density); got != first.WithTextScale(1.5).Metrics {
			t.Errorf("scale and density applied in a different order: %+v", got)
		}
	}
}
