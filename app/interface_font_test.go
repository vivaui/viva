package app

import (
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestFontFamiliesAreSortedUniqueAndIncludeTheSystemsOwn(t *testing.T) {
	families := FontFamilies()
	if runtime.GOOS == "js" {
		if len(families) != 0 {
			t.Fatalf("a page listed fonts: %v", families)
		}
		return
	}
	if len(families) == 0 {
		t.Skipf("%s reports no families here", runtime.GOOS)
	}
	if !slices.IsSortedFunc(families, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) }) {
		t.Fatalf("not sorted: %v", families)
	}
	seen := map[string]bool{}
	for _, name := range families {
		if name == "" || seen[name] || strings.ContainsAny(name, "\n\x00") || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
			t.Fatalf("bad entry %q in %v", name, families)
		}
		seen[name] = true
	}
	// A family the system draws its own interface in, and no terminal of the
	// system ships without: the list holds every family, not the fixed-pitch
	// ones alone.
	want := map[string]string{"darwin": "Helvetica", "windows": "Segoe UI"}[runtime.GOOS]
	if want != "" && !seen[want] {
		t.Fatalf("%s is missing from %v", want, families)
	}
	mono := MonospaceFamilies()
	if len(mono) > len(families) || !slices.ContainsFunc(families, func(name string) bool {
		return len(mono) > 0 && mono[0] == name
	}) {
		t.Fatalf("the monospace list %v is not part of the whole %v", mono, families)
	}
}

func TestSetInterfaceFontFamilyShapesAgainAndTheDefaultComesBack(t *testing.T) {
	if runtime.GOOS == "js" {
		if SetInterfaceFontFamily("Anything") {
			t.Fatal("a page accepted an interface family")
		}
		return
	}
	engine := PlatformTextEngine()
	if engine == nil {
		t.Skipf("%s has no native text engine here", runtime.GOOS)
	}
	if !SetInterfaceFontFamily("") {
		t.Fatal("the platform's engine cannot follow a family")
	}
	before := engine.Width("The quick brown fox 0123", 14, 0.15)
	if before <= 0 {
		t.Fatalf("the engine measures %v", before)
	}
	if families := FontFamilies(); len(families) > 0 {
		// A family the system has: shaped without a complaint, in it.
		if !SetInterfaceFontFamily(families[0]) {
			t.Fatal("the engine refused a family it listed")
		}
		if width := engine.Width("The quick brown fox 0123", 14, 0.15); width <= 0 {
			t.Fatalf("the engine measures %v in %s", width, families[0])
		}
	}
	// A family nothing has is the platform's stand-in, not an error, and the
	// default shapes exactly as it did once it is asked for again.
	SetInterfaceFontFamily("No such family, certainly not installed")
	SetInterfaceFontFamily("")
	if after := engine.Width("The quick brown fox 0123", 14, 0.15); after != before {
		t.Fatalf("the default came back as %v, want %v", after, before)
	}
}
