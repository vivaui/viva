//go:build !js

package components_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The package promises that importing it costs nothing until a recipe is
// called. A package-level variable built by a function call (an icon from a
// helper, a ui.NewPath) breaks that: the linker must keep the initializer and
// everything it reaches. This builds a program that only imports the package
// and fails if any of its symbols are linked.
func TestBlankImportLinksNoComponentSymbols(t *testing.T) {
	if testing.Short() {
		t.Skip("builds a program")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	module := fmt.Sprintf("module linkage\n\ngo 1.26\n\nrequire github.com/vivaui/viva v0.0.0\nreplace github.com/vivaui/viva => %q\n", filepath.ToSlash(root))
	program := "package main\n\nimport _ \"github.com/vivaui/viva/components\"\n\nfunc main() {}\n"
	for name, content := range map[string]string{"go.mod": module, "main.go": program} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	binary := filepath.Join(dir, "linkage.bin")
	build := exec.Command("go", "build", "-mod=mod", "-o", binary, ".")
	build.Dir = dir
	build.Env = append(os.Environ(), "GOWORK=off")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, output)
	}
	symbols, err := exec.Command("go", "tool", "nm", binary).Output()
	if err != nil {
		t.Fatalf("nm: %v", err)
	}
	var linked []string
	for _, line := range strings.Split(string(symbols), "\n") {
		if strings.Contains(line, " github.com/vivaui/viva/components.") {
			linked = append(linked, line)
		}
	}
	if len(linked) > 0 {
		t.Fatalf("a blank import of github.com/vivaui/viva/components links %d symbols, want 0 (a package-level variable is probably built by a function call):\n%s",
			len(linked), strings.Join(linked, "\n"))
	}
}
