package app

import (
	_ "embed"
	"errors"
	"testing"

	"github.com/vivaui/viva/ui"
)

// A generated font with no third-party outlines: every printable ASCII character has one advance.
//
//go:embed testdata/VivaTestGrid-Regular.ttf
var gridFont []byte

func TestARegisteredFontIsDrawnInItsFamily(t *testing.T) {
	family, unregister, err := RegisterFont(gridFont)
	if errors.Is(err, ErrUnsupported) {
		t.Skip("this platform takes no fonts at run time:", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if family != "Viva Test Grid" {
		t.Fatalf("family = %q", family)
	}
	engine := PlatformTextEngine().(ui.FontFamilyAware)
	t.Cleanup(func() { engine.SetFontFamily("") })
	engine.SetFontFamily(family)
	// The registered font gives every printable ASCII character one advance.
	registeredNarrow, registeredWide := engine.Width("iiiiiiii", 16, 400), engine.Width("WWWWWWWW", 16, 400)
	if registeredNarrow != registeredWide {
		t.Fatalf("the family is not the registered font: widths %v and %v", registeredNarrow, registeredWide)
	}
	unregister()
	unregister()
	// No assumption about what the stand-in is: only that the registration ended, so the
	// family no longer measures as the font it named. A platform whose stand-in happens to
	// share this font's metrics is not covered by this, which is what the ink and the family
	// name in an application's own tests are for.
	narrow, wide := engine.Width("iiiiiiii", 16, 400), engine.Width("WWWWWWWW", 16, 400)
	if narrow == registeredNarrow && wide == registeredWide {
		t.Fatalf("the family still measures as the registered font after it was unregistered: widths %v and %v", narrow, wide)
	}
}

func TestRegisteringWhatIsNoFontFails(t *testing.T) {
	_, _, err := RegisterFont([]byte("not a font"))
	if err == nil {
		t.Fatal("garbage was registered")
	}
}
