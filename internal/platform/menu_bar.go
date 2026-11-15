package platform

// MenuBarMenu is one top-level application menu. See app.Menu.
type MenuBarMenu struct {
	Title string
	Items []MenuBarItem
}

// MenuBarItem is one entry of a MenuBarMenu.
type MenuBarItem struct {
	Title     string
	Separator bool
	Disabled  bool
	Checked   bool
	// Key is a single lowercase character the system offers as the shortcut;
	// empty shows none. Modifiers are NSEventModifierFlags on macOS.
	Key       string
	Modifiers uint32
	Items     []MenuBarItem
	OnSelect  func()
}
