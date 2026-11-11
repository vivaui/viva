package ui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// The guide viva init writes into every new project is what a coding assistant reads
// before it writes a page. A method it does not mention is one the assistant will
// reinvent with coordinates, so every exported method of Box and the public state
// types must appear in it. Methods of the unexported engine are not application API.
func TestTheGuideMentionsEveryMethod(t *testing.T) {
	guide, err := os.ReadFile("../cmd/viva/initfiles/AGENTS.md.txt")
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	var missing []string
	for _, file := range []string{"frame.go", "box.go", "absolute.go", "grid.go", "popup.go", "text_wrap.go", "table.go", "tabs.go", "select.go", "dialog.go", "leaves.go", "splitter.go", "field.go", "slider.go", "scroll.go", "bases.go", "draw.go"} {
		parsed, err := parser.ParseFile(set, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range parsed.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv == nil || !function.Name.IsExported() {
				continue
			}
			if len(function.Recv.List) != 1 || !ast.IsExported(receiverName(function.Recv.List[0].Type)) {
				continue // the unexported engine (frame, table, dialog, ...) is not application API
			}
			switch function.Name.Name {
			case "Canvas", "Theme", "Begin", "End", "Rect", "Children", "Content", "Clicked":
				continue // plumbing the guide shows in context rather than lists
			}
			if !strings.Contains(string(guide), function.Name.Name) {
				missing = append(missing, function.Name.Name)
			}
		}
	}
	if len(missing) > 0 {
		t.Errorf("AGENTS.md.txt never mentions: %s", strings.Join(missing, ", "))
	}
}

// receiverName is the type a method is declared on, through a pointer or a type
// parameter: "Box" for func (b Box) ..., "Frame" for func (f *Frame) ...
func receiverName(expr ast.Expr) string {
	for {
		switch typed := expr.(type) {
		case *ast.StarExpr:
			expr = typed.X
		case *ast.IndexExpr:
			expr = typed.X
		case *ast.IndexListExpr:
			expr = typed.X
		case *ast.ParenExpr:
			expr = typed.X
		case *ast.Ident:
			return typed.Name
		default:
			return ""
		}
	}
}

// A guide that names a theme color that does not exist teaches a coding assistant to
// write code that does not compile, and one that omits a real color hides it: the
// theme's fields and the guide's list must agree.
func TestTheGuideListsTheThemeFieldsAndOnlyThose(t *testing.T) {
	guide, err := os.ReadFile("../cmd/viva/initfiles/AGENTS.md.txt")
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, "../gfx/theme.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fields []string
	ast.Inspect(parsed, func(node ast.Node) bool {
		spec, ok := node.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "Theme" {
			return true
		}
		for _, field := range spec.Type.(*ast.StructType).Fields.List {
			for _, name := range field.Names {
				fields = append(fields, name.Name)
			}
		}
		return false
	})
	if len(fields) < 20 {
		t.Fatalf("found %d theme fields: %v", len(fields), fields)
	}
	for _, name := range fields {
		if name == "Metrics" || name == "Transition" {
			continue
		}
		if !strings.Contains(string(guide), "`"+name+"`") {
			t.Errorf("the guide does not list the theme color %s", name)
		}
	}
}
