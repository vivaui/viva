// Command apicheck rejects dynamic types in the exported API of the packages a
// page is written with. Reflection, 'any' and map[string]any are what the
// engine's layout, input, scene and render paths must not see; this makes a
// new exported symbol that introduces one a build failure instead of a review
// catch.
//
// It reads the packages named on the command line (ui, components and app by
// default), and a file of allowed symbols, one per line, for the cases that
// already exist. Run it with:
//
//	go run ./tools/apicheck ./ui ./components ./app
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type finding struct {
	position string
	symbol   string
	reason   string
}

func main() {
	allowPath := flag.String("allow", "tools/apicheck/allow.txt", "file of symbols allowed to use a dynamic type, one per line")
	flag.Parse()
	dirs := flag.Args()
	if len(dirs) == 0 {
		dirs = []string{"ui", "components", "app", "gfx"}
	}
	findings, err := check(dirs)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apicheck:", err)
		os.Exit(2)
	}
	allowed, err := loadAllow(*allowPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "apicheck:", err)
		os.Exit(2)
	}
	kept := findings[:0]
	for _, finding := range findings {
		if !allowed[finding.symbol] {
			kept = append(kept, finding)
		}
	}
	for _, finding := range kept {
		fmt.Fprintf(os.Stderr, "%s: %s: %s\n", finding.position, finding.symbol, finding.reason)
	}
	if len(kept) > 0 {
		fmt.Fprintf(os.Stderr, "apicheck: %d exported symbol(s) use a dynamic type; use a typed one, or add the symbol to %s\n", len(kept), *allowPath)
		os.Exit(1)
	}
}

// loadAllow reads the allowed symbols; a missing file allows nothing.
func loadAllow(path string) (map[string]bool, error) {
	allowed := make(map[string]bool)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return allowed, nil
	}
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if index := strings.IndexByte(line, '#'); index >= 0 {
			line = strings.TrimSpace(line[:index])
		}
		if line == "" {
			continue
		}
		allowed[line] = true
	}
	return allowed, nil
}

// check reports the exported declarations in dirs that name a dynamic type.
func check(dirs []string) ([]finding, error) {
	var findings []finding
	for _, dir := range dirs {
		paths, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			return nil, err
		}
		fset := token.NewFileSet()
		var files []*ast.File
		for _, path := range paths {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
			if err != nil {
				return nil, err
			}
			files = append(files, file)
		}
		if len(files) == 0 {
			continue
		}
		pkg := files[0].Name.Name
		for _, file := range files {
			findings = append(findings, checkFile(fset, pkg, file)...)
		}
	}
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].position != findings[j].position {
			return findings[i].position < findings[j].position
		}
		return findings[i].symbol < findings[j].symbol
	})
	return findings, nil
}

func checkFile(fset *token.FileSet, pkg string, file *ast.File) []finding {
	var findings []finding
	add := func(node ast.Node, symbol, reason string) {
		findings = append(findings, finding{position: fset.Position(node.Pos()).String(), symbol: symbol, reason: reason})
	}
	for _, spec := range file.Imports {
		if spec.Path.Value == `"reflect"` {
			add(spec, pkg+"/reflect", "imports reflect")
		}
	}
	for _, decl := range file.Decls {
		switch decl := decl.(type) {
		case *ast.FuncDecl:
			if !decl.Name.IsExported() {
				continue
			}
			symbol := pkg + "." + decl.Name.Name
			if decl.Recv != nil && len(decl.Recv.List) > 0 {
				if receiver := receiverName(decl.Recv.List[0].Type); receiver != "" {
					symbol = pkg + "." + receiver + "." + decl.Name.Name
				}
			}
			if fieldListUsesAny(decl.Type.Params) || fieldListUsesAny(decl.Type.Results) {
				add(decl, symbol, "a parameter or result names any")
			}
		case *ast.GenDecl:
			for _, raw := range decl.Specs {
				switch spec := raw.(type) {
				case *ast.TypeSpec:
					if !spec.Name.IsExported() {
						continue
					}
					symbol := pkg + "." + spec.Name.Name
					switch typ := spec.Type.(type) {
					case *ast.StructType:
						for _, field := range typ.Fields.List {
							for _, name := range field.Names {
								if !name.IsExported() {
									continue
								}
								if usesAny(field.Type) {
									add(field, symbol+"."+name.Name, "an exported field names any")
								}
							}
						}
					case *ast.InterfaceType:
						for _, method := range typ.Methods.List {
							funcType, ok := method.Type.(*ast.FuncType)
							if !ok {
								continue // an embedded interface is checked where it is declared
							}
							for _, name := range method.Names {
								if !name.IsExported() {
									continue
								}
								if fieldListUsesAny(funcType.Params) || fieldListUsesAny(funcType.Results) {
									add(method, symbol+"."+name.Name, "an exported interface method names any")
								}
							}
						}
					default:
						if usesAny(spec.Type) {
							add(spec, symbol, "an exported type names any")
						}
					}
				case *ast.ValueSpec:
					for _, name := range spec.Names {
						if !name.IsExported() || spec.Type == nil {
							continue
						}
						if usesAny(spec.Type) {
							add(spec, pkg+"."+name.Name, "an exported value names any")
						}
					}
				}
			}
		}
	}
	return findings
}

func receiverName(expr ast.Expr) string {
	for {
		switch e := expr.(type) {
		case *ast.StarExpr:
			expr = e.X
		case *ast.IndexExpr:
			expr = e.X
		case *ast.IndexListExpr:
			expr = e.X
		case *ast.Ident:
			return e.Name
		default:
			return ""
		}
	}
}

func fieldListUsesAny(fields *ast.FieldList) bool {
	if fields == nil {
		return false
	}
	for _, field := range fields.List {
		if usesAny(field.Type) {
			return true
		}
	}
	return false
}

// usesAny reports whether a type expression names `any`, `interface{}`, or
// holds one inside a container.
func usesAny(expr ast.Expr) bool {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name == "any"
	case *ast.InterfaceType:
		if e.Methods == nil || len(e.Methods.List) == 0 {
			return true
		}
		for _, method := range e.Methods.List {
			funcType, ok := method.Type.(*ast.FuncType)
			if !ok {
				continue // an embedded interface is checked where it is declared
			}
			if fieldListUsesAny(funcType.Params) || fieldListUsesAny(funcType.Results) {
				return true
			}
		}
		return false
	case *ast.MapType:
		return usesAny(e.Key) || usesAny(e.Value)
	case *ast.ArrayType:
		return usesAny(e.Elt)
	case *ast.StarExpr:
		return usesAny(e.X)
	case *ast.ChanType:
		return usesAny(e.Value)
	case *ast.Ellipsis:
		return usesAny(e.Elt)
	case *ast.FuncType:
		return fieldListUsesAny(e.Params) || fieldListUsesAny(e.Results)
	case *ast.StructType:
		for _, field := range e.Fields.List {
			if usesAny(field.Type) {
				return true
			}
		}
		return false
	}
	return false
}
