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
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// The core packages whose exported API the website documents, in display order.
// Optional ui/<package> packages are appended by apiPackagePaths. Each is
// documented as built for macOS on Apple silicon, so the output does not depend
// on the machine that runs the generator.
var apiPackages = []string{"app", "ui", "gfx", "components", "text", "headless", "keymap", "inspector"}

// apiPackagePaths lists the directories the API reference documents: the core
// packages, then every optional ui/<package> directory that has Go source.
func apiPackagePaths(root string) ([]string, error) {
	paths := append([]string{}, apiPackages...)
	entries, err := os.ReadDir(filepath.Join(root, "ui"))
	if err != nil {
		return nil, fmt.Errorf("api: optional packages: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		files, err := os.ReadDir(filepath.Join(root, "ui", entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("api: ui/%s: %w", entry.Name(), err)
		}
		hasSource := false
		for _, file := range files {
			name := file.Name()
			if !file.IsDir() && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
				hasSource = true
				break
			}
		}
		if hasSource {
			paths = append(paths, "ui/"+entry.Name())
		}
	}
	return paths, nil
}

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
	Name      string     `json:"name"`
	Receiver  string     `json:"receiver,omitempty"`
	Decl      string     `json:"decl"`
	Signature string     `json:"signature,omitempty"`
	Synopsis  string     `json:"synopsis"`
	Doc       []DocBlock `json:"doc"`
}

// APIField is one exported field of a documented struct type.
type APIField struct {
	Name string     `json:"name"`
	Type string     `json:"type"`
	Doc  []DocBlock `json:"doc"`
}

// APIType is a type with the functions that return it and its methods.
type APIType struct {
	Name         string     `json:"name"`
	Decl         string     `json:"decl"`
	Synopsis     string     `json:"synopsis"`
	Doc          []DocBlock `json:"doc"`
	Fields       []APIField `json:"fields,omitempty"`
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

type loadedPackage struct {
	name, importPath string
	pkg              *doc.Package
	comments         []*ast.CommentGroup
	fset             *token.FileSet
}

// loadDocPackage parses one package's exported declarations the way the API
// reference documents them: as built for macOS on Apple silicon, so the result
// does not depend on the machine that runs the generator.
func loadDocPackage(root, path string) (loadedPackage, error) {
	context := build.Default
	context.GOOS, context.GOARCH, context.CgoEnabled = "darwin", "arm64", true
	context.Dir = root
	directory := filepath.Join(root, path)
	info, err := context.ImportDir(directory, 0)
	if err != nil {
		return loadedPackage{}, fmt.Errorf("api: %s: %w", path, err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	var comments []*ast.CommentGroup
	for _, file := range append(append([]string{}, info.GoFiles...), info.CgoFiles...) {
		parsed, err := parser.ParseFile(fset, filepath.Join(directory, file), nil, parser.ParseComments)
		if err != nil {
			return loadedPackage{}, fmt.Errorf("api: %s: %w", file, err)
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
	importPath := "github.com/vivaui/viva/" + path
	pkg, err := doc.NewFromFiles(fset, ordered, importPath)
	if err != nil {
		return loadedPackage{}, fmt.Errorf("api: %s: %w", path, err)
	}
	return loadedPackage{pkg.Name, importPath, pkg, comments, fset}, nil
}

func loadAPI(root string) (APIData, error) {
	paths, err := apiPackagePaths(root)
	if err != nil {
		return APIData{}, err
	}
	var packages []loadedPackage
	index := symbolIndex{}
	documented := map[string]string{}
	for _, path := range paths {
		entry, err := loadDocPackage(root, path)
		if err != nil {
			return APIData{}, err
		}
		packages = append(packages, entry)
		documented[entry.name] = entry.importPath
		indexPackage(index, entry.name, entry.pkg)
	}
	// base holds the state behind ui's aliases (ui.Toaster is base.Toaster).
	// Load it to give an alias the fields and methods its type page needs, but
	// do not document the package itself: ui is the public surface.
	base, err := loadDocPackage(root, "base")
	if err != nil {
		return APIData{}, fmt.Errorf("api: base aliases: %w", err)
	}
	baseTypes := map[string]*doc.Type{}
	for _, typ := range base.pkg.Types {
		baseTypes[typ.Name] = typ
	}
	baseFuncs := map[string]*doc.Func{}
	for _, function := range base.pkg.Funcs {
		baseFuncs[function.Name] = function
	}
	baseConverter := docConverter{importPath: base.importPath, index: index, packages: documented}

	var data APIData
	for _, entry := range packages {
		converter := docConverter{importPath: entry.importPath, index: index, packages: documented}
		output := APIPackage{
			Name: entry.name, ImportPath: entry.importPath,
			Synopsis: entry.pkg.Synopsis(entry.pkg.Doc),
			Doc:      converter.blocks(entry.pkg.Doc),
		}
		var err error
		if output.Constants, err = apiValues(entry.fset, entry.pkg.Consts, entry.comments, converter); err != nil {
			return APIData{}, err
		}
		if output.Variables, err = apiValues(entry.fset, entry.pkg.Vars, entry.comments, converter); err != nil {
			return APIData{}, err
		}
		if output.Functions, err = apiFunctions(entry.fset, entry.pkg.Funcs, entry.comments, entry.pkg.Synopsis, converter); err != nil {
			return APIData{}, err
		}
		if entry.name == "ui" {
			// The page facade is thin: ui.Button wraps base.Button without its
			// own comment. Give the API table the underlying function's doc.
			for index := range output.Functions {
				item := &output.Functions[index]
				if item.Synopsis != "" {
					continue
				}
				if baseFunc, ok := baseFuncs[item.Name]; ok {
					item.Synopsis = base.pkg.Synopsis(baseFunc.Doc)
					item.Doc = baseConverter.blocks(baseFunc.Doc)
				}
			}
		}
		for _, typ := range entry.pkg.Types {
			decl, err := printCommented(entry.fset, typ.Decl, entry.comments)
			if err != nil {
				return APIData{}, err
			}
			item := APIType{Name: typ.Name, Decl: decl, Synopsis: entry.pkg.Synopsis(typ.Doc), Doc: converter.blocks(typ.Doc)}
			item.Fields = structFields(entry.fset, typ.Decl, converter)
			if item.Constants, err = apiValues(entry.fset, typ.Consts, entry.comments, converter); err != nil {
				return APIData{}, err
			}
			if item.Variables, err = apiValues(entry.fset, typ.Vars, entry.comments, converter); err != nil {
				return APIData{}, err
			}
			if item.Constructors, err = apiFunctions(entry.fset, typ.Funcs, entry.comments, entry.pkg.Synopsis, converter); err != nil {
				return APIData{}, err
			}
			if item.Methods, err = apiFunctions(entry.fset, typ.Methods, entry.comments, entry.pkg.Synopsis, converter); err != nil {
				return APIData{}, err
			}
			if target := aliasTarget(typ.Decl, "base"); target != "" {
				if baseType, ok := baseTypes[target]; ok {
					item.Fields = structFields(base.fset, baseType.Decl, baseConverter)
					if item.Methods, err = apiFunctions(base.fset, userMethods(base.fset, baseType.Methods), base.comments, base.pkg.Synopsis, baseConverter); err != nil {
						return APIData{}, err
					}
					if item.Synopsis == "" {
						item.Synopsis = base.pkg.Synopsis(baseType.Doc)
					}
					if len(item.Doc) == 0 {
						item.Doc = baseConverter.blocks(baseType.Doc)
					}
					// The ui facade's constructors are thin wrappers too: copy
					// the base constructor's comment when the wrapper has none.
					baseConstructors := map[string]*doc.Func{}
					for _, constructor := range baseType.Funcs {
						baseConstructors[constructor.Name] = constructor
					}
					for index := range item.Constructors {
						constructor := &item.Constructors[index]
						if constructor.Synopsis != "" {
							continue
						}
						if baseFunc, ok := baseConstructors[constructor.Name]; ok {
							constructor.Synopsis = base.pkg.Synopsis(baseFunc.Doc)
							constructor.Doc = baseConverter.blocks(baseFunc.Doc)
						}
					}
				}
			}
			output.Types = append(output.Types, item)
		}
		data.Packages = append(data.Packages, output)
	}
	return data, nil
}

func printCommented(fset *token.FileSet, node any, comments []*ast.CommentGroup) (string, error) {
	var buffer bytes.Buffer
	config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if err := config.Fprint(&buffer, fset, &printer.CommentedNode{Node: node, Comments: comments}); err != nil {
		return "", err
	}
	return strings.TrimSpace(buffer.String()), nil
}

func apiValues(fset *token.FileSet, list []*doc.Value, comments []*ast.CommentGroup, converter docConverter) ([]APIValue, error) {
	result := []APIValue{}
	for _, value := range list {
		decl, err := printCommented(fset, value.Decl, comments)
		if err != nil {
			return nil, err
		}
		result = append(result, APIValue{Names: value.Names, Decl: decl, Doc: converter.blocks(value.Doc)})
	}
	return result, nil
}

// userMethods drops the generated accessors that let the engine read fields;
// they are exported for the framework, not for the application's API table.
func userMethods(fset *token.FileSet, list []*doc.Func) []*doc.Func {
	result := make([]*doc.Func, 0, len(list))
	for _, function := range list {
		if function.Decl != nil && strings.HasSuffix(fset.Position(function.Decl.Pos()).Filename, "accessors.go") {
			continue
		}
		result = append(result, function)
	}
	return result
}

func apiFunctions(fset *token.FileSet, list []*doc.Func, comments []*ast.CommentGroup, synopsis func(string) string, converter docConverter) ([]APIFunc, error) {
	result := []APIFunc{}
	for _, function := range list {
		decl, err := printCommented(fset, function.Decl, comments)
		if err != nil {
			return nil, err
		}
		signature := ""
		if function.Decl != nil {
			copy := *function.Decl
			copy.Body = nil
			signature = printNode(fset, &copy)
		}
		result = append(result, APIFunc{
			Name: function.Name, Receiver: strings.TrimPrefix(function.Recv, "*"), Decl: decl,
			Signature: signature, Synopsis: synopsis(function.Doc), Doc: converter.blocks(function.Doc),
		})
	}
	return result, nil
}

// aliasTarget is the name a type spec aliases from pkg, or empty when it is not
// an alias there.
func aliasTarget(decl *ast.GenDecl, pkg string) string {
	if decl == nil || len(decl.Specs) == 0 {
		return ""
	}
	spec, ok := decl.Specs[0].(*ast.TypeSpec)
	if !ok || !spec.Assign.IsValid() {
		return ""
	}
	selector, ok := spec.Type.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := selector.X.(*ast.Ident)
	if !ok || ident.Name != pkg {
		return ""
	}
	return selector.Sel.Name
}

// printNode prints one AST node without the comments around it, for a type or
// a signature a table cell holds.
func printNode(fset *token.FileSet, node ast.Node) string {
	var buffer bytes.Buffer
	config := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 4}
	if err := config.Fprint(&buffer, fset, node); err != nil {
		return ""
	}
	return strings.TrimSpace(buffer.String())
}

// structFields lists the exported fields of a documented struct type, each with
// its written type and doc comment. Other types have none.
func structFields(fset *token.FileSet, decl *ast.GenDecl, converter docConverter) []APIField {
	if decl == nil {
		return nil
	}
	var structure *ast.StructType
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		if value, ok := typeSpec.Type.(*ast.StructType); ok {
			structure = value
			break
		}
	}
	if structure == nil {
		return nil
	}
	var fields []APIField
	for _, field := range structure.Fields.List {
		doc := field.Doc
		if doc == nil {
			doc = field.Comment
		}
		text := ""
		if doc != nil {
			text = doc.Text()
		}
		typ := printNode(fset, field.Type)
		names := field.Names
		if len(names) == 0 {
			name := embeddedFieldName(field.Type)
			if name == "" || !ast.IsExported(name) {
				continue
			}
			fields = append(fields, APIField{Name: name, Type: typ, Doc: converter.blocks(text)})
			continue
		}
		for _, name := range names {
			if !name.IsExported() {
				continue
			}
			fields = append(fields, APIField{Name: name.Name, Type: typ, Doc: converter.blocks(text)})
		}
	}
	return fields
}

func embeddedFieldName(expr ast.Expr) string {
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name
	case *ast.StarExpr:
		return embeddedFieldName(expr.X)
	case *ast.SelectorExpr:
		return expr.Sel.Name
	}
	return ""
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
	packages   map[string]string
}

func (converter docConverter) blocks(text string) []DocBlock {
	if strings.TrimSpace(text) == "" {
		return []DocBlock{}
	}
	var parser comment.Parser
	parser.LookupPackage = func(name string) (string, bool) {
		importPath, ok := converter.packages[name]
		return importPath, ok
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

// pageAPISymbol reports whether symbol names a type, function, constructor or
// method the API data documents. Constants and variables have no table row, so
// a derived page list drops them.
func pageAPISymbol(data APIData, symbol string) bool {
	pkgName, name, ok := strings.Cut(symbol, ".")
	if !ok || pkgName == "" || name == "" {
		return false
	}
	for _, pkg := range data.Packages {
		if pkg.Name != pkgName {
			continue
		}
		for _, function := range pkg.Functions {
			if function.Name == name {
				return true
			}
		}
		for _, typ := range pkg.Types {
			if typ.Name == name {
				return true
			}
			for _, function := range typ.Constructors {
				if function.Name == name {
					return true
				}
			}
			for _, method := range typ.Methods {
				if method.Name == name {
					return true
				}
			}
		}
	}
	return false
}

// prunePageAPI drops the symbols a derived page list guessed but the API data
// does not document, such as package constants and local example identifiers.
func prunePageAPI(pages []Page, data APIData) {
	for index := range pages {
		seen := map[string]bool{}
		kept := pages[index].API[:0]
		for _, symbol := range pages[index].API {
			if seen[symbol] || !pageAPISymbol(data, symbol) {
				continue
			}
			seen[symbol] = true
			kept = append(kept, symbol)
		}
		pages[index].API = kept
	}
}

// validatePageAPI reports a component page that names a symbol the API data
// does not contain, which would render an empty or broken table.
func validatePageAPI(pages []Page, data APIData) error {
	for _, page := range pages {
		for _, symbol := range page.API {
			if _, _, ok := strings.Cut(symbol, "."); !ok {
				return fmt.Errorf("catalog: %s API symbol %q is not package.Name", page.ID, symbol)
			}
			if !pageAPISymbol(data, symbol) {
				return fmt.Errorf("catalog: %s names unknown API symbol %s", page.ID, symbol)
			}
		}
	}
	return nil
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
