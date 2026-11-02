package ui

import "testing"

func TestVegaBluePresetTokens(t *testing.T) {
	light := LightTheme()
	if light.Background != RGB(255, 255, 255) || light.Foreground != RGB(10, 10, 10) || light.Primary != RGB(20, 71, 230) {
		t.Fatalf("light Vega tokens = %+v", light)
	}
	if light.Accent != RGB(245, 245, 245) || light.Secondary != RGB(244, 244, 245) || light.Chart[4] != RGB(25, 60, 184) {
		t.Fatalf("light semantic tokens = %+v", light)
	}
	if light.Selection.Alpha == 0 || light.SelectionForeground.Alpha == 0 || light.Selection == light.Primary {
		t.Fatalf("light selection tokens are missing or still follow the action accent: %+v", light)
	}

	dark := DarkTheme()
	if dark.Background != RGB(10, 10, 10) || dark.Foreground != RGB(250, 250, 250) || dark.Primary != RGB(25, 60, 184) {
		t.Fatalf("dark Vega tokens = %+v", dark)
	}
	if dark.Border.Alpha != 0.10 || dark.Input.Alpha != 0.15 || dark.Popover != dark.Card {
		t.Fatalf("dark translucent tokens = %+v", dark)
	}
	if dark.Selection.Alpha == 0 || dark.SelectionForeground.Alpha == 0 || dark.Selection == dark.Primary {
		t.Fatalf("dark selection tokens are missing or still follow the action accent: %+v", dark)
	}
	if metrics := DefaultMetrics(); metrics.RadiusSurface != 11 || metrics.RadiusControl != 7.5 {
		t.Fatalf("small-radius metrics = %+v", metrics)
	}
}

// Hover must read as a whisper against the page, not a highlight: just above
// the background in dark mode, just below it in light mode. It must also stay
// distinct from CardElevated, which the sidebar fills its surface with.
func TestHoverSitsCloseToBackground(t *testing.T) {
	dark := DarkTheme()
	if !colorLighter(dark.Hover, dark.Background) {
		t.Fatalf("dark hover %+v must sit just above background %+v", dark.Hover, dark.Background)
	}
	if dark.Hover == dark.CardElevated {
		t.Fatal("dark hover must stay distinct from CardElevated")
	}

	light := LightTheme()
	if !colorDarker(light.Hover, light.Background) {
		t.Fatalf("light hover %+v must sit just below background %+v", light.Hover, light.Background)
	}
	if light.Hover == light.CardElevated {
		t.Fatal("light hover must stay distinct from CardElevated")
	}
}

func colorLighter(a, b Color) bool {
	return a.Red > b.Red && a.Green > b.Green && a.Blue >= b.Blue
}

func colorDarker(a, b Color) bool {
	return a.Red < b.Red && a.Green < b.Green && a.Blue <= b.Blue
}

// Corner radii come from the metrics alone: no component clamps them, so a theme
// that asks for rounder controls gets them, and derived radii stay inside their
// parents' at every density.
func TestRadiiAreDerivedFromMetricsAtEveryDensity(t *testing.T) {
	for _, density := range []Density{DensityRegular, DensityCompact, DensityLarge} {
		m := DefaultMetrics().WithDensity(density)
		if !(m.RadiusItem() < m.RadiusControl && m.RadiusControl <= m.RadiusPopup() && m.RadiusPopup() < m.RadiusSurface) {
			t.Errorf("density %d: item %v control %v popup %v surface %v are not nested", density, m.RadiusItem(), m.RadiusControl, m.RadiusPopup(), m.RadiusSurface)
		}
	}
	if (Metrics{}).RadiusItem() != 0 || (Metrics{}).RadiusPopup() != 0 {
		t.Error("zero metrics must give square corners, not negative radii")
	}
}

func TestRGB24ReadsTheChannelsOfAHexNumber(t *testing.T) {
	if got, want := RGB24(0x0a84ff), RGB(0x0a, 0x84, 0xff); got != want {
		t.Fatalf("RGB24(0x0a84ff) = %+v, want %+v", got, want)
	}
	if got := RGB24(0x171719).WithAlpha(0.5); got.Alpha != 0.5 || got.Red != RGB(0x17, 0x17, 0x19).Red {
		t.Fatalf("WithAlpha changed the channels: %+v", got)
	}
}

// A control painted with its own radius or font size must not change the theme
// the rest of the window is painted with: the override is a copy.
func TestThemeOverridesChangeOnlyTheCopy(t *testing.T) {
	base := LightTheme()
	custom := base.WithRadius(3).WithFontSize(21).WithControlHeight(44)
	if base.Metrics != DefaultMetrics() {
		t.Fatalf("overriding changed the original theme: %+v", base.Metrics)
	}
	if custom.Metrics.RadiusControl != 3 || custom.Metrics.FontSizeM != 21 || custom.Metrics.ControlHeight != 44 {
		t.Fatalf("overrides not applied: %+v", custom.Metrics)
	}
	if custom.Metrics.RadiusSurface != base.Metrics.RadiusSurface {
		t.Fatalf("surface radius changed with the control radius: %v", custom.Metrics.RadiusSurface)
	}
	if custom.Metrics.RadiusItem() != 1 {
		t.Fatalf("menu rows do not follow the control radius: %v", custom.Metrics.RadiusItem())
	}
	// The small and large steps keep their ratio to the body size.
	if got, want := custom.Metrics.FontSizeS/custom.Metrics.FontSizeM, base.Metrics.FontSizeS/base.Metrics.FontSizeM; got != want {
		t.Fatalf("small/body ratio = %v, want %v", got, want)
	}
	if got, want := custom.Metrics.FontSizeL/custom.Metrics.FontSizeM, base.Metrics.FontSizeL/base.Metrics.FontSizeM; got != want {
		t.Fatalf("large/body ratio = %v, want %v", got, want)
	}
}

func TestThemeOverridesIgnoreNonsense(t *testing.T) {
	base := DarkTheme()
	if got := base.WithFontSize(0).Metrics; got != base.Metrics {
		t.Fatalf("a zero font size changed the metrics: %+v", got)
	}
	if got := base.WithFontSize(-4).Metrics; got != base.Metrics {
		t.Fatalf("a negative font size changed the metrics: %+v", got)
	}
	if got := base.WithRadius(-5).Metrics.RadiusControl; got != 0 {
		t.Fatalf("a negative radius = %v, want 0", got)
	}
	if got := base.WithControlHeight(-1).Metrics.ControlHeight; got != 0 {
		t.Fatalf("a negative control height = %v, want 0", got)
	}
	var empty Theme
	if got := empty.WithFontSize(14).Metrics; got.FontSizeM != 0 {
		t.Fatalf("a theme without metrics gained a font size: %+v", got)
	}
}

// The text of a text-entry control starts where the 1px border and the
// control's own padding leave it, at every density.
func TestTextInsetFollowsControlPadding(t *testing.T) {
	m := DefaultMetrics()
	if got := m.TextInsetX(); got != 11 {
		t.Fatalf("default inset = %v, want 11 (Vega px-2.5 plus the border)", got)
	}
	if compact, large := m.WithDensity(DensityCompact).TextInsetX(), m.WithDensity(DensityLarge).TextInsetX(); !(compact < 11 && 11 < large) {
		t.Fatalf("inset does not follow density: compact %v, large %v", compact, large)
	}
}
