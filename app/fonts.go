package app

import "github.com/vivaui/viva/ui"

// MonospaceFamilies lists the fixed-pitch font families installed on this
// system, sorted and without duplicates. An application offers them to the user
// and gives the chosen one to ui.MonoFamily to draw terminal text in it.
//
// The list is read from the system on every call and can take tens of
// milliseconds, so it is for a settings page, not for painting. It is empty
// where the platform cannot enumerate fonts (a browser, an unsupported target),
// in which case ui.MonoSystem and ui.MonoClassic remain the choices.
func MonospaceFamilies() []string { return nativeMonospaceFamilies() }

// FontFamilies lists the font families installed on this system, sorted and
// without duplicates — every family, not only the fixed-pitch ones an
// application gives its terminal. An application offers them for the interface's
// own text and gives the chosen one to SetInterfaceFontFamily.
//
// The list is read from the system on every call and can take tens of
// milliseconds, so it is for a settings page, not for painting. It is empty
// where the platform cannot enumerate fonts (a browser, an unsupported target).
func FontFamilies() []string { return nativeFontFamilies() }

// SetInterfaceFontFamily names the family the interface's proportional text is
// shaped in — labels, menus, dialogs, everything but the terminal's monospace
// face — or "" for the platform's own interface font. It reports false where the
// platform's engine cannot follow a family (a browser, an unsupported target);
// the call has no effect there, and FontFamilies is empty too.
//
// The change applies to everything shaped afterwards; what was shaped before is
// forgotten. A repaint is the application's to ask for.
func SetInterfaceFontFamily(family string) bool {
	if aware, ok := PlatformTextEngine().(ui.FontFamilyAware); ok {
		aware.SetFontFamily(family)
		return true
	}
	return false
}
