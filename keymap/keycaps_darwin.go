package keymap

import "github.com/vivaui/viva/ui"

// Primary is the modifier that carries an application's own shortcuts on this
// platform: Command on macOS, Control elsewhere.
const Primary = ui.ModifierMeta

// modifierKeycap is the symbol macOS draws for a modifier.
func modifierKeycap(modifier ui.Modifiers) string {
	switch modifier {
	case ui.ModifierControl:
		return "⌃"
	case ui.ModifierAlt:
		return "⌥"
	case ui.ModifierShift:
		return "⇧"
	default:
		return "⌘"
	}
}
