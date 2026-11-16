package app

import "github.com/vivaui/viva/gfx"

// The weights of the text Viva draws, which the application may choose where the
// platform's engine has the choice: which installed weight of a family the main
// text and the bold text are drawn at, for the interface's proportional text and
// for the fixed-pitch text apart.

// SetInterfaceFont is SetInterfaceFontFamily with the two weights the interface's
// text is drawn at: main for regular and medium text, bold for semibold and bold.
// The weights are DWRITE_FONT_WEIGHT values — 300 Light, 400 Regular, 500 Medium,
// 600 Semibold, 700 Bold and so on, as FontWeights lists a family's — and 0 keeps
// the platform's own (400 and 700). A weight the family is not installed with is
// drawn through the platform's nearest match, so naming one never fails.
//
// Weights are followed where the platform's engine has the choice (Windows today);
// an engine without it draws the family at its own weights, and FontWeights lists
// nothing there. Set either band to the weight already in use and nothing changes.
func SetInterfaceFont(family string, main, bold int) bool {
	engine := PlatformTextEngine()
	aware, ok := engine.(gfx.FontFamilyAware)
	if !ok {
		return false
	}
	aware.SetFontFamily(interfaceFamilyName(family))
	if weights, ok := engine.(gfx.FontWeightAware); ok {
		weights.SetFontWeights(main, bold)
	}
	return true
}

// FontWeights lists the weights family is installed with, ascending, as
// DWRITE_FONT_WEIGHT values (300 Light, 400 Regular, 500 Medium, 600 Semibold,
// 700 Bold, 800 ExtraBold, 900 Black …): the choices SetInterfaceFont takes for
// the main and the bold text of that family. The list is the family's own, so a
// family installed in one weight lists one number, and a family the system does
// not have lists none.
//
// It is empty where the platform cannot report a family's weights (macOS and
// Linux today, and every target that cannot enumerate fonts), where an
// application offers the family's default weights instead. The list is read from
// the system on every call, so it is for a settings page, not for painting.
func FontWeights(family string) []int { return nativeFontWeights(family) }

// SetMonospaceFontWeights names the weights the fixed-pitch text that Viva draws
// is drawn at: the terminal's and the code blocks', whose face the application
// chooses through gfx.MonoFace. main is the regular text's weight, bold the bold
// runs' (SGR bold, WeightBold), as DWRITE_FONT_WEIGHT values, and 0 keeps the
// platform's own (400 and 700). The cell the fixed-pitch text is laid out on
// follows the weights, as the face's own advance does.
//
// It reports false where the platform's engine has no such choice (macOS and
// Linux today); the call has no effect there, as SetInterfaceFont's weights have
// none. The change applies to everything shaped afterwards, and the application
// asks for the next paint.
func SetMonospaceFontWeights(main, bold int) bool {
	weights, ok := PlatformTextEngine().(gfx.MonoWeightAware)
	if !ok {
		return false
	}
	weights.SetMonoWeights(main, bold)
	return true
}
