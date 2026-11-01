//go:build darwin

package darwin

import (
	"strings"
	"testing"
	"unsafe"
)

func TestSystemStringArrayABI(t *testing.T) {
	pointerSize := unsafe.Sizeof(uintptr(0))
	size, countOffset := systemStringArrayABI()
	if got, want := size, pointerSize*2; got != want {
		t.Fatalf("viva_system_string_array size = %d, want %d", got, want)
	}
	if got := countOffset; got != pointerSize {
		t.Fatalf("viva_system_string_array.count offset = %d, want %d", got, pointerSize)
	}
}

func TestAllowedExtensions(t *testing.T) {
	extensions, err := allowedExtensions([]FileFilter{
		{Name: "Images", Pattern: "*.png; *.JPG"},
		{Name: "Text", Pattern: ".txt;*.png"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"png", "JPG", "txt"}
	if len(extensions) != len(want) {
		t.Fatalf("extensions = %q, want %q", extensions, want)
	}
	for index := range want {
		if extensions[index] != want[index] {
			t.Fatalf("extensions = %q, want %q", extensions, want)
		}
	}
}

func TestAllowedExtensionsAllFilesClearsRestriction(t *testing.T) {
	extensions, err := allowedExtensions([]FileFilter{
		{Name: "Text", Pattern: "*.txt"},
		{Name: "All", Pattern: "*.*"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if extensions != nil {
		t.Fatalf("extensions = %q, want no restriction", extensions)
	}
}

func TestAllowedExtensionsRejectsUnsupportedGlob(t *testing.T) {
	for _, pattern := range []string{"report-*.txt", "*.", "*.txt;;*.md"} {
		_, err := allowedExtensions([]FileFilter{{Name: "Text", Pattern: pattern}})
		if err == nil {
			t.Fatalf("pattern %q was accepted", pattern)
		}
		if pattern == "report-*.txt" && !strings.Contains(err.Error(), "wildcard") {
			t.Fatalf("error = %v, want wildcard error", err)
		}
	}
}

func TestSetCursorRejectsUnknownShape(t *testing.T) {
	if err := SetCursorShape(255); err == nil {
		t.Fatal("unknown cursor shape was accepted")
	}
}

func TestSystemCStringRejectsNUL(t *testing.T) {
	if value, err := systemCString("test value", "bad\x00value"); err == nil || value != nil {
		t.Fatalf("systemCString returned value %p, error %v", value, err)
	}
}

func TestNormalizeExtension(t *testing.T) {
	if value, err := normalizeExtension(" .txt "); err != nil || value != "txt" {
		t.Fatalf("normalizeExtension = %q, %v", value, err)
	}
	for _, value := range []string{"*.txt", "path/txt", "txt;md"} {
		if _, err := normalizeExtension(value); err == nil {
			t.Fatalf("normalizeExtension accepted %q", value)
		}
	}
}

func TestAppearanceCanBeReadAndWatchedTwice(t *testing.T) {
	first := DarkAppearance()
	if again := DarkAppearance(); again != first {
		t.Fatalf("two reads disagree: %v then %v", first, again)
	}
	WatchAppearance()
	WatchAppearance() // one observer, however often it is asked for
	if DarkAppearance() != first {
		t.Fatal("watching changed the answer")
	}
}

func TestPinningTheNativeAppearanceLeavesTheSystemsLookReadable(t *testing.T) {
	system := DarkAppearance()
	defer SetAppearance(false, false)

	SetAppearance(true, false)
	if EffectiveDark() {
		t.Fatal("pinned light, what the application draws is still dark")
	}
	if DarkAppearance() != system {
		t.Fatal("the system's own look changed with the pin")
	}
	SetAppearance(false, true)
	if !EffectiveDark() {
		t.Fatal("pinned dark, what the application draws is still light")
	}
	if DarkAppearance() != system {
		t.Fatal("the system's own look changed with the pin")
	}
	SetAppearance(false, false)
	if EffectiveDark() != system || DarkAppearance() != system {
		t.Fatal("giving the look back did not return the system's")
	}
}

func TestSchemeHandlerAvailable(t *testing.T) {
	// https is claimed by the browser on every Mac that can run the tests; a
	// made-up scheme is claimed by nothing.
	if available, err := SchemeHandlerAvailable("https"); err != nil || !available {
		t.Fatalf("https: available = %t, err = %v", available, err)
	}
	if available, err := SchemeHandlerAvailable("viva-test-unregistered"); err != nil || available {
		t.Fatalf("unregistered scheme: available = %t, err = %v", available, err)
	}
}
