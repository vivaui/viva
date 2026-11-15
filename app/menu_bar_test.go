package app

import (
	"testing"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/keymap"
)

func TestMenuBarItemsCarryCharacterShortcuts(t *testing.T) {
	menus := platformMenuBar([]Menu{{
		Title: "File",
		Items: []MenuBarItem{
			{Title: "New", Shortcut: keymap.Chord{Char: 'N', Modifiers: gfx.ModifierMeta}},
			{Title: "Disabled", Disabled: true},
			{Title: "More", Shortcut: keymap.Chord{}, Submenu: []MenuBarItem{{Title: "Inside", Shortcut: keymap.Chord{Char: 'i', Modifiers: gfx.ModifierAlt}}}},
		},
	}})
	if len(menus) != 1 || menus[0].Title != "File" || len(menus[0].Items) != 3 {
		t.Fatalf("menus = %+v", menus)
	}
	newItem := menus[0].Items[0]
	if newItem.Key != "n" || newItem.Modifiers != 1<<20 {
		t.Fatalf("New = %+v, want key n with Command", newItem)
	}
	if menus[0].Items[1].Disabled != true {
		t.Fatalf("Disabled = %+v", menus[0].Items[1])
	}
	nested := menus[0].Items[2]
	if len(nested.Items) != 1 || nested.Items[0].Key != "i" || nested.Items[0].Modifiers != 1<<19 {
		t.Fatalf("nested = %+v", nested)
	}
}

func TestANamedKeyShowsNoMenuShortcut(t *testing.T) {
	menus := platformMenuBar([]Menu{{Title: "Edit", Items: []MenuBarItem{
		{Title: "Delete", Shortcut: keymap.Chord{Key: gfx.KeyDelete, Modifiers: gfx.ModifierMeta}},
	}}})
	if menus[0].Items[0].Key != "" || menus[0].Items[0].Modifiers != 0 {
		t.Fatalf("a named key became a menu shortcut: %+v", menus[0].Items[0])
	}
}
