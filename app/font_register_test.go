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
	// The registered font gives every printable ASCII character one advance; any stand-in
	// for a family that was not found is proportional.
	if narrow, wide := engine.Width("iiiiiiii", 16, 400), engine.Width("WWWWWWWW", 16, 400); narrow != wide {
		t.Fatalf("the family is not the registered font: widths %v and %v", narrow, wide)
	}
	unregister()
	unregister()
	if narrow, wide := engine.Width("iiiiiiii", 16, 400), engine.Width("WWWWWWWW", 16, 400); narrow == wide {
		t.Fatalf("the font is still drawn after it was unregistered: widths %v and %v", narrow, wide)
	}
}

func TestRegisteringWhatIsNoFontFails(t *testing.T) {
	_, _, err := RegisterFont([]byte("not a font"))
	if err == nil {
		t.Fatal("garbage was registered")
	}
}
