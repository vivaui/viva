// Package layering checks the direction of the public packages' imports, so a
// package cannot start depending on one above it unnoticed.
package layering

import (
	"os/exec"
	"slices"
	"strings"
	"testing"
)

const module = "github.com/vivaui/viva/"

// forbidden lists, for each public package, the module packages its non-test
// code must not import. The rules are the dependency directions that hold today
// and that the packages' documentation relies on. ADR 0084: base is the
// behaviour layer below the styles; ui is the styled controls and the facade,
// and components is the style library ui composes.
var forbidden = map[string][]string{
	// Values and the canvas know nothing of any layer above them.
	"gfx":  {"app", "ui", "components", "base", "headless", "keymap", "field", "markdown", "text"},
	"text": {"app", "ui", "components", "base", "headless", "gfx", "keymap"},
	// The engine declares pages and paints their elements; appearance, the
	// window loop and the styled controls are above it.
	"base": {"app", "ui", "components", "headless"},
	// Recipes paint into a gfx.Canvas; they neither run a window nor declare a page.
	"components": {"app", "ui", "headless"},
	// A page is not tied to the headless test driver.
	"ui": {"headless"},
	// The platform loop knows nothing of pages or recipes.
	"app": {"ui", "components", "base", "headless"},
}

func TestImportDirections(t *testing.T) {
	for pkg, banned := range forbidden {
		out, err := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", module+pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			rel, ok := strings.CutPrefix(line, module)
			if !ok {
				continue
			}
			for _, b := range banned {
				if rel == b || strings.HasPrefix(rel, b+"/") {
					t.Errorf("%s imports %s; the layers only import downward (internal/layering)", pkg, rel)
				}
			}
		}
	}
}

func TestRulesNamePackagesThatExist(t *testing.T) {
	out, err := exec.Command("go", "list", module+"...").Output()
	if err != nil {
		t.Fatal(err)
	}
	have := strings.Fields(string(out))
	exists := func(rel string) bool { return slices.Contains(have, module+rel) }
	for pkg, banned := range forbidden {
		for _, rel := range append([]string{pkg}, banned...) {
			if !exists(rel) {
				t.Errorf("rule mentions %s, which does not exist", rel)
			}
		}
	}
}
