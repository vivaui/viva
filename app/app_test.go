package app

import (
	"testing"
)

func TestConfigNormalizeDefaults(t *testing.T) {
	config, err := (Config{}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if config.Title != defaultTitle || config.Width != defaultWidth || config.Height != defaultHeight {
		t.Fatalf("unexpected defaults: %+v", config)
	}
	if config.ClearColor.Alpha != 1 {
		t.Fatalf("default color is not opaque: %+v", config.ClearColor)
	}
	if config.WindowMode != WindowTitled {
		t.Fatalf("default window mode = %d", config.WindowMode)
	}
}

func TestWindowModeValuesStayStable(t *testing.T) {
	if WindowTitled != 0 || WindowFrameless != 1 || WindowCustomTitlebar != 2 {
		t.Fatal("window mode constants must stay stable for the native ABI")
	}
}

func TestConfigNormalizeRejectsInvalidValues(t *testing.T) {
	tests := []Config{
		{Width: -1},
		{Height: -1},
		{MinWidth: -1},
		{MinHeight: -1},
		{ClearColor: Color{Red: 2, Alpha: 1}},
	}
	for _, config := range tests {
		if _, err := config.normalize(); err == nil {
			t.Fatalf("normalize accepted invalid config: %+v", config)
		}
	}
}

func TestConfigRestoresATinyWindowAtItsMinimum(t *testing.T) {
	config, err := (Config{Width: 160, Height: 90, MinWidth: 960, MinHeight: 600}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if config.Width != 960 || config.Height != 600 {
		t.Fatalf("restored window = %dx%d", config.Width, config.Height)
	}
	config, err = (Config{Width: 1200, Height: 800, MinWidth: 960}).normalize()
	if err != nil || config.Width != 1200 || config.Height != 800 || config.MinHeight != 0 {
		t.Fatalf("larger/unrestricted axes changed: %+v, %v", config, err)
	}
}

func TestSystemIntegrationValidation(t *testing.T) {
	if err := SetClipboardText("bad\x00text"); err == nil {
		t.Fatal("clipboard accepted NUL")
	}
	if err := OpenURL("file:///tmp/example"); err == nil {
		t.Fatal("OpenURL accepted file scheme")
	}
	if _, err := OpenFiles(OpenFileOptions{Filters: []FileFilter{{Name: "Text"}}}); err == nil {
		t.Fatal("OpenFiles accepted incomplete filter")
	}
	if err := SendNotification(Notification{}); err == nil {
		t.Fatal("SendNotification accepted empty notification")
	}
}

func TestTransparentWindowKeepsAZeroClearColorWhereSupported(t *testing.T) {
	config, err := (Config{Transparent: true}).normalize()
	if err != nil {
		t.Fatal(err)
	}
	if Supports(CapabilityTransparentWindow) {
		if !config.Transparent || config.ClearColor != (Color{}) {
			t.Fatalf("a transparent window lost its clear color: %+v", config)
		}
		return
	}
	if config.Transparent || config.ClearColor.Alpha != 1 {
		t.Fatalf("an unsupported transparent window must stay opaque: %+v", config)
	}
}

func TestOpaqueWindowKeepsTheDefaultFill(t *testing.T) {
	config, err := (Config{}).normalize()
	if err != nil || config.Transparent || config.ClearColor.Alpha != 1 {
		t.Fatalf("a window without Transparent must stay opaque: %+v, %v", config, err)
	}
}
