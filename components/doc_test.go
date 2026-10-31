package components

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"regexp"
	"strings"
	"testing"
)

// docPrimaryEntries maps component files whose entry points do not follow the
// Begin*/Paint* naming convention to their cataloged entry functions. Extend
// this map when adding such a component; the test below fails until doc.go
// mentions the new entry.
var docPrimaryEntries = map[string][]string{
	"alert.go":            {"Alert"},
	"aspect_ratio.go":     {"AspectRatio"},
	"avatar.go":           {"Avatar"},
	"bubble.go":           {"Bubble", "BubbleReaction"},
	"card.go":             {"Card"},
	"chart_downsample.go": {"ChartDownsample"},
	"code_editor.go":      {"CodeEditor"},
	"code_diff.go":        {"CodeDiff"},
	"code_diff_model.go":  {"DiffCodeLines"},
	"direction.go":        {"Direction"},
	"empty.go":            {"Empty"},
	"kbd.go":              {"Kbd"},
	"markdown.go":         {"MarkdownView"},
	"marker.go":           {"Marker"},
	"menu_typeahead.go":   {"MenuTypeahead"},
	"message.go":          {"Message"},
	"metric_card.go":      {"MetricCard"},
	"progress.go":         {"Progress"},
	"typography.go":       {"Typography"},
}

// TestDocCatalogCoversEveryComponent keeps the package catalog in doc.go in
// sync with the component files: every exported Begin*/Paint* entry and every
// curated primary entry must be mentioned there, and doc.go must not mention
// an exported name that no longer exists.
func TestDocCatalogCoversEveryComponent(t *testing.T) {
	doc, err := os.ReadFile("doc.go")
	if err != nil {
		t.Fatalf("read doc.go: %v", err)
	}
	catalog := string(doc)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package directory: %v", err)
	}
	fset := token.NewFileSet()
	exported := make(map[string]bool)
	required := make(map[string]string)
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || name == "doc.go" {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		hasEntry := false
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv != nil || !decl.Name.IsExported() {
					continue
				}
				fn := decl.Name.Name
				exported[fn] = true
				if strings.HasPrefix(fn, "Begin") || strings.HasPrefix(fn, "Paint") {
					required[fn] = name
					hasEntry = true
				}
			case *ast.GenDecl:
				for _, spec := range decl.Specs {
					switch spec := spec.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							exported[spec.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, ident := range spec.Names {
							if ident.IsExported() {
								exported[ident.Name] = true
							}
						}
					}
				}
			}
		}
		for _, primary := range docPrimaryEntries[name] {
			required[primary] = name
			hasEntry = true
		}
		if !hasEntry {
			t.Errorf("%s exports no Begin*/Paint* entry and has no docPrimaryEntries mapping; add its entry to the doc.go catalog and the map in doc_test.go", name)
		}
	}
	for fn, file := range required {
		if !strings.Contains(catalog, "`"+fn+"`") {
			t.Errorf("doc.go catalog does not mention `%s` (entry of %s)", fn, file)
		}
	}

	identifiers := regexp.MustCompile("`([A-Z][A-Za-z0-9]*)`")
	for _, match := range identifiers.FindAllStringSubmatch(catalog, -1) {
		if !exported[match[1]] {
			t.Errorf("doc.go mentions `%s` which is not exported by this package", match[1])
		}
	}
}
