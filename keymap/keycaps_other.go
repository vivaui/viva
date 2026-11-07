//go:build !darwin

package keymap

import "github.com/vivaui/viva/ui"

// Primary is the modifier that carries an application's own shortcuts on this
// platform: Command on macOS, Control elsewhere.
const Primary = ui.ModifierControl

// modifierKeycap is the name Windows and Linux keyboards print on a modifier;
// the Meta key is the Windows key.
func modifierKeycap(modifier ui.Modifiers) string {
	switch modifier {
	case ui.ModifierControl:
		return "Ctrl"
	case ui.ModifierAlt:
		return "Alt"
	case ui.ModifierShift:
		return "Shift"
	default:
		return "Win"
	}
}
