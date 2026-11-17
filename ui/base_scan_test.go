package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"
)

// comparedWithTheirStyledForm are the page Bases whose styled control is compared with them
// in TestStyledControlsDeclareWhatTheirBasesDeclare. Adding a Base means adding a row there
// and its name here, or, when no styled control is built from it, to withoutStyledForm.
var comparedWithTheirStyledForm = []string{
	"ButtonBase", "CheckboxBase", "SwitchBase", "SelectBase", "SliderBase",
	"TextFieldBase", "TabBase", "RadioBase",
}

// withoutStyledForm are the Bases that are not the Base of a control that ships with a look,
// so there is nothing to compare them with. ToggleBase has no Toggle on the page.
var withoutStyledForm = []string{"ToggleBase"}

// A Base of a page is declared in a file that also declares the control it undresses. This
// test makes a new one say which of the two lists it belongs to, so a Base cannot be added
// without deciding whether it must stay the same control as the one with a look.
func TestEveryPageBaseIsComparedOrExplained(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	var found []string
	set := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(set, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !fn.Name.IsExported() || !strings.HasSuffix(fn.Name.Name, "Base") {
				continue
			}
			// A page element takes the page's C first; the frame's own constructors are methods.
			if params := fn.Type.Params.List; len(params) > 0 {
				if ident, ok := params[0].Type.(*ast.Ident); ok && ident.Name == "C" {
					found = append(found, fn.Name.Name)
				}
			}
		}
	}
	known := append(slices.Clone(comparedWithTheirStyledForm), withoutStyledForm...)
	for _, name := range found {
		if !slices.Contains(known, name) {
			t.Errorf("%s is a page Base that no test compares with its styled control: add it to TestStyledControlsDeclareWhatTheirBasesDeclare and comparedWithTheirStyledForm, or to withoutStyledForm if it has none", name)
		}
	}
	for _, name := range known {
		if !slices.Contains(found, name) {
			t.Errorf("%s is listed in base_scan_test.go but is not a page Base any more: remove it", name)
		}
	}
}
