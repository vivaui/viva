//go:build !android

package linux

import (
	"os"
	"testing"

	"github.com/vivaui/viva/internal/platform"
)

func TestTheLabelsOnScreenAreDrawnAgainWhenTheDesktopSwitchesToLCDText(t *testing.T) {
	resetDesktopState(t)
	environment := os.Getenv
	gray := settingsOf(t, map[string]int32{"Xft/Antialias": 1}, map[string]string{"Xft/RGBA": "none", "Xft/HintStyle": "hintslight"})
	publishSettings(gray, "", environment)

	offscreen, err := NewOffscreen(160, 28, 1, platform.Color{Red: 1, Green: 1, Blue: 1, Alpha: 1})
	if err != nil {
		t.Skipf("no Vulkan device: %v", err)
	}
	t.Cleanup(offscreen.Close)
	label := platform.TextLabel{Value: "Hamburgefonstiv", Width: 150, Height: 28, FontSize: 14, Color: platform.Color{Red: 0.1, Green: 0.1, Blue: 0.1, Alpha: 1}}
	if colored, _ := coloredInk(renderScene(t, offscreen, nil, []platform.TextLabel{label})); colored != 0 {
		t.Fatalf("grayscale text has %d colored pixels", colored)
	}

	lcd := settingsOf(t, map[string]int32{"Xft/Antialias": 1}, map[string]string{"Xft/RGBA": "rgb", "Xft/HintStyle": "hintslight"})
	if change := publishSettings(lcd, "", environment); change != changedFonts {
		t.Fatalf("the switch reported %v, want the fonts", change)
	}
	refreshTextEngine()
	if err := offscreen.RefreshText(); err != nil {
		t.Fatal(err)
	}
	image, err := offscreen.Image()
	if err != nil {
		t.Fatal(err)
	}
	if colored, darkest := coloredInk(image); colored < 20 || darkest > 80 {
		t.Fatalf("after the switch: %d colored pixels, darkest %d; the label was not drawn again with LCD text", colored, darkest)
	}
}

func TestANewInterfaceFontReshapesTextTheEngineHadMeasured(t *testing.T) {
	resetDesktopState(t)
	engine := sharedTextEngine()
	if engine == nil {
		t.Skip("no Pango on this machine")
	}
	text := "Hamburgefonstiv 0123456789"
	proportional := settingsOf(t, nil, map[string]string{"Gtk/FontName": "Sans 10"})
	publishSettings(proportional, "", os.Getenv)
	refreshTextEngine()
	before := engine.Width(text, 14, 0)
	mono := settingsOf(t, nil, map[string]string{"Gtk/FontName": "Monospace 10"})
	if change := publishSettings(mono, "", os.Getenv); change != changedFonts {
		t.Fatalf("the new font reported %v", change)
	}
	if stale := engine.Width(text, 14, 0); stale != before {
		t.Fatalf("the engine changed its answer before it was told: %v then %v", before, stale)
	}
	refreshTextEngine()
	after := engine.Width(text, 14, 0)
	if after == before {
		t.Fatalf("the width is %v with both fonts: the cached line was kept", before)
	}
	t.Logf("%q is %.1f wide in Sans and %.1f in Monospace", text, before, after)
}
