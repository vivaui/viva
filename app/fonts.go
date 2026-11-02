package app

import (
	"strings"

	"github.com/vivaui/viva/ui"
)

// MonospaceFamilies lists the fixed-pitch font families installed on this
// system, sorted and without duplicates. An application offers them to the user
// and gives the chosen one to ui.MonoFamily to draw terminal text in it.
//
// What counts as monospace follows the platform. macOS measures each family and
// offers every one whose printable ASCII characters share one advance width,
// including coding families that do not set the fixed-pitch flag; the other
// platforms list what the system declares fixed-pitch.
//
// The list is read from the system on every call and takes a moment, since a
// family is measured rather than taken at its word, so it is for a settings
// page, not for painting. It is empty where the platform cannot enumerate fonts
// (a browser, an unsupported target), in which case ui.MonoSystem and
// ui.MonoClassic remain the choices.
func MonospaceFamilies() []string { return nativeMonospaceFamilies() }

// FontFamilies lists the font families installed on this system, sorted and
// without duplicates — every family, not only the fixed-pitch ones an
// application gives its terminal. An application offers them for the interface's
// own text and gives the chosen one to SetInterfaceFontFamily.
//
// The list is what the system reports, symbol and icon families included
// (Wingdings, Apple Symbols): no platform marks them in a way all three agree
// on, so a picker that wants only families fit for reading prose chooses which
// to leave out. On Windows a family whose name fills the 31 characters GDI
// reports (the system cuts a longer one) is left out, since the cut name would
// name no family. The list is read from the system on every call and can take
// tens of milliseconds, so it is for a settings page, not for painting. It is
// empty where the platform cannot enumerate fonts (a browser, an unsupported
// target).
func FontFamilies() []string { return nativeFontFamilies() }

// maxInterfaceFamilyName bounds a family name; real ones are a few words long.
const maxInterfaceFamilyName = 96

// SetInterfaceFontFamily names the family the proportional text that Viva draws
// itself is shaped in — labels, text fields, the menus, popups and dialogs of the
// application's own windows, everything but the terminal's monospace face — or
// "" for the platform's own interface font. Text the operating system draws
// (native menus, status-item menus, file dialogs, alerts) keeps the system font.
// A name that is too long (96 bytes) or holds a control character is taken as "".
// It reports false where the platform's engine cannot follow a family (a
// browser, an unsupported target); the call has no effect there, and FontFamilies
// is empty too.
//
// The change applies to everything shaped afterwards; what was shaped before is
// forgotten, and the next paint of a window lays out and draws its text in the
// new family, text that did not change included. A repaint is the application's
// to ask for: it is how the windows learn the family changed. The family is the
// process's: every window follows it. A name the system has no family for is
// drawn in the system font.
func SetInterfaceFontFamily(family string) bool {
	if aware, ok := PlatformTextEngine().(ui.FontFamilyAware); ok {
		aware.SetFontFamily(interfaceFamilyName(family))
		return true
	}
	return false
}

// interfaceFamilyName is family as the engines take it: trimmed, and "" when it
// cannot be a family name.
func interfaceFamilyName(family string) string {
	family = strings.TrimSpace(family)
	if len(family) > maxInterfaceFamilyName {
		return ""
	}
	for _, r := range family {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return family
}
