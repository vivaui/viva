package keymap

import "github.com/vivaui/viva/gfx"

// Primary is the modifier that carries an application's own shortcuts on this
// platform: Command on macOS, Control elsewhere.
const Primary = gfx.ModifierMeta

// modifierKeycap is the symbol macOS draws for a modifier.
func modifierKeycap(modifier gfx.Modifiers) string {
	switch modifier {
	case gfx.ModifierControl:
		return "⌃"
	case gfx.ModifierAlt:
		return "⌥"
	case gfx.ModifierShift:
		return "⇧"
	default:
		return "⌘"
	}
}
