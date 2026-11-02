//go:build darwin

package darwin

import (
	"slices"
	"testing"
)

// A named family follows the weight of the text on the system font's continuous scale,
// so a medium label is not drawn in the regular face and semibold not in regular either:
// Helvetica Neue ships a face for each step.
func TestANamedInterfaceFamilyHasItsMediumAndBoldFaces(t *testing.T) {
	const family = "Helvetica Neue"
	if !slices.Contains(FontFamilies(), family) {
		t.Skipf("%s is not installed", family)
	}
	engine := SharedTextEngine()
	engine.SetFontFamily(family)
	t.Cleanup(func() { engine.SetFontFamily("") })
	const text = "The quick brown fox jumps over 0123"
	regular := engine.Width(text, 14, 0)
	medium := engine.Width(text, 14, 0.15)
	bold := engine.Width(text, 14, 0.4)
	if !(regular < medium && medium < bold) {
		t.Fatalf("%s measures %v regular, %v medium, %v bold: the faces were not told apart", family, regular, medium, bold)
	}
}
