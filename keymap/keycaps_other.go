//go:build !darwin

package keymap

import "github.com/vivaui/viva/gfx"

// Primary is the modifier that carries an application's own shortcuts on this
// platform: Command on macOS, Control elsewhere.
const Primary = gfx.ModifierControl

// modifierKeycap is the name Windows and Linux keyboards print on a modifier;
// the Meta key is the Windows key.
func modifierKeycap(modifier gfx.Modifiers) string {
	switch modifier {
	case gfx.ModifierControl:
		return "Ctrl"
	case gfx.ModifierAlt:
		return "Alt"
	case gfx.ModifierShift:
		return "Shift"
	default:
		return "Win"
	}
}
