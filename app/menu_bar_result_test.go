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

// AppKit offers a menu's key equivalent to the focused content before the
// menu, and so does the runtime: the application's shortcut handler sees the
// chord with the focused control as its target, and the entry's own action
// runs only when that handler declines it. Choosing the entry runs the same
// command as pressing its shortcut, which is what a menu advertises.
func TestMenuShortcutReachesTheFocusedControlBeforeTheEntry(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	selected := gfx.EventResult{Handled: true, ClipboardText: "selected text", Repaint: true}
	var pressed gfx.KeyEvent
	selectedEntry := false
	if err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{
			Title: "menu",
			OnShortcut: func(event gfx.KeyEvent) gfx.EventResult {
				pressed = event
				return selected
			},
		}, nil); err != nil {
			return err
		}
		return application.SetMenuBar([]Menu{{
			Title: "Edit",
			Items: []MenuBarItem{{
				Title:    "Copy",
				Shortcut: keymap.Chord{Char: 'c', Modifiers: gfx.ModifierMeta},
				OnSelect: func() gfx.EventResult {
					selectedEntry = true
					return gfx.EventResult{}
				},
			}},
		}})
	}); err != nil {
		t.Fatal(err)
	}
	window := host.windows[0]
	window.focused = gfx.Item(7)
	if result := host.menuBar[0].Items[0].OnSelect(); result != selected {
		t.Fatalf("menu shortcut returned %+v, want the handler's %+v", result, selected)
	}
	if selectedEntry {
		t.Fatal("the entry's own action ran although the shortcut handler took the chord")
	}
	if pressed.Target != gfx.Item(7) || pressed.Text != "c" || pressed.Modifiers != gfx.ModifierMeta {
		t.Fatalf("handler saw %+v, want Cmd+C aimed at the focused control", pressed)
	}
	if len(window.applied) != 1 || window.applied[0] != selected {
		t.Fatalf("window carried out %+v, want %+v", window.applied, selected)
	}
	if window.redraws != 1 {
		t.Fatalf("menu shortcut asked for %d repaints, want 1", window.redraws)
	}
}

func TestMenuEntryRunsWhenTheShortcutHandlerDeclines(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	entry := gfx.EventResult{Handled: true, Repaint: true, Focus: gfx.Item(3)}
	if err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{
			Title:      "menu",
			OnShortcut: func(gfx.KeyEvent) gfx.EventResult { return gfx.EventResult{} },
		}, nil); err != nil {
			return err
		}
		return application.SetMenuBar([]Menu{{
			Title: "Edit",
			Items: []MenuBarItem{{
				Title:    "Find",
				Shortcut: keymap.Chord{Char: 'f', Modifiers: gfx.ModifierMeta},
				OnSelect: func() gfx.EventResult { return entry },
			}},
		}})
	}); err != nil {
		t.Fatal(err)
	}
	if got := host.menuBar[0].Items[0].OnSelect(); got != entry {
		t.Fatalf("menu entry returned %+v, want %+v", got, entry)
	}
	window := host.windows[0]
	if len(window.applied) != 1 || window.applied[0] != entry {
		t.Fatalf("window carried out %+v, want %+v", window.applied, entry)
	}
}

// A named key has no menu key equivalent, so choosing its entry is how the
// menu runs it. The runtime offers it to the key handler as pressing the key
// would, and the entry's own action stays unused when the handler takes it.
func TestMenuEntryOffersANamedChordToTheKeyHandler(t *testing.T) {
	host := &fakeHost{}
	useHost(t, host)
	copied := gfx.EventResult{Handled: true, ClipboardText: "selected text"}
	var pressed gfx.KeyEvent
	entryRan := false
	if err := RunApplication(func(application *Application) error {
		if _, err := application.OpenWindow(Config{
			Title: "menu",
			OnKey: func(event gfx.KeyEvent) gfx.EventResult {
				pressed = event
				return copied
			},
		}, nil); err != nil {
			return err
		}
		return application.SetMenuBar([]Menu{{
			Title: "Edit",
			Items: []MenuBarItem{{
				Title:    "Copy",
				Shortcut: keymap.Chord{Key: gfx.KeyF8},
				OnSelect: func() gfx.EventResult {
					entryRan = true
					return gfx.EventResult{}
				},
			}},
		}})
	}); err != nil {
		t.Fatal(err)
	}
	host.windows[0].focused = gfx.Item(4)
	if got := host.menuBar[0].Items[0].OnSelect(); got != copied {
		t.Fatalf("menu entry returned %+v, want the key handler's %+v", got, copied)
	}
	if entryRan {
		t.Fatal("the entry's own action ran although the key handler took the chord")
	}
	if pressed.Key != gfx.KeyF8 || pressed.Target != gfx.Item(4) {
		t.Fatalf("handler saw %+v, want F8 aimed at the focused control", pressed)
	}
}
