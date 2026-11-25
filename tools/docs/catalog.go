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
	"regexp"
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
	PageSnippet            string `json:"PageSnippet,omitempty"`
	PageSnippetZh          string `json:"PageSnippetZh,omitempty"`
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
	// PageUsage is the page-first call site, when the component has one.
	PageUsage string
	// PageSource is where PageUsage is implemented; empty uses Source.
	PageSource string
	// Source is the Go file the examples call, relative to the repository root:
	// components/<ID>.go, or ui/list.go for a page-layer control.
	Source string
	// API lists the symbols the component page summarizes, as "package.Name".
	// The page renders them from api.json, so the tables cannot drift from the
	// code they document.
	API            []string `json:"api,omitempty"`
	All            []Nav
	Examples       []Demo
	InstallCommand string
	Sections       []Section
	Capabilities   []Capability
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

// componentAPI maps a component page to the API symbols it summarizes. Pilot:
// the toast pages. Sites keep this list explicit because a page often calls
// symbols outside its own recipe file: the toast page's page API lives in base.
// pageAPIHelpers are the page-layout and measurement calls every example uses;
// they are not the component's own API and stay out of its table.
var pageAPIHelpers = map[string]bool{
	"Row": true, "Column": true, "Text": true, "Textf": true, "Grid": true,
	"Box": true, "Slot": true, "Spacer": true, "Children": true,
	"If": true, "For": true, "Start": true, "Center": true, "End": true,
	"Stretch": true, "Baseline": true, "Wrap": true, "Gap": true, "Pad": true,
	"PadX": true, "PadY": true, "Margin": true, "MarginY": true, "Width": true,
	"Height": true, "Size": true, "Fill": true, "FillWidth": true, "Shrink": true,
	"Grow": true, "Self": true, "Align": true, "Justify": true, "Position": true,
	"Padding": true, "ID": true, "C": true, "Theme": true,
	"DrawContext": true,
}

var apiSymbolPattern = regexp.MustCompile(`\b([a-z][a-z0-9]*)\.([A-Z][A-Za-z0-9_]*)`)

// derivePageAPI is the first pass for a component page without an explicit
// list: the page symbols its examples call, then the recipe symbols their
// canvas snippets call. Unknown names are dropped against api.json later.
func derivePageAPI(examples []Demo, pagePackages map[string]bool) []string {
	var symbols []string
	seen := map[string]bool{}
	add := func(symbol string) {
		if !seen[symbol] {
			seen[symbol] = true
			symbols = append(symbols, symbol)
		}
	}
	for _, example := range examples {
		for _, snippet := range []string{example.PageSnippet, example.PageSnippetZh} {
			for _, match := range apiSymbolPattern.FindAllStringSubmatch(snippet, -1) {
				pkg, name := match[1], match[2]
				if pkg == "components" {
					continue
				}
				if pkg != "ui" && !pagePackages[pkg] {
					continue
				}
				if pkg == "ui" && pageAPIHelpers[name] {
					continue
				}
				add(pkg + "." + name)
			}
		}
	}
	for _, example := range examples {
		for _, match := range apiSymbolPattern.FindAllStringSubmatch(example.Snippet, -1) {
			if match[1] == "components" {
				add("components." + match[2])
			}
		}
	}
	return symbols
}

// componentAPIExtra adds a page API the example snippets do not name, such as
// a control built through a local variable.
var componentAPIExtra = map[string][]string{
	"toggle-group": {"togglegroup.Group"},
}

var componentAPI = map[string][]string{
	"toast": {
		"ui.Toaster", "ui.ToastConfig", "ui.Toasts",
		"components.PaintToast", "components.ToastOptions", "components.ToastSnapshot",
	},
	"sonner": {
		"ui.Toaster", "ui.ToastConfig", "ui.Toasts",
		"components.PaintSonner", "components.SonnerOptions", "components.SonnerToast",
	},
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
	paths, err := apiPackagePaths(root)
	if err != nil {
		return nil, manifest, err
	}
	pagePackages := map[string]bool{"ui": true}
	for _, path := range paths {
		if strings.HasPrefix(path, "ui/") {
			pagePackages[strings.TrimPrefix(path, "ui/")] = true
		}
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
		page.Source = component.Source
		page.PageUsage = component.PageUsage
		page.PageSource = component.PageSource
		if page.Source == "" {
			page.Source = "components/" + strings.ReplaceAll(component.ID, "-", "_") + ".go"
		}
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
			page.Examples = append(page.Examples, Demo{example.ID, example.Title, example.Description, example.Snippet, example.PageSnippet, example.PageSnippetZh, example.Height})
		}
		if explicit, ok := componentAPI[component.ID]; ok {
			page.API = explicit
		} else {
			page.API = derivePageAPI(page.Examples, pagePackages)
		}
		page.API = append(page.API, componentAPIExtra[component.ID]...)
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
		// The files that belong to another component's page: its layout and the
		// bars its examples show.
		if file.Name() == "doc.go" || file.Name() == "direction.go" ||
			file.Name() == "menu_typeahead.go" || file.Name() == "code_diff_model.go" || file.Name() == "code_diff_inline.go" ||
			file.Name() == "code_editor_layout.go" || file.Name() == "code_editor_decorations.go" || file.Name() == "code_editor_shaped.go" || file.Name() == "caret_blink.go" || file.Name() == "find_bar.go" || file.Name() == "go_to_line.go" || file.Name() == "hierarchy_menu.go" ||
			file.Name() == "graph_sim.go" || file.Name() == "markdown_anchor.go" || file.Name() == "text_selection.go" {
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
	for _, directory := range []string{"app", "ui", "base", "gfx", "components", "internal", "cmd/component-preview", "cmd/viva/templates", "tools/docs"} {
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
	api, err := loadAPI(root)
	if err != nil {
		return err
	}
	if err := validateAPI(api); err != nil {
		return err
	}
	prunePageAPI(pages, api)
	if err := validatePageAPI(pages, api); err != nil {
		return err
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
