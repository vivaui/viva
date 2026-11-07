package main

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/build"
	"go/doc"
	"go/doc/comment"
	"go/parser"
	"go/printer"
	"go/token"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// The packages whose exported API the website documents, in display order. Each
// is documented as built for macOS on Apple silicon, so the output does not
// depend on the machine that runs the generator.
var apiPackages = []string{"app", "ui", "components", "text", "headless", "keymap", "inspector"}

// APIData is the generated API reference: every exported identifier of the
// public packages, with its declaration and doc comment.
type APIData struct {
	Packages []APIPackage `json:"packages"`
}

// APIPackage is one importable Viva package.
type APIPackage struct {
	Name       string     `json:"name"`
	ImportPath string     `json:"importPath"`
	Synopsis   string     `json:"synopsis"`
	Doc        []DocBlock `json:"doc"`
	Constants  []APIValue `json:"constants"`
	Variables  []APIValue `json:"variables"`
	Functions  []APIFunc  `json:"functions"`
	Types      []APIType  `json:"types"`
}

// APIValue is a const or var declaration group.
type APIValue struct {
	Names []string   `json:"names"`
	Decl  string     `json:"decl"`
	Doc   []DocBlock `json:"doc"`
}

// APIFunc is a function or a method.
type APIFunc struct {
	Name     string     `json:"name"`
	Receiver string     `json:"receiver,omitempty"`
	Decl     string     `json:"decl"`
	Synopsis string     `json:"synopsis"`
	Doc      []DocBlock `json:"doc"`
}

// APIType is a type with the functions that return it and its methods.
type APIType struct {
	Name         string     `json:"name"`
	Decl         string     `json:"decl"`
	Synopsis     string     `json:"synopsis"`
	Doc          []DocBlock `json:"doc"`
	Constants    []APIValue `json:"constants"`
	Variables    []APIValue `json:"variables"`
	Constructors []APIFunc  `json:"constructors"`
	Methods      []APIFunc  `json:"methods"`
}

// DocBlock is one block of a doc comment: a paragraph, a heading, a list or code.
type DocBlock struct {
	Kind    string       `json:"kind"`
	Text    []DocInline  `json:"text,omitempty"`
	Items   [][]DocBlock `json:"items,omitempty"`
	Code    string       `json:"code,omitempty"`
	Ordered bool         `json:"ordered,omitempty"`
}

// DocInline is a run of text inside a block. Href is a website-relative API
// path (package/Type, optionally with an #anchor) for a link to another symbol,
// or an absolute URL for an external link.
type DocInline struct {
	Text   string `json:"text"`
	Italic bool   `json:"italic,omitempty"`
	Code   bool   `json:"code,omitempty"`
	Href   string `json:"href,omitempty"`
}

// symbolIndex maps "importPath.Name" and "importPath.Recv.Name" to the page and
// anchor that documents the symbol.
type symbolIndex map[string]string

func loadAPI(root string) (APIData, error) {
	context := build.Default
	context.GOOS, context.GOARCH, context.CgoEnabled = "darwin", "arm64", true
	context.Dir = root

	type loaded struct {
		name, importPath string
		pkg              *doc.Package
		comments         []*ast.CommentGroup
		fset             *token.FileSet
	}
	var packages []loaded
	index := symbolIndex{}
	for _, name := range apiPackages {
		directory := filepath.Join(root, name)
		info, err := context.ImportDir(directory, 0)
		if err != nil {
			return APIData{}, fmt.Errorf("api: %s: %w", name, err)
		}
		fset := token.NewFileSet()
		files := map[string]*ast.File{}
		var comments []*ast.CommentGroup
		for _, file := range append(append([]string{}, info.GoFiles...), info.CgoFiles...) {
			parsed, err := parser.ParseFile(fset, filepath.Join(directory, file), nil, parser.ParseComments)
			if err != nil {
				return APIData{}, fmt.Errorf("api: %s: %w", file, err)
			}
			files[file] = parsed
			comments = append(comments, parsed.Comments...)
		}
		var ordered []*ast.File
		keys := make([]string, 0, len(files))
		for key := range files {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			ordered = append(ordered, files[key])
		}
		importPath := "github.com/vivaui/viva/" + name
		pkg, err := doc.NewFromFiles(fset, ordered, importPath)
		if err != nil {
			return APIData{}, fmt.Errorf("api: %s: %w", name, err)
		}
		packages = append(packages, loaded{name, importPath, pkg, comments, fset})
		indexPackage(index, name, pkg)
	}

	var data APIData
	for _, entry := range packages {
		converter := docConverter{importPath: entry.importPath, index: index}
		output := APIPackage{
			Name: entry.name, ImportPath: entry.importPath,
			Synopsis: entry.pkg.Synopsis(entry.pkg.Doc),
			Doc:      converter.blocks(entry.pkg.Doc),
		}
		printDecl := func(node any) (string, error) {
			var buffer bytes.Buffer
			config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
			if err := config.Fprint(&buffer, entry.fset, &printer.CommentedNode{Node: node, Comments: entry.comments}); err != nil {
				return "", err
			}
			return strings.TrimSpace(buffer.String()), nil
		}
		values := func(list []*doc.Value) ([]APIValue, error) {
			result := []APIValue{}
			for _, value := range list {
				decl, err := printDecl(value.Decl)
				if err != nil {
					return nil, err
				}
				result = append(result, APIValue{Names: value.Names, Decl: decl, Doc: converter.blocks(value.Doc)})
			}
			return result, nil
		}
		functions := func(list []*doc.Func) ([]APIFunc, error) {
			result := []APIFunc{}
			for _, function := range list {
				decl, err := printDecl(function.Decl)
				if err != nil {
					return nil, err
				}
				result = append(result, APIFunc{
					Name: function.Name, Receiver: strings.TrimPrefix(function.Recv, "*"), Decl: decl,
					Synopsis: entry.pkg.Synopsis(function.Doc), Doc: converter.blocks(function.Doc),
				})
			}
			return result, nil
		}
		var err error
		if output.Constants, err = values(entry.pkg.Consts); err != nil {
			return APIData{}, err
		}
		if output.Variables, err = values(entry.pkg.Vars); err != nil {
			return APIData{}, err
		}
		if output.Functions, err = functions(entry.pkg.Funcs); err != nil {
			return APIData{}, err
		}
		for _, typ := range entry.pkg.Types {
			decl, err := printDecl(typ.Decl)
			if err != nil {
				return APIData{}, err
			}
			item := APIType{Name: typ.Name, Decl: decl, Synopsis: entry.pkg.Synopsis(typ.Doc), Doc: converter.blocks(typ.Doc)}
			if item.Constants, err = values(typ.Consts); err != nil {
				return APIData{}, err
			}
			if item.Variables, err = values(typ.Vars); err != nil {
				return APIData{}, err
			}
			if item.Constructors, err = functions(typ.Funcs); err != nil {
				return APIData{}, err
			}
			if item.Methods, err = functions(typ.Methods); err != nil {
				return APIData{}, err
			}
			output.Types = append(output.Types, item)
		}
		data.Packages = append(data.Packages, output)
	}
	return data, nil
}

func indexPackage(index symbolIndex, name string, pkg *doc.Package) {
	importPath := "github.com/vivaui/viva/" + name
	for _, value := range pkg.Consts {
		for _, symbol := range value.Names {
			index[importPath+"."+symbol] = name + "#value-" + symbol
		}
	}
	for _, value := range pkg.Vars {
		for _, symbol := range value.Names {
			index[importPath+"."+symbol] = name + "#value-" + symbol
		}
	}
	for _, function := range pkg.Funcs {
		index[importPath+"."+function.Name] = name + "#func-" + function.Name
	}
	for _, typ := range pkg.Types {
		index[importPath+"."+typ.Name] = name + "/" + typ.Name
		for _, value := range typ.Consts {
			for _, symbol := range value.Names {
				index[importPath+"."+symbol] = name + "/" + typ.Name + "#value-" + symbol
			}
		}
		for _, value := range typ.Vars {
			for _, symbol := range value.Names {
				index[importPath+"."+symbol] = name + "/" + typ.Name + "#value-" + symbol
			}
		}
		for _, function := range typ.Funcs {
			index[importPath+"."+function.Name] = name + "/" + typ.Name + "#func-" + function.Name
		}
		for _, method := range typ.Methods {
			index[importPath+"."+typ.Name+"."+method.Name] = name + "/" + typ.Name + "#method-" + method.Name
		}
	}
}

// docConverter turns a doc comment into blocks, resolving [Name] links against
// the symbols the website documents.
type docConverter struct {
	importPath string
	index      symbolIndex
}

func (converter docConverter) blocks(text string) []DocBlock {
	if strings.TrimSpace(text) == "" {
		return []DocBlock{}
	}
	var parser comment.Parser
	parser.LookupPackage = func(name string) (string, bool) {
		importPath := "github.com/vivaui/viva/" + name
		for _, known := range apiPackages {
			if known == name {
				return importPath, true
			}
		}
		return "", false
	}
	parser.LookupSym = func(recv, name string) bool {
		key := converter.importPath + "." + name
		if recv != "" {
			key = converter.importPath + "." + recv + "." + name
		}
		_, ok := converter.index[key]
		return ok
	}
	return converter.convert(parser.Parse(text).Content)
}

func (converter docConverter) convert(blocks []comment.Block) []DocBlock {
	result := []DocBlock{}
	for _, block := range blocks {
		switch block := block.(type) {
		case *comment.Paragraph:
			result = append(result, DocBlock{Kind: "paragraph", Text: converter.inline(block.Text)})
		case *comment.Heading:
			result = append(result, DocBlock{Kind: "heading", Text: converter.inline(block.Text)})
		case *comment.Code:
			result = append(result, DocBlock{Kind: "code", Code: strings.TrimRight(block.Text, "\n")})
		case *comment.List:
			list := DocBlock{Kind: "list", Ordered: block.Items[0].Number != ""}
			for _, item := range block.Items {
				list.Items = append(list.Items, converter.convert(item.Content))
			}
			result = append(result, list)
		}
	}
	return result
}

func (converter docConverter) inline(texts []comment.Text) []DocInline {
	result := []DocInline{}
	var visit func(texts []comment.Text, italic bool, href string)
	visit = func(texts []comment.Text, italic bool, href string) {
		for _, text := range texts {
			switch text := text.(type) {
			case comment.Plain:
				result = append(result, DocInline{Text: string(text), Italic: italic, Href: href})
			case comment.Italic:
				result = append(result, DocInline{Text: string(text), Italic: true, Href: href})
			case *comment.Link:
				visit(text.Text, italic, text.URL)
			case *comment.DocLink:
				target := converter.resolve(text)
				if target == "" {
					visit(text.Text, italic, href)
					continue
				}
				for _, part := range text.Text {
					if plain, ok := part.(comment.Plain); ok {
						result = append(result, DocInline{Text: string(plain), Code: true, Href: target})
					}
				}
			}
		}
	}
	visit(texts, false, "")
	return result
}

func (converter docConverter) resolve(link *comment.DocLink) string {
	importPath := link.ImportPath
	if importPath == "" {
		importPath = converter.importPath
	}
	key := importPath + "." + link.Name
	if link.Recv != "" {
		key = importPath + "." + link.Recv + "." + link.Name
	}
	return converter.index[key]
}

// validateAPI reports a structural problem that would produce a broken page.
func validateAPI(data APIData) error {
	seen := map[string]bool{}
	for _, pkg := range data.Packages {
		if pkg.Name == "" || len(pkg.Types)+len(pkg.Functions) == 0 {
			return fmt.Errorf("api: package %q documents nothing", pkg.Name)
		}
		for _, typ := range pkg.Types {
			key := path.Join(pkg.Name, typ.Name)
			if seen[key] {
				return fmt.Errorf("api: duplicate type page %s", key)
			}
			seen[key] = true
			if !ast.IsExported(typ.Name) {
				return fmt.Errorf("api: unexported type %s leaked into %s", typ.Name, pkg.Name)
			}
		}
	}
	return nil
}
