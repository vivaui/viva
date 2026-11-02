//go:build linux

package app

import "testing"

func TestNewWindowControlsAdoptsEnvironmentDesktopTheme(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	gnomeControls := newWindowControls(&Config{TitlebarHeight: 36})
	if gnomeControls.theme != desktopAdwaita {
		t.Errorf("expected desktopAdwaita, got %v", gnomeControls.theme)
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "KDE")
	kdeControls := newWindowControls(&Config{TitlebarHeight: 36})
	if kdeControls.theme != desktopBreeze {
		t.Errorf("expected desktopBreeze, got %v", kdeControls.theme)
	}

	t.Setenv("XDG_CURRENT_DESKTOP", "XFCE")
	xfceControls := newWindowControls(&Config{TitlebarHeight: 36})
	if xfceControls.theme != desktopDefault {
		t.Errorf("expected desktopDefault, got %v", xfceControls.theme)
	}
}
