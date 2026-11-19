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

// The layer graph (ADR 0084):
//
//	gfx -> style -> base -> ui
//	           \-> components -> (base, ui consume it)
//	app is the platform loop, below ui and outside base.
//
// Behavior and state live in base; style carries look values only; components
// is the style library and may depend on base (a recipe copied into an
// application imports base for the state it reads); ui composes both and is the
// application-facing facade. app must not know about pages.
//
// forbidden lists, for each public package, the module packages its non-test
// code must not import. allowed lists the module packages it may import, so a
// new dependency is a deliberate edit rather than silent drift.
var forbidden = map[string][]string{
	// Values and the canvas know nothing of any layer above them.
	"gfx":  {"app", "ui", "components", "base", "headless", "keymap", "field", "markdown", "text"},
	"text": {"app", "ui", "components", "base", "headless", "gfx", "keymap"},
	// The engine declares pages and paints their elements; appearance, the
	// window loop and the styled controls are above it.
	"base": {"app", "ui", "components", "headless"},
	// Recipes paint into a gfx.Canvas; they neither run a window nor declare a
	// page. base is allowed: a recipe reads the engine's state.
	"components": {"app", "ui", "headless"},
	// A page is not tied to the headless test driver.
	"ui": {"headless"},
	// The platform loop knows nothing of pages or recipes.
	"app": {"ui", "components", "base", "headless"},
}

var allowed = map[string][]string{
	"style":      {"gfx", "text"},
	"base":       {"gfx", "style", "text", "field", "keymap", "window"},
	"components": {"base", "gfx", "style", "text", "field", "markdown", "keymap"},
	"ui":         {"base", "components", "gfx", "style", "text", "field", "keymap", "window", "app"},
	"app":        {"gfx", "keymap", "text", "window", "leaks", "internal"},
}

func TestImportsStayWithinTheLayerGraph(t *testing.T) {
	for pkg, list := range allowed {
		out, err := exec.Command("go", "list", "-f", "{{join .Imports \"\\n\"}}", module+pkg).Output()
		if err != nil {
			t.Fatalf("go list %s: %v", pkg, err)
		}
		permitted := map[string]bool{}
		for _, name := range list {
			permitted[name] = true
		}
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			rel, ok := strings.CutPrefix(line, module)
			if !ok {
				continue
			}
			root := strings.SplitN(rel, "/", 2)[0]
			if permitted[rel] || permitted[root] {
				continue
			}
			t.Errorf("%s imports %s, which is not in its allowed layer set (internal/layering)", pkg, rel)
		}
	}
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
