package main

import (
	"bytes"
	"encoding/json"
	"errors"
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
		if !strings.Contains(example.Snippet, "f.Button") {
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
