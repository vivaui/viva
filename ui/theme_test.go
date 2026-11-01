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
