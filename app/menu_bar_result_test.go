//go:build darwin || windows || linux

package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

// A native menu runs its entry outside the window's event handlers. What the
// action did must still reach the window the user is working in, or a Find
// chosen from the menu bar opens its bar without the keyboard.
func TestMenuActionCarriesItsResultToTheActiveWindow(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	want := gfx.EventResult{Handled: true, Repaint: true, Focus: gfx.Item(7), FocusVisible: true}
	if err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{Title: "menu"}, nil); err != nil {
			return err
		}
		return application.SetMenuBar([]Menu{{
			Title: "Edit",
			Items: []MenuBarItem{
				{Title: "Find", Shortcut: keymap.Chord{Char: 'f', Modifiers: gfx.ModifierMeta}, OnSelect: func() gfx.EventResult { return want }},
				{Title: "More", Submenu: []MenuBarItem{{Title: "Inside", OnSelect: func() gfx.EventResult { return gfx.EventResult{Handled: true} }}}},
			},
		}})
	}); err != nil {
		t.Fatal(err)
	}
	window := host.windows[0]
	window.config.OnActivate() // the user brings this window to the front
	if got := host.menuBar[0].Items[0].OnSelect(); got != want {
		t.Fatalf("menu action returned %+v, want %+v", got, want)
	}
	if len(window.applied) != 1 || window.applied[0] != want {
		t.Fatalf("window carried out %+v, want %+v", window.applied, want)
	}
	if window.redraws != 1 {
		t.Fatalf("menu action asked for %d repaints, want 1", window.redraws)
	}
	// A submenu entry is wrapped the same way, and a result that neither moves
	// the focus nor asks for a repaint does not wake the window.
	if got := host.menuBar[0].Items[1].Submenu[0].OnSelect(); got != (gfx.EventResult{Handled: true}) {
		t.Fatalf("submenu action returned %+v", got)
	}
	if len(window.applied) != 2 || window.redraws != 1 {
		t.Fatalf("submenu action applied=%d redraws=%d, want 2 applied and 1 repaint", len(window.applied), window.redraws)
	}
}
