package app

import (
	"fmt"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// MonospaceFamilies lists the fixed-pitch font families installed on this
// system, sorted and without duplicates. An application offers them to the user
// and gives the chosen one to gfx.MonoFamily to draw terminal text in it.
//
// What counts as monospace follows the platform. macOS and Windows measure
// each family and offer every one whose printable ASCII characters share one
// advance width, including coding families that do not set the fixed-pitch
// flag; the other platforms list what the system declares fixed-pitch.
//
// The list is read from the system on every call and takes a moment, since a
// family is measured rather than taken at its word, so it is for a settings
// page, not for painting. It is empty where the platform cannot enumerate fonts
// (a browser, an unsupported target), in which case gfx.MonoSystem and
// gfx.MonoClassic remain the choices.
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

// RegisterFont makes the TrueType or OpenType font in data (a .ttf, .otf or .ttc, as an
// application embeds with //go:embed) available to this process, without installing it in the
// user's font library, and returns the family it names, ready for gfx.MonoFamily,
// gfx.MonoStack and SetInterfaceFontFamily, and a function that takes the registration away
// again. A collection registers every font in it and names the first one's family; the weights
// and styles of a family are separate files, each registered. Registering the same data again
// shares the registration, which ends with the last unregister.
//
// Register before the font is asked for: a family looked up while it was missing is drawn in the
// platform's stand-in until the next shaping. Text already shaped is forgotten, so the next paint
// draws it in the new font.
//
// It fails with ErrUnsupported where the platform cannot take a font at run time, and with an
// error for data that is not a font or that the engine refuses. WOFF data is not read. The Web
// target returns ErrUnsupported because FontFace loading is asynchronous; use RegisterFontAsync
// there.
func RegisterFont(data []byte) (family string, unregister func(), err error) {
	registrar, ok := PlatformTextEngine().(gfx.FontRegistrar)
	if !ok {
		return "", nil, fmt.Errorf("%w: registering fonts", ErrUnsupported)
	}
	return registrar.RegisterFont(data)
}

// maxInterfaceFamilyName bounds a family name; real ones are a few words long.
const maxInterfaceFamilyName = 96

// SetInterfaceFontFamily names the family the proportional text that Viva draws
// itself is shaped in — labels, text fields, the menus, popups and dialogs of the
// application's own windows, everything but the terminal's monospace face — or
// "" for the platform's own interface font. Text the operating system draws
// (native menus, status-item menus, file dialogs, alerts) keeps the system font.
// A name that is too long (96 bytes) or holds a control character is taken as "".
// It reports false where the platform's engine cannot follow a family (an
// unsupported target); the call has no effect there, and FontFamilies is empty
// too. The Web target follows the family through its document text engine.
//
// The change applies to everything shaped afterwards; what was shaped before is
// forgotten, and the next paint of a window lays out and draws its text in the
// new family, text that did not change included. A repaint is the application's
// to ask for: it is how the windows learn the family changed. The family is the
// process's: every window follows it. A name the system has no family for is
// drawn in the system font.
func SetInterfaceFontFamily(family string) bool {
	if aware, ok := PlatformTextEngine().(gfx.FontFamilyAware); ok {
		aware.SetFontFamily(interfaceFamilyName(family))
		return true
	}
	return false
}

// SetFontThickening draws the text Viva itself rasterizes — labels, text fields, the terminal
// and its preedit — with a thicker stroke, and reports whether the platform has the choice.
// macOS applies it with CoreText's font smoothing, which is what the system's own text looks
// like. The process starts at the weight the platform's own text has — on macOS the user's
// font smoothing, elsewhere the font's own outlines — and this call is the application's
// explicit choice from then on, as a terminal with a font-thicken option makes. Windows and
// Linux rasterize text their own way with no such knob, and a browser's page follows its
// document, so their engines do not offer it: the call reports false and draws what it drew
// before.
//
// The weight is the process's and shows on the next paint: shaped lines are kept, but the
// pictures of the labels and of the editor are made again, so the application asks for a
// redraw when the weight changes. Asking for the weight that is already in use changes
// nothing.
func SetFontThickening(on bool) bool {
	if thickener, ok := PlatformTextEngine().(gfx.FontThickener); ok {
		thickener.SetFontThickening(on)
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
