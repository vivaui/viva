package fontfile

import (
	"embed"
	"os"
	"testing"
)

// The fixtures are generated fonts with no third-party outlines.
//
//go:embed testdata
var fixtures embed.FS

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestFamilyIsWhatTheFontNames(t *testing.T) {
	for file, want := range map[string]string{
		"VivaTestGrid-Regular.ttf":         "Viva Test Grid",
		"VivaTestProportional-Regular.ttf": "Viva Test Proportional",
	} {
		got, err := Family(fixture(t, file))
		if err != nil || got != want {
			t.Errorf("%s: family %q, %v; want %q", file, got, err, want)
		}
	}
}

func TestFamilyRefusesWhatIsNotAnSfntFont(t *testing.T) {
	for name, data := range map[string][]byte{
		"empty":     nil,
		"text":      []byte("this is not a font at all"),
		"woff":      []byte("wOFF\x00\x01\x00\x00" + "0000000000000000"),
		"truncated": fixture(t, "VivaTestGrid-Regular.ttf")[:20],
	} {
		if family, err := Family(data); err == nil {
			t.Errorf("%s: family %q, want an error", name, family)
		}
	}
}

func TestARegisteredFontIsAddedOnceAndRemovedWithItsLastHolder(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LocalAppData", t.TempDir())
	var registry Registry
	var added, removed []string
	add := func(path string) error { added = append(added, path); return nil }
	remove := func(path string) { removed = append(removed, path) }
	data := fixture(t, "VivaTestGrid-Regular.ttf")

	family, first, err := registry.Register(data, add, remove)
	if err != nil || family != "Viva Test Grid" {
		t.Fatalf("register: %q, %v", family, err)
	}
	if again, second, err := registry.Register(data, add, remove); err != nil || again != family {
		t.Fatalf("register again: %q, %v", again, err)
	} else {
		first()
		first() // giving a registration up twice must not take the other holder's away
		if len(removed) != 0 {
			t.Fatalf("removed with a holder left: %v", removed)
		}
		second()
	}
	if len(added) != 1 || len(removed) != 1 || added[0] != removed[0] {
		t.Fatalf("added %v, removed %v; want one of each, the same file", added, removed)
	}
	staged, err := os.ReadFile(added[0])
	if err != nil || string(staged) != string(data) {
		t.Fatalf("the staged file does not hold the font: %v", err)
	}
}

func TestAFontTheEngineRefusesIsNotKept(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	t.Setenv("LocalAppData", t.TempDir())
	var registry Registry
	refuse := func(string) error { return os.ErrInvalid }
	data := fixture(t, "VivaTestGrid-Regular.ttf")
	if _, _, err := registry.Register(data, refuse, func(string) {}); err == nil {
		t.Fatal("a refused font was registered")
	}
	added := 0
	if _, _, err := registry.Register(data, func(string) error { added++; return nil }, func(string) {}); err != nil || added != 1 {
		t.Fatalf("after a refusal the font registers again: %v, added %d", err, added)
	}
}
