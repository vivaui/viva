package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"unicode"

	"github.com/vivaui/viva/internal/componentdemo"
)

func TestCatalogAndExportUseActualGoSource(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	pages, manifest, err := loadPages(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != len(componentdemo.Catalog()) || len(manifest.Revision) != 12 {
		t.Fatalf("unexpected catalog: %d pages, revision %q", len(pages), manifest.Revision)
	}
	output := t.TempDir()
	if err := writeData(root, output, pages, manifest); err != nil {
		t.Fatal(err)
	}
	preview, err := os.ReadFile(filepath.Join(output, "public/preview/index.html"))
	if err != nil || !bytes.Contains(preview, []byte("preview-"+manifest.Revision+".wasm")) || bytes.Contains(preview, []byte("__VIVA_REVISION__")) {
		t.Fatalf("preview assets lack the current source fingerprint: %v", err)
	}
	// The copies import the repository's packages; without a module of their own
	// they would be compiled again by "go vet ./..." when the output sits under
	// build/.
	if module, err := os.ReadFile(filepath.Join(output, "public/sources/go.mod")); err != nil || !bytes.Contains(module, []byte("\nmodule ")) {
		t.Fatalf("the published sources are not a module of their own: %v", err)
	}
	documented, componentPages, examples := 0, 0, 0
	for _, entry := range manifest.Coverage {
		if entry.Documented {
			documented++
		}
		original, err := os.ReadFile(filepath.Join(root, entry.File))
		if err != nil {
			t.Fatal(err)
		}
		exported, err := os.ReadFile(filepath.Join(output, "public", "sources", entry.File))
		if err != nil || !bytes.Equal(original, exported) {
			t.Fatalf("source mismatch for %s: %v", entry.ID, err)
		}
	}
	for _, page := range pages {
		if strings.HasPrefix(page.Source, "components/") {
			componentPages++
		}
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(page.Source))); err != nil {
			t.Fatalf("%s: source %s: %v", page.ID, page.Source, err)
		}
	}
	if documented != componentPages {
		t.Fatalf("documented coverage %d != component pages %d", documented, componentPages)
	}
	for _, page := range pages {
		if page.Revision != manifest.Revision {
			t.Fatalf("stale page metadata: %s", page.ID)
		}
		for range page.Examples {
			examples++
		}
	}
	wantExamples := 0
	for _, component := range componentdemo.Catalog() {
		wantExamples += len(component.Examples)
	}
	if examples != wantExamples {
		t.Fatalf("expected %d examples, got %d", wantExamples, examples)
	}
	data, err := os.ReadFile(filepath.Join(output, "catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var exported struct {
		Pages    []Page     `json:"pages"`
		Revision string     `json:"revision"`
		Coverage []Coverage `json:"coverage"`
	}
	if err := json.Unmarshal(data, &exported); err != nil || !reflect.DeepEqual(exported.Pages, pages) || exported.Revision != manifest.Revision || !reflect.DeepEqual(exported.Coverage, manifest.Coverage) {
		t.Fatalf("Astro data contract mismatch: %v", err)
	}
}

func manifestFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, directory := range []string{"app", "ui", "gfx", "components", "internal", "cmd/component-preview", "cmd/viva/templates", "tools/docs"} {
		if err := os.MkdirAll(filepath.Join(root, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, filename := range []string{"go.mod", "go.sum", "components/button.go"} {
		if err := os.WriteFile(filepath.Join(root, filename), []byte("original"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestButtonExamplesCarryUsageSnippets(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	pages, _, err := loadPages(root)
	if err != nil {
		t.Fatal(err)
	}
	var button *Page
	for index := range pages {
		if pages[index].ID == "button" {
			button = &pages[index]
			break
		}
	}
	if button == nil {
		t.Fatal("button page missing")
	}
	for _, example := range button.Examples {
		if !strings.Contains(example.Snippet, "ui.Button(") {
			t.Errorf("button/%s has no call-site snippet: %q", example.ID, example.Snippet)
		}
		if example.ID != "variants" && example.ID != "disabled" {
			continue
		}
		for _, label := range []string{"Default", "Destructive", "Outline", "Secondary", "Ghost", "Link"} {
			if !strings.Contains(example.Snippet, `"`+label+`"`) {
				t.Errorf("button/%s snippet omits the %s variant", example.ID, label)
			}
		}
		if example.ID == "disabled" && !strings.Contains(example.Snippet, "Disabled(true)") {
			t.Errorf("button/%s snippet does not disable its variants", example.ID)
		}
	}
}

func TestEveryExampleCarriesUsageSnippet(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	pages, _, err := loadPages(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		for _, example := range page.Examples {
			if strings.TrimSpace(example.Snippet) == "" {
				t.Errorf("%s/%s has no usage snippet", page.ID, example.ID)
			}
		}
	}
}

func TestUsageSnippetsAreEnglish(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	pages, _, err := loadPages(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		for _, example := range page.Examples {
			for _, r := range example.Snippet {
				if unicode.Is(unicode.Han, r) {
					t.Errorf("%s/%s usage snippet still contains Chinese text", page.ID, example.ID)
					break
				}
			}
		}
	}
}

func TestUsageSnippetsParseAsGo(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	pages, _, err := loadPages(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range pages {
		for _, example := range page.Examples {
			source := "package snippet\nfunc example() {\n" + example.Snippet + "\n}\n"
			if _, err := parser.ParseFile(token.NewFileSet(), "", source, 0); err != nil {
				t.Errorf("%s/%s usage snippet does not parse: %v", page.ID, example.ID, err)
			}
		}
	}
}

func TestManifestDeterministicAndTracksSourceNotTests(t *testing.T) {
	root := manifestFixture(t)
	first, err := sourceManifest(root)
	if err != nil {
		t.Fatal(err)
	}
	second, err := sourceManifest(root)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("unstable manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "components/button_test.go"), []byte("ignored"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err = sourceManifest(root)
	if err != nil || !reflect.DeepEqual(first, second) {
		t.Fatalf("test changed source manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "components/button.go"), []byte("changed"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err = sourceManifest(root)
	if err != nil || first.Revision == second.Revision {
		t.Fatalf("source mutation not detected: %v", err)
	}
	t.Run("RejectsSymlinkSource", func(t *testing.T) {
		docTestSymlink(t, filepath.Join(root, "go.mod"), filepath.Join(root, "components/link.go"))
		if _, err := sourceManifest(root); err == nil {
			t.Fatal("accepted symlink source")
		}
	})
}

func TestOutputBoundaryAndPublication(t *testing.T) {
	root := t.TempDir()
	for _, suffix := range []string{"build", "components", "../outside"} {
		if err := validateOutput(root, filepath.Join(root, suffix)); err == nil {
			t.Fatalf("accepted %s", suffix)
		}
	}
	output := filepath.Join(root, "build", "docs-data")
	if err := validateOutput(root, output); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(output, 0o755); err != nil {
		t.Fatal(err)
	}
	staging := filepath.Join(root, "build", "staging")
	if err := os.MkdirAll(staging, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "catalog.json"), []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publishData(staging, output); err == nil {
		t.Fatal("replaced unowned output")
	}
	if err := os.WriteFile(filepath.Join(output, "catalog.json"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := publishData(staging, output); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(output, "catalog.json"))
	if err != nil || string(data) != "new" {
		t.Fatalf("publication failed: %q, %v", data, err)
	}
	if _, err := os.Stat(staging + "-previous"); !os.IsNotExist(err) {
		t.Fatal("previous output leaked")
	}
	t.Run("RejectsSymlinkOutput", func(t *testing.T) {
		link := filepath.Join(root, "build", "link")
		docTestSymlink(t, output, link)
		if err := validateOutput(root, filepath.Join(link, "docs")); err == nil {
			t.Fatal("accepted symlink output")
		}
	})
}

func docTestSymlink(t *testing.T, target, path string) {
	t.Helper()
	if err := os.Symlink(target, path); err != nil {
		if runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)) {
			t.Skipf("Windows account lacks symbolic-link privilege: %v", err)
		}
		t.Fatal(err)
	}
}

func TestTargetEnvironmentOverridesNativeVariables(t *testing.T) {
	want := []string{"PATH=/bin", "OTHER=value", "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0"}
	got := targetEnvironment([]string{"GOOS=darwin", "PATH=/bin", "GOARCH=arm64", "CGO_ENABLED=1", "OTHER=value"})
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("target environment = %v", got)
	}
}

func TestRetainPreviousPreviewKeepsOnlyPublishedRevision(t *testing.T) {
	root := t.TempDir()
	output := filepath.Join(root, "output")
	staging := filepath.Join(root, "staging")
	for _, directory := range []string{filepath.Join(output, "public", "preview"), filepath.Join(staging, "public", "preview")} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(output, "catalog.json"), []byte(`{"revision":"old123"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "public", "preview", "preview-old123.wasm"), []byte("old wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "public", "preview", "preview-older.wasm"), []byte("older wasm"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := retainPreviousPreview(output, staging, "new456"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(staging, "public", "preview", "preview-old123.wasm"))
	if err != nil || string(data) != "old wasm" {
		t.Fatalf("previous preview was not retained: %q, %v", data, err)
	}
	if _, err := os.Stat(filepath.Join(staging, "public", "preview", "preview-older.wasm")); !os.IsNotExist(err) {
		t.Fatal("retained more than the one published revision")
	}
	if err := retainPreviousPreview(output, staging, "old123"); err != nil {
		t.Fatal(err)
	}
}

func TestFailedBuildPreservesPublishedPreview(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX command fixture")
	}
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "build"), 0o755); err != nil {
		t.Fatal(err)
	}
	directory, err := os.MkdirTemp(filepath.Join(root, "build"), "docs-failure-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(directory) })
	output := filepath.Join(directory, "published")
	if err := os.Mkdir(output, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "catalog.json"), []byte("previous-preview"), 0o644); err != nil {
		t.Fatal(err)
	}
	commands := t.TempDir()
	if err := os.WriteFile(filepath.Join(commands, "go"), []byte("#!/bin/sh\nexit 23\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", commands+string(os.PathListSeparator)+os.Getenv("PATH"))
	if err := run(output, false); err == nil || !strings.Contains(err.Error(), "compile shared Web preview") {
		t.Fatalf("expected build failure, got %v", err)
	}
	data, err := os.ReadFile(filepath.Join(output, "catalog.json"))
	if err != nil || string(data) != "previous-preview" {
		t.Fatalf("failed build changed published output: %q, %v", data, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("staging directory leaked: %v, %v", entries, err)
	}
}

// Every page-first example snippet is valid Go and stays on the page API: no
// canvas constructors or components package calls.
func TestPageSnippetsArePageLevel(t *testing.T) {
	for _, component := range componentdemo.Catalog() {
		for _, example := range component.Examples {
			for _, snippet := range []string{example.PageSnippet, example.PageSnippetZh} {
				if snippet == "" {
					continue
				}
				if strings.Contains(snippet, "components.") || strings.Contains(snippet, "Begin") {
					t.Errorf("%s/%s page snippet uses the canvas API: %q", component.ID, example.ID, snippet)
				}
				source := "package snippet\nfunc example() {\n" + snippet + "\n}\n"
				if _, err := parser.ParseFile(token.NewFileSet(), "snippet.go", source, 0); err != nil {
					t.Errorf("%s/%s page snippet does not parse: %v\n%s", component.ID, example.ID, err, snippet)
				}
			}
			if strings.Contains(example.PageSnippet, "components.") || strings.Contains(example.PageSnippet, "Begin") {
				t.Errorf("%s/%s page snippet uses the canvas API: %q", component.ID, example.ID, example.PageSnippet)
			}
			source := "package snippet\nfunc example() {\n" + example.PageSnippet + "\n}\n"
			if _, err := parser.ParseFile(token.NewFileSet(), "snippet.go", source, 0); err != nil {
				t.Errorf("%s/%s page snippet does not parse: %v\n%s", component.ID, example.ID, err, example.PageSnippet)
			}
		}
	}
}

// TestEveryExampleCarriesLocalizedPageSnippet keeps the preview and the code
// under it in one language and one authoring model: a canvas-only snippet is
// not a page call, and an English snippet under a Chinese preview is not the
// same example.
func TestEveryExampleCarriesLocalizedPageSnippet(t *testing.T) {
	for _, component := range componentdemo.Catalog() {
		for _, example := range component.Examples {
			if strings.TrimSpace(example.PageSnippet) == "" {
				t.Errorf("%s/%s has no page snippet", component.ID, example.ID)
			}
			if strings.TrimSpace(example.PageSnippetZh) == "" {
				t.Errorf("%s/%s has no Chinese page snippet", component.ID, example.ID)
			}
		}
	}
}

// TestPageUsageIsPageLevel parses every component-level PageUsage and rejects
// the canvas API, so the line above the examples is a real call site too.
func TestPageUsageIsPageLevel(t *testing.T) {
	for _, component := range componentdemo.Catalog() {
		usage := component.PageUsage
		if usage == "" {
			continue
		}
		if strings.Contains(usage, "components.") || strings.Contains(usage, "Begin") || strings.Contains(usage, "Paint(") {
			t.Errorf("%s PageUsage uses the canvas API: %q", component.ID, usage)
		}
		source := "package snippet\nfunc example(c ui.C, m model) {\n" + usage + "\n}\n"
		if _, err := parser.ParseFile(token.NewFileSet(), "usage.go", source, 0); err != nil {
			t.Errorf("%s PageUsage does not parse: %v\n%s", component.ID, err, usage)
		}
	}
}

// TestPageSnippetsHaveNoUndeclaredNames rejects a snippet that reads a name the
// page function does not declare. Model data must be written m.field, so the
// snippet shows where it comes from; a bare `highlight` or `desktop` is a
// canvas-side local that would not compile at a call site.
func TestPageSnippetsHaveNoUndeclaredNames(t *testing.T) {
	packages := map[string]bool{
		"ui": true, "base": true, "gfx": true, "window": true, "text": true,
		"style": true, "strings": true, "utf8": true, "time": true, "math": true,
		"codeeditor": true, "chartview": true, "calendarview": true,
		"carouselview": true, "multiselect": true, "hierarchypicker": true,
		"markdownview": true, "columnresizeview": true,
		"hovercardview":       true,
		"sidebarview":         true,
		"markerview":          true,
		"metriccardview":      true,
		"questionnaireview":   true,
		"inputotpview":        true,
		"inputgroupview":      true,
		"skeletonview":        true,
		"aspectratioview":     true,
		"imageview":           true,
		"navigationmenuview":  true,
		"menubarview":         true,
		"commandview":         true,
		"messagescrollerview": true,
		"messageview":         true,
		"attachmentview":      true,
		"avatarview":          true,
		"avatareditorview":    true, "reordergridview": true,
	}
	builtin := map[string]bool{
		"bool": true, "byte": true, "complex64": true, "complex128": true,
		"error": true, "float32": true, "float64": true, "int": true, "int8": true,
		"int16": true, "int32": true, "int64": true, "rune": true, "string": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"uintptr": true, "any": true, "comparable": true,
		"true": true, "false": true, "nil": true, "iota": true,
		"append": true, "cap": true, "clear": true, "close": true, "complex": true,
		"copy": true, "delete": true, "imag": true, "len": true, "make": true,
		"max": true, "min": true, "new": true, "panic": true, "print": true,
		"println": true, "real": true, "recover": true,
	}
	for _, component := range componentdemo.Catalog() {
		for _, example := range component.Examples {
			for _, snippet := range []string{example.PageSnippet, example.PageSnippetZh} {
				if snippet == "" {
					continue
				}
				source := "package snippet\nfunc example(c ui.C, m model) {\n" + snippet + "\n}\n"
				file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", source, 0)
				if err != nil {
					continue // the parse test reports it
				}
				for _, ident := range file.Unresolved {
					if packages[ident.Name] || builtin[ident.Name] || ident.Name == "model" {
						continue
					}
					t.Errorf("%s/%s page snippet reads undeclared %q; write model data as m.%s", component.ID, example.ID, ident.Name, ident.Name)
				}
			}
		}
	}
}

// TestPageSnippetsUseCallerNames rejects a snippet that reads a canvas-side
// local: the snippet is shown inside a page function, where `theme`, `canvas`,
// `bounds`, `stage`, `page` and `tree` are not defined. A page reads its theme
// through c.Theme().
func TestPageSnippetsUseCallerNames(t *testing.T) {
	forbidden := map[string]bool{"theme": true, "canvas": true, "bounds": true, "stage": true, "page": true, "tree": true}
	for _, component := range componentdemo.Catalog() {
		for _, example := range component.Examples {
			for _, snippet := range []string{example.PageSnippet, example.PageSnippetZh} {
				if snippet == "" {
					continue
				}
				source := "package snippet\nfunc example() {\n" + snippet + "\n}\n"
				file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", source, 0)
				if err != nil {
					continue // the parse test reports it
				}
				for _, ident := range file.Unresolved {
					if forbidden[ident.Name] {
						t.Errorf("%s/%s page snippet uses the canvas-side name %q", component.ID, example.ID, ident.Name)
					}
				}
			}
		}
	}
}

// TestPageSnippetsNameRealAPIs rejects a snippet that calls a package function
// the package does not declare. It does not type-check arguments: a snippet
// leaves the application's model implicit, so its types are not known here.
func TestPageSnippetsNameRealAPIs(t *testing.T) {
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	packages := map[string]map[string]bool{
		"ui":                  exports(t, filepath.Join(root, "ui")),
		"base":                exports(t, filepath.Join(root, "base")),
		"components":          exports(t, filepath.Join(root, "components")),
		"gfx":                 exports(t, filepath.Join(root, "gfx")),
		"window":              exports(t, filepath.Join(root, "window")),
		"text":                exports(t, filepath.Join(root, "text")),
		"codeeditor":          exports(t, filepath.Join(root, "ui/codeeditor")),
		"chartview":           exports(t, filepath.Join(root, "ui/chartview")),
		"calendarview":        exports(t, filepath.Join(root, "ui/calendarview")),
		"carouselview":        exports(t, filepath.Join(root, "ui/carouselview")),
		"multiselect":         exports(t, filepath.Join(root, "ui/multiselect")),
		"hierarchypicker":     exports(t, filepath.Join(root, "ui/hierarchypicker")),
		"markdownview":        exports(t, filepath.Join(root, "ui/markdownview")),
		"columnresizeview":    exports(t, filepath.Join(root, "ui/columnresizeview")),
		"avatareditorview":    exports(t, filepath.Join(root, "ui/avatareditorview")),
		"reordergridview":     exports(t, filepath.Join(root, "ui/reordergridview")),
		"avatarview":          exports(t, filepath.Join(root, "ui/avatarview")),
		"attachmentview":      exports(t, filepath.Join(root, "ui/attachmentview")),
		"messageview":         exports(t, filepath.Join(root, "ui/messageview")),
		"messagescrollerview": exports(t, filepath.Join(root, "ui/messagescrollerview")),
		"commandview":         exports(t, filepath.Join(root, "ui/commandview")),
		"menubarview":         exports(t, filepath.Join(root, "ui/menubarview")),
		"navigationmenuview":  exports(t, filepath.Join(root, "ui/navigationmenuview")),
		"hovercardview":       exports(t, filepath.Join(root, "ui/hovercardview")),
	}
	for _, component := range componentdemo.Catalog() {
		for _, example := range component.Examples {
			for _, snippet := range []string{example.PageSnippet, example.PageSnippetZh} {
				if snippet == "" {
					continue
				}
				source := "package snippet\nfunc example() {\n" + snippet + "\n}\n"
				file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", source, 0)
				if err != nil {
					continue // the parse test reports it
				}
				ast.Inspect(file, func(node ast.Node) bool {
					selector, ok := node.(*ast.SelectorExpr)
					if !ok {
						return true
					}
					pkg, ok := selector.X.(*ast.Ident)
					if !ok {
						return true
					}
					names, ok := packages[pkg.Name]
					if !ok {
						return true // the model or a local value
					}
					if !names[selector.Sel.Name] {
						t.Errorf("%s/%s page snippet calls %s.%s, which the package does not declare", component.ID, example.ID, pkg.Name, selector.Sel.Name)
					}
					return true
				})
			}
		}
	}
}

// exports is the exported top-level names of one package directory.
func exports(t *testing.T, dir string) map[string]bool {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	set := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Recv == nil && declaration.Name.IsExported() {
					names[declaration.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							names[spec.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, ident := range spec.Names {
							if ident.IsExported() {
								names[ident.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return names
}
