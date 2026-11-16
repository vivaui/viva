package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// The agent-facing documents are read by a coding assistant that cannot run the
// compiler first. These tests keep them tied to the code: a name they show must
// exist, a link must resolve, and a quoted panic must be one the framework
// really raises. They are deliberately one-directional: they do not require the
// documents to mention every symbol, only that what they do mention is real.

var agentDocuments = []string{
	"initfiles/AGENTS.md.txt",
	"../../docs/skills/building-with-viva/SKILL.md",
	"../../docs/skills/building-with-viva/recipes.md",
	"../../docs/README.md",
}

var agentPackages = map[string]string{
	"ui":         "../../ui",
	"gfx":        "../../gfx",
	"app":        "../../app",
	"components": "../../components",
	"headless":   "../../headless",
	"field":      "../../field",
	"text":       "../../text",
}

// Names the documents mention to say they do not exist.
var knownAbsent = map[string]bool{
	"gfx.Handled": true,
}

var qualified = regexp.MustCompile(`\b(ui|gfx|app|components|headless|field|text)\.([A-Z][A-Za-z0-9_]*)\b`)

func TestAgentDocsNameRealSymbols(t *testing.T) {
	symbols := map[string]map[string]bool{}
	for name, dir := range agentPackages {
		set, err := exportedTopLevel(dir)
		if err != nil {
			t.Fatal(err)
		}
		symbols[name] = set
	}
	for _, document := range agentDocuments {
		text := readDocument(t, document)
		for _, match := range qualified.FindAllStringSubmatch(text, -1) {
			reference := match[0]
			if knownAbsent[reference] {
				continue
			}
			if symbols[match[1]][match[2]] {
				continue
			}
			t.Errorf("%s names %s, which %s does not export", document, reference, match[1])
		}
	}
}

func exportedTopLevel(dir string) (map[string]bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	set := map[string]bool{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, declaration := range file.Decls {
			switch d := declaration.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.IsExported() {
					set[d.Name.Name] = true
				}
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if s.Name.IsExported() {
							set[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, ident := range s.Names {
							if ident.IsExported() {
								set[ident.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return set, nil
}

func TestAgentDocsLinksResolve(t *testing.T) {
	link := regexp.MustCompile(`\]\(([^)]+)\)`)
	for _, document := range agentDocuments {
		text := readDocument(t, document)
		dir := filepath.Dir(document)
		for _, match := range link.FindAllStringSubmatch(text, -1) {
			target := strings.TrimSpace(match[1])
			if target == "" || strings.HasPrefix(target, "#") || strings.Contains(target, "://") || strings.HasPrefix(target, "mailto:") {
				continue
			}
			// A type-parameter or call in a code span also reads as "](", in
			// ui.State[bool](c, "open"); a real link target has no spaces and
			// names a path or a file.
			if strings.ContainsAny(target, " \t\"") {
				continue
			}
			if !strings.ContainsAny(target, "./") {
				continue
			}
			if filepath.Base(document) == "AGENTS.md.txt" {
				t.Errorf("%s links %q; it is installed alone, so use an absolute URL", document, target)
				continue
			}
			path := strings.SplitN(target, "#", 2)[0]
			if _, err := os.Stat(filepath.Join(dir, path)); err != nil {
				t.Errorf("%s links %q, which does not exist", document, target)
			}
		}
	}
}

// The guide's repair table is the assistant's path out of a failed compile or a
// panic. A row that quotes a message the framework does not raise sends it the
// wrong way, so every quoted ui: message must appear in a panic literal.
func TestAgentGuideQuotesRealPanics(t *testing.T) {
	guide := readDocument(t, "initfiles/AGENTS.md.txt")
	start := strings.Index(guide, "## If it does not compile or panics")
	if start < 0 {
		t.Fatal("the guide has no error table")
	}
	section := guide[start:]
	if end := strings.Index(section[1:], "\n## "); end >= 0 {
		section = section[:1+end]
	}
	var sources strings.Builder
	entries, err := os.ReadDir("../../ui")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		data, err := os.ReadFile(filepath.Join("../../ui", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sources.Write(data)
	}
	for _, match := range regexp.MustCompile("`(ui: [^`]+)`").FindAllStringSubmatch(section, -1) {
		if !strings.Contains(sources.String(), match[1]) {
			t.Errorf("the error table quotes %q, which no ui panic contains", match[1])
		}
	}
}

func TestAgentGuideIsScannable(t *testing.T) {
	guide := readDocument(t, "initfiles/AGENTS.md.txt")
	headings := map[string]int{}
	inFence := false
	for number, line := range strings.Split(guide, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
			headings[line]++
		}
		if len(line) > 1500 && !strings.HasPrefix(line, "|") && !strings.Contains(line, "://") {
			t.Errorf("line %d is %d characters; split it into a subsection or a bullet", number+1, len(line))
		}
	}
	var repeated []string
	for heading, count := range headings {
		if count > 1 {
			repeated = append(repeated, heading)
		}
	}
	sort.Strings(repeated)
	if len(repeated) > 0 {
		t.Errorf("headings repeat, so a cross-reference is ambiguous: %s", strings.Join(repeated, ", "))
	}
}

func readDocument(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
