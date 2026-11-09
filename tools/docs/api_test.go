package main

import (
	"go/ast"
	"reflect"
	"strings"
	"testing"
)

func loadTestAPI(t *testing.T) APIData {
	t.Helper()
	root, err := findRoot()
	if err != nil {
		t.Fatal(err)
	}
	data, err := loadAPI(root)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func findType(t *testing.T, data APIData, pkg, name string) APIType {
	t.Helper()
	for _, p := range data.Packages {
		if p.Name != pkg {
			continue
		}
		for _, typ := range p.Types {
			if typ.Name == name {
				return typ
			}
		}
	}
	t.Fatalf("%s.%s is not documented", pkg, name)
	return APIType{}
}

func TestAPIDocumentsTheRealExportedSurface(t *testing.T) {
	data := loadTestAPI(t)
	if len(data.Packages) != len(apiPackages) {
		t.Fatalf("packages = %d, want %d", len(data.Packages), len(apiPackages))
	}
	config := findType(t, data, "app", "Config")
	for _, want := range []string{"Paint func(canvas *gfx.Canvas, size gfx.Size)", "OnPointer func(event gfx.PointerEvent) gfx.EventResult", "Invalidator *Invalidator"} {
		if !strings.Contains(config.Decl, want) {
			t.Errorf("app.Config declaration lacks %q:\n%s", want, config.Decl)
		}
	}
	if !strings.Contains(config.Decl, "// Paint, when set") {
		t.Error("field doc comments are missing from the declaration")
	}
	button := findType(t, data, "components", "Button")
	constructors := ""
	for _, constructor := range button.Constructors {
		constructors += constructor.Name + " "
	}
	if !strings.Contains(constructors, "BeginButton") {
		t.Errorf("components.Button constructors = %q", constructors)
	}
	methods := ""
	for _, method := range button.Methods {
		methods += method.Name + " "
		if method.Receiver != "Button" {
			t.Errorf("method %s receiver = %q", method.Name, method.Receiver)
		}
	}
	if !strings.Contains(methods, "Outline") || !strings.Contains(methods, "SizeToFit") {
		t.Errorf("components.Button methods = %q", methods)
	}
}

func TestAPIExposesNothingUnexported(t *testing.T) {
	data := loadTestAPI(t)
	for _, pkg := range data.Packages {
		if strings.TrimSpace(pkg.Synopsis) == "" {
			t.Errorf("package %s has no synopsis", pkg.Name)
		}
		for _, function := range pkg.Functions {
			if !ast.IsExported(function.Name) {
				t.Errorf("%s.%s is not exported", pkg.Name, function.Name)
			}
		}
		for _, typ := range pkg.Types {
			if !ast.IsExported(typ.Name) {
				t.Errorf("%s.%s is not exported", pkg.Name, typ.Name)
			}
			for _, method := range typ.Methods {
				if !ast.IsExported(method.Name) {
					t.Errorf("%s.%s.%s is not exported", pkg.Name, typ.Name, method.Name)
				}
			}
		}
	}
}

func TestAPIIsDeterministic(t *testing.T) {
	first, second := loadTestAPI(t), loadTestAPI(t)
	if !reflect.DeepEqual(first, second) {
		t.Fatal("two runs produced different API data")
	}
}

func TestDocLinksResolveToDocumentedSymbolsOnly(t *testing.T) {
	index := symbolIndex{
		"github.com/vivaui/viva/app.Config":         "app/Config",
		"github.com/vivaui/viva/gfx.Canvas":         "ui/Canvas",
		"github.com/vivaui/viva/gfx.Canvas.Hovered": "ui/Canvas#method-Hovered",
		"github.com/vivaui/viva/app.RunCanvas":      "app#func-RunCanvas",
	}
	converter := docConverter{importPath: "github.com/vivaui/viva/app", index: index}
	blocks := converter.blocks("RunCanvas uses a [Config] and a [gfx.Canvas]; see [gfx.Canvas.Hovered] and [Missing].\n\n\tcode := block()\n\nThen:\n\n  - one\n  - two\n")
	if len(blocks) != 4 || blocks[0].Kind != "paragraph" || blocks[1].Kind != "code" || blocks[2].Kind != "paragraph" || blocks[3].Kind != "list" || len(blocks[3].Items) != 2 {
		t.Fatalf("blocks = %+v", blocks)
	}
	hrefs := map[string]string{}
	var plain strings.Builder
	for _, inline := range blocks[0].Text {
		plain.WriteString(inline.Text)
		if inline.Href != "" {
			hrefs[inline.Text] = inline.Href
		}
	}
	if hrefs["Config"] != "app/Config" || hrefs["gfx.Canvas"] != "ui/Canvas" || hrefs["gfx.Canvas.Hovered"] != "ui/Canvas#method-Hovered" {
		t.Errorf("hrefs = %v", hrefs)
	}
	if _, linked := hrefs["Missing"]; linked || !strings.Contains(plain.String(), "[Missing]") {
		t.Errorf("an unknown symbol must stay plain text: %q %v", plain.String(), hrefs)
	}
}

func TestEveryDocLinkTargetsAGeneratedPage(t *testing.T) {
	data := loadTestAPI(t)
	pages := map[string]bool{}
	for _, pkg := range data.Packages {
		pages[pkg.Name] = true
		for _, typ := range pkg.Types {
			pages[pkg.Name+"/"+typ.Name] = true
		}
	}
	var check func(blocks []DocBlock)
	check = func(blocks []DocBlock) {
		for _, block := range blocks {
			for _, inline := range block.Text {
				if inline.Href == "" || strings.Contains(inline.Href, "://") {
					continue
				}
				page, _, _ := strings.Cut(inline.Href, "#")
				if !pages[page] {
					t.Errorf("link %q (%q) has no page", inline.Href, inline.Text)
				}
			}
			for _, item := range block.Items {
				check(item)
			}
		}
	}
	for _, pkg := range data.Packages {
		check(pkg.Doc)
		for _, function := range pkg.Functions {
			check(function.Doc)
		}
		for _, typ := range pkg.Types {
			check(typ.Doc)
			for _, function := range append(append([]APIFunc{}, typ.Constructors...), typ.Methods...) {
				check(function.Doc)
			}
		}
	}
}
