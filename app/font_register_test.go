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
	registered := engine.Width("iiiiiiii", 16, 400) == engine.Width("WWWWWWWW", 16, 400)
	if !registered {
		t.Fatalf("the family is not the registered font: widths %v and %v", engine.Width("iiiiiiii", 16, 400), engine.Width("WWWWWWWW", 16, 400))
	}
	if !listsFamily(family) {
		t.Fatalf("the registered family %q is not listed among the installed families", family)
	}
	unregister()
	unregister()
	// Giving the registration up has to be observable, and what changes is the platform's
	// to decide: the family leaves fontconfig's set on Linux, while macOS keeps enumerating
	// it after a process-scope removal and stops measuring in it instead. The same widths
	// and the same listing together would mean the registration is still in effect.
	sameWidths := engine.Width("iiiiiiii", 16, 400) == engine.Width("WWWWWWWW", 16, 400)
	if sameWidths == registered && listsFamily(family) {
		t.Fatalf("nothing changed after the family %q was unregistered: it still measures as the registered font and is still listed", family)
	}
}

// listsFamily reports whether family is among the families the system reports.
func listsFamily(family string) bool {
	for _, listed := range FontFamilies() {
		if listed == family {
			return true
		}
	}
	return false
}

func TestRegisteringWhatIsNoFontFails(t *testing.T) {
	_, _, err := RegisterFont([]byte("not a font"))
	if err == nil {
		t.Fatal("garbage was registered")
	}
}
