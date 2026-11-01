package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vivaui/viva/internal/componentdemo"
)

type Nav struct {
	ID, Title, Category string
}

type Demo struct {
	ID, Title, Description string
	Snippet                string `json:"Snippet,omitempty"`
	Height                 int    `json:"Height,omitempty"`
}

type Section struct {
	ID, Title, Text string
	Items           []string
}

type Capability struct {
	Platform, Status, Detail string
}

type Page struct {
	ID, Title, Category, Summary, Revision string
	All                                    []Nav
	Examples                               []Demo
	InstallCommand                         string
	Sections                               []Section
	Capabilities                           []Capability
}

type SourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type Coverage struct {
	ID         string `json:"id"`
	File       string `json:"file"`
	Documented bool   `json:"documented"`
}

type Manifest struct {
	Revision string       `json:"revision"`
	Files    []SourceFile `json:"files"`
	Coverage []Coverage   `json:"coverage"`
}

func loadPages(root string) ([]Page, Manifest, error) {
	manifest, err := sourceManifest(root)
	if err != nil {
		return nil, manifest, err
	}
	var navigation []Nav
	seen := make(map[string]bool)
	for _, component := range componentdemo.Catalog() {
		if !validID(component.ID) || seen[component.ID] || len(component.Examples) == 0 {
			return nil, manifest, fmt.Errorf("invalid or duplicate component %q", component.ID)
		}
		seen[component.ID] = true
		navigation = append(navigation, Nav{component.ID, component.Title, component.Category})
	}
	metadata := documentation()
	var pages []Page
	for _, component := range componentdemo.Catalog() {
		page, exists := metadata[component.ID]
		if !exists {
			return nil, manifest, fmt.Errorf("component %s lacks documentation", component.ID)
		}
		page.ID, page.Title, page.Category, page.Summary = component.ID, component.Title, component.Category, component.Summary
		page.All, page.Revision = navigation, manifest.Revision
		page.InstallCommand = installCommand(root, component.ID)
		exampleIDs := make(map[string]bool)
		for _, example := range component.Examples {
			if !validID(example.ID) || exampleIDs[example.ID] {
				return nil, manifest, fmt.Errorf("invalid or duplicate example %s/%s", component.ID, example.ID)
			}
			exampleIDs[example.ID] = true
			if _, err := componentdemo.New(component.ID, example.ID); err != nil {
				return nil, manifest, err
			}
			page.Examples = append(page.Examples, Demo{example.ID, example.Title, example.Description, example.Snippet, example.Height})
		}
		pages = append(pages, page)
	}
	if len(metadata) != len(pages) {
		return nil, manifest, errors.New("documentation contains an orphan component")
	}
	files, err := os.ReadDir(filepath.Join(root, "components"))
	if err != nil {
		return nil, manifest, err
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") || strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		if file.Name() == "doc.go" || file.Name() == "direction.go" ||
			file.Name() == "menu_typeahead.go" || file.Name() == "code_diff_model.go" {
			continue
		}
		id := strings.ReplaceAll(strings.TrimSuffix(file.Name(), ".go"), "_", "-")
		documented := seen[id]
		manifest.Coverage = append(manifest.Coverage, Coverage{ID: id, File: "components/" + file.Name(), Documented: documented})
		if !documented {
			return nil, manifest, fmt.Errorf("source component %s lacks a documentation preview", id)
		}
	}
	return pages, manifest, nil
}

// installCommand is the viva add command for a component, or empty when the
// component is not a recipe of its own: every file of the components package
// is installable, so a page has one exactly when its source file exists.
func installCommand(root, component string) string {
	file := strings.ReplaceAll(component, "-", "_") + ".go"
	if info, err := os.Stat(filepath.Join(root, "components", file)); err != nil || info.IsDir() {
		return ""
	}
	return "go run ./cmd/viva add --out ./components/local " + component
}

func validID(value string) bool {
	if value == "" {
		return false
	}
	for _, character := range value {
		if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
			return false
		}
	}
	return true
}

func sourceManifest(root string) (Manifest, error) {
	manifest := Manifest{}
	for _, directory := range []string{"app", "ui", "components", "internal", "cmd/component-preview", "cmd/viva/templates", "tools/docs"} {
		err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(directory)), func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || strings.HasSuffix(entry.Name(), "_test.go") {
				return nil
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("source must not be a symlink: %s", path)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			digest := sha256.Sum256(data)
			manifest.Files = append(manifest.Files, SourceFile{filepath.ToSlash(relative), hex.EncodeToString(digest[:])})
			return nil
		})
		if err != nil {
			return manifest, err
		}
	}
	for _, path := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return manifest, err
		}
		digest := sha256.Sum256(data)
		manifest.Files = append(manifest.Files, SourceFile{path, hex.EncodeToString(digest[:])})
	}
	sort.Slice(manifest.Files, func(left, right int) bool { return manifest.Files[left].Path < manifest.Files[right].Path })
	hash := sha256.New()
	for _, file := range manifest.Files {
		fmt.Fprintf(hash, "%s\x00%s\n", file.Path, file.SHA256)
	}
	manifest.Revision = hex.EncodeToString(hash.Sum(nil))[:12]
	return manifest, nil
}

func writeData(root, output string, pages []Page, manifest Manifest) error {
	write := func(path string, data []byte) error {
		path = filepath.Join(output, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	}
	catalog := struct {
		Pages    []Page     `json:"pages"`
		Revision string     `json:"revision"`
		Coverage []Coverage `json:"coverage"`
	}{pages, manifest.Revision, manifest.Coverage}
	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return err
	}
	if err := write("catalog.json", data); err != nil {
		return err
	}
	api, err := loadAPI(root)
	if err != nil {
		return err
	}
	if err := validateAPI(api); err != nil {
		return err
	}
	data, err = json.MarshalIndent(api, "", "  ")
	if err != nil {
		return err
	}
	if err := write("api.json", data); err != nil {
		return err
	}
	entries, err := fs.ReadDir(assets, "preview")
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := assets.ReadFile("preview/" + entry.Name())
		if err != nil {
			return err
		}
		data = []byte(strings.ReplaceAll(string(data), "__VIVA_REVISION__", manifest.Revision))
		if err := write("public/preview/"+entry.Name(), data); err != nil {
			return err
		}
	}
	for _, file := range manifest.Files {
		if !strings.HasPrefix(file.Path, "components/") && !strings.HasPrefix(file.Path, "internal/componentdemo/") && !strings.HasPrefix(file.Path, "cmd/component-preview/") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
		if err != nil {
			return err
		}
		if err := write("public/sources/"+file.Path, data); err != nil {
			return err
		}
	}
	// The copies above import the repository's own packages, so left bare they are
	// Go packages of the repository whenever the output sits inside it (it does,
	// under build/): "go vet ./..." would compile them a second time. A go.mod
	// makes the directory a module of its own, which "./..." skips.
	if err := write("public/sources/go.mod", []byte(sourcesModule)); err != nil {
		return err
	}
	data, err = json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	if err := write("public/source-manifest.json", data); err != nil {
		return err
	}
	coverage, err := json.MarshalIndent(manifest.Coverage, "", "  ")
	if err != nil {
		return err
	}
	return write("public/coverage.json", coverage)
}

// sourcesModule is the go.mod written beside the published source copies.
const sourcesModule = "// Copies of the repository's component sources, published for download.\n" +
	"// This file keeps them out of the repository's own module.\nmodule viva.invalid/docs-sources\n\ngo 1.26\n"
