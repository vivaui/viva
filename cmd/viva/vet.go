package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// viva vet finds the mistakes in a page written with package ui that compile and run but
// do the wrong thing, before the program runs. It reads the
// packages with go/types and the export data `go list -export` builds, so it needs
// nothing but the Go toolchain.

const uiPath = "github.com/vivaui/viva/ui"

// vetFinding is one mistake, at a place in a file.
type vetFinding struct {
	pos     token.Position
	check   string
	message string
}

func (finding vetFinding) String() string {
	return fmt.Sprintf("%s:%d:%d: %s (%s)", finding.pos.Filename, finding.pos.Line, finding.pos.Column, finding.message, finding.check)
}

func runVet(arguments []string, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("vet", flag.ContinueOnError)
	directory := "."
	flags.StringVar(&directory, "C", directory, "module directory")
	c, _ := findCommand("vet")
	patterns, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	if err := requireGo(errorOutput); err != nil {
		return err
	}
	// The export data go list writes is read by the go/types this program was built
	// with; a different Go writes a format it may not read. Then the project's own
	// Go builds the viva vet of the Viva version the project requires, and runs it.
	if version, err := goEnv(directory, "GOVERSION"); err == nil && version != runtime.Version() && os.Getenv("VIVA_VET_CHILD") == "" {
		child := exec.Command("go", append([]string{"run", "github.com/vivaui/viva/cmd/viva", "vet"}, patterns...)...)
		child.Dir = directory
		child.Env = append(os.Environ(), "VIVA_VET_CHILD=1")
		child.Stdout, child.Stderr = output, errorOutput
		if err := child.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return errors.New("vet: see above (it ran with the project's Go, " + version + ")")
			}
			return err
		}
		return nil
	}
	findings, err := vetPackages(directory, patterns)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		if relative, err := filepath.Rel(directory, finding.pos.Filename); err == nil && !strings.HasPrefix(relative, "..") {
			finding.pos.Filename = relative
		}
		fmt.Fprintln(output, finding)
	}
	if len(findings) > 0 {
		return fmt.Errorf("vet: %d problem(s) found", len(findings))
	}
	return nil
}

// listedPackage is what vet reads of `go list -json -export`.
type listedPackage struct {
	ImportPath      string
	Dir             string
	Name            string
	Export          string
	GoFiles         []string
	CompiledGoFiles []string
	Imports         []string
	ImportMap       map[string]string
	DepOnly         bool
	Error           *struct{ Err string }
}

// vetPackages checks the packages patterns name in directory.
func vetPackages(directory string, patterns []string) ([]vetFinding, error) {
	list := exec.Command("go", append([]string{"list", "-e", "-json", "-export", "-deps"}, patterns...)...)
	list.Dir = directory
	var stdout, stderr bytes.Buffer
	list.Stdout, list.Stderr = &stdout, &stderr
	if err := list.Run(); err != nil {
		return nil, fmt.Errorf("go list: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	exports := map[string]string{}
	var targets []listedPackage
	decoder := json.NewDecoder(&stdout)
	for decoder.More() {
		var pkg listedPackage
		if err := decoder.Decode(&pkg); err != nil {
			return nil, err
		}
		if pkg.Export != "" {
			exports[pkg.ImportPath] = pkg.Export
		}
		if !pkg.DepOnly {
			targets = append(targets, pkg)
		}
	}
	var findings []vetFinding
	for _, pkg := range targets {
		if pkg.Error != nil {
			return nil, fmt.Errorf("%s: %s", pkg.ImportPath, pkg.Error.Err)
		}
		if !importsFlex(pkg) {
			continue
		}
		found, err := vetPackage(pkg, exports)
		if err != nil {
			return nil, err
		}
		findings = append(findings, found...)
	}
	sort.Slice(findings, func(i, j int) bool {
		a, b := findings[i].pos, findings[j].pos
		if a.Filename != b.Filename {
			return a.Filename < b.Filename
		}
		return a.Line < b.Line || a.Line == b.Line && a.Column < b.Column
	})
	return findings, nil
}

func importsFlex(pkg listedPackage) bool {
	for _, path := range pkg.Imports {
		if path == uiPath {
			return true
		}
	}
	return false
}

// vetPackage type-checks one package against its dependencies' export data and
// walks it for the mistakes.
func vetPackage(pkg listedPackage, exports map[string]string) ([]vetFinding, error) {
	fset := token.NewFileSet()
	names := pkg.CompiledGoFiles
	if len(names) == 0 {
		names = pkg.GoFiles
	}
	var files []*ast.File
	for _, name := range names {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(pkg.Dir, name)
		}
		if !strings.HasSuffix(path, ".go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	lookup := func(path string) (io.ReadCloser, error) {
		if mapped, ok := pkg.ImportMap[path]; ok {
			path = mapped
		}
		export, ok := exports[path]
		if !ok {
			return nil, fmt.Errorf("no export data for %s", path)
		}
		return os.Open(export)
	}
	config := types.Config{Importer: importer.ForCompiler(fset, "gc", lookup), Error: func(error) {}}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
	}
	if _, err := checkTypes(config, pkg.ImportPath, fset, files, info); err != nil {
		var typeErr types.Error
		if errors.As(err, &typeErr) {
			return nil, fmt.Errorf("%s does not compile: %v (run go vet first)", pkg.ImportPath, err)
		}
		return nil, err
	}
	checker := &vetChecker{fset: fset, info: info}
	for _, file := range files {
		checker.file = file
		checker.walk(file, nil)
		checker.retained(file)
		checker.goroutines(file)
	}
	return checker.findings, nil
}

type vetChecker struct {
	fset     *token.FileSet
	info     *types.Info
	file     *ast.File
	findings []vetFinding
}

// isContext reports whether t is ui.C (a *ui.Ctx): what a page and its
// components are declared with.
func isContext(t types.Type) bool {
	pointer, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	named, ok := types.Unalias(pointer.Elem()).(*types.Named)
	return ok && named.Obj().Pkg() != nil && named.Obj().Pkg().Path() == uiPath && named.Obj().Name() == "Ctx"
}

// isRetainedType reports whether t is a value that belongs to one paint: ui.C,
// ui.Box or *ui.Frame, or a container or function type that can carry one.
// Holding one across paints reads a reused node or a Frame no longer declaring.
func isRetainedType(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	switch t := t.(type) {
	case *types.Pointer:
		return isRetainedType(t.Elem())
	case *types.Slice:
		return isRetainedType(t.Elem())
	case *types.Array:
		return isRetainedType(t.Elem())
	case *types.Map:
		return isRetainedType(t.Key()) || isRetainedType(t.Elem())
	case *types.Chan:
		return isRetainedType(t.Elem())
	case *types.Signature:
		if params := t.Params(); params != nil {
			for i := range params.Len() {
				if isRetainedType(params.At(i).Type()) {
					return true
				}
			}
		}
		if results := t.Results(); results != nil {
			for i := range results.Len() {
				if isRetainedType(results.At(i).Type()) {
					return true
				}
			}
		}
		return false
	case *types.Named:
		object := t.Obj()
		if object.Pkg() == nil || object.Pkg().Path() != uiPath {
			return false
		}
		switch object.Name() {
		case "Ctx", "Box", "Frame":
			return true
		}
		return false
	}
	return false
}

// retainedCheck names what is kept and how to keep the persistent identity instead.
func retainedCheck(t types.Type) string {
	if isContext(t) {
		return "a ui.C is kept in a struct, but it is valid only inside the page function it was given to: " +
			"keep the *ui.Page that NewPage returns, or pass c down as a parameter"
	}
	return "a per-paint ui value (a Box, Frame or parts value) is kept across paints, but it names a node of one paint: " +
		"keep the *ui.Page and a gfx.ID, or pass the value down as a parameter"
}

// walk visits node with stack, the nodes around it, outermost first.
func (checker *vetChecker) walk(node ast.Node, stack []ast.Node) {
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if call, ok := n.(*ast.CallExpr); ok {
			checker.call(call, stack)
		}
		stack = append(stack, n)
		return true
	})
}

// call checks one call of package ui: a Button whose label is the same in every
// row of a loop whose rows have no Key, or a Memo whose value does not cover
// what its function reads.
func (checker *vetChecker) call(call *ast.CallExpr, stack []ast.Node) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := checker.info.Uses[identOf(selector.X)].(*types.PkgName)
	if !ok || pkg.Imported().Path() != uiPath {
		return
	}
	switch selector.Sel.Name {
	case "Button":
		checker.unkeyedButton(call, stack)
	case "Memo":
		checker.memo(call)
	}
}

// unkeyedButton reports a Button with the same label in every row of a loop
// whose rows have no Key: a click can move to the wrong row.
func (checker *vetChecker) unkeyedButton(call *ast.CallExpr, stack []ast.Node) {
	if len(call.Args) != 2 {
		return
	}
	if t := checker.info.Types[call.Args[0]].Type; t == nil || !isContext(t) {
		return
	}
	value := checker.info.Types[call.Args[1]].Value
	loop := innermostLoop(stack)
	if value == nil || loop == nil || checker.keyedIn(loop, call) {
		return
	}
	checker.report(call, "unkeyed-button", fmt.Sprintf(
		"every row of this loop has a Button labeled %s and the rows have no Key, so a row added or removed above moves a click to the wrong row: "+
			"declare each row with ui.Row(c.KeyInt(item.ID)) (or c.Key(name)) and the Button inside it", value.ExactString()))
}

// memo reports a Memo whose value does not cover a root or field its function
// reads. The analysis is deliberately conservative: a named helper, an
// interface value or a call is not followed, so a missed read is possible; a
// read reported as missing is a real dependency the value expression never
// mentions.
func (checker *vetChecker) memo(call *ast.CallExpr) {
	if len(call.Args) != 3 {
		return
	}
	if t := checker.info.Types[call.Args[0]].Type; t == nil || !isContext(t) {
		return
	}
	fn, ok := ast.Unparen(call.Args[2]).(*ast.FuncLit)
	if !ok {
		return
	}
	cover, known := memoCoverage(checker.info, call.Args[1])
	if !known {
		return
	}
	missing := []string{}
	seen := map[string]bool{}
	for _, path := range memoReads(checker.info, fn) {
		if seen[path] || cover.covers(path) {
			continue
		}
		seen[path] = true
		missing = append(missing, path)
	}
	if len(missing) == 0 {
		return
	}
	sort.Strings(missing)
	if len(missing) > 3 {
		missing = append(missing[:3], "...")
	}
	checker.report(call, "memo-value", "the Memo's value does not cover "+strings.Join(missing, ", ")+
		", which its function reads: pass the row itself (ui.Memo(c, *row, ...)) or a key that includes these fields, "+
		"or silence this line with viva:ignore memo-value")
}

// memoCover is what a Memo's value expression covers: a bare root (the whole
// value, or a pointer to it) covers every field read through it; a field exact
// path covers that one read.
type memoCover struct {
	bare  map[string]bool
	exact map[string]bool
}

func (c memoCover) covers(path string) bool {
	if c.exact[path] {
		return true
	}
	for i := 0; i < len(path); i++ {
		if path[i] == '.' && c.bare[path[:i]] {
			return true
		}
	}
	return c.bare[path]
}

// memoCoverage builds the covered paths of a Memo's value. A value it cannot
// read statically (a call, an arithmetic expression) makes the check unknown
// and the caller skips it.
func memoCoverage(info *types.Info, value ast.Expr) (memoCover, bool) {
	cover := memoCover{bare: map[string]bool{}, exact: map[string]bool{}}
	known := true
	var add func(ast.Expr)
	add = func(e ast.Expr) {
		e = ast.Unparen(e)
		switch e := e.(type) {
		case *ast.Ident:
			if _, ok := info.Uses[e].(*types.Var); ok {
				cover.bare[e.Name] = true
			}
		case *ast.StarExpr:
			if path, ok := exprPath(e.X); ok {
				cover.bare[path] = true
			} else {
				known = false
			}
		case *ast.SelectorExpr:
			if path, ok := exprPath(e); ok {
				if isStructValue(info.Types[e].Type) {
					cover.bare[path] = true // a row, not one of its scalar fields
				} else {
					cover.exact[path] = true
				}
			} else {
				known = false
			}
		case *ast.CompositeLit:
			for _, element := range e.Elts {
				if pair, ok := element.(*ast.KeyValueExpr); ok {
					add(pair.Value)
				} else {
					add(element)
				}
			}
		case *ast.BasicLit:
			// A constant value covers no read.
		default:
			known = false
		}
	}
	add(value)
	return cover, known
}

// isStructValue reports whether a value is a struct or a pointer to one, so
// reading its fields is covered by the value itself rather than by one field.
func isStructValue(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	if pointer, ok := t.(*types.Pointer); ok {
		t = types.Unalias(pointer.Elem())
	}
	_, ok := t.Underlying().(*types.Struct)
	return ok
}

// memoReads lists the paths a Memo's function reads from outside itself: roots
// and fields of variables declared before the closure, including arguments
// passed to a call. Method calls are not followed, and locals, constants,
// packages and the page's C are not reads of the value.
func memoReads(info *types.Info, fn *ast.FuncLit) []string {
	parents := map[ast.Node]ast.Node{}
	var stack []ast.Node
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if len(stack) > 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	var reads []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.SelectorExpr:
			if parent, ok := parents[node].(*ast.SelectorExpr); ok && parent.X == node {
				return true // the inner half of a chain: the outer path is the read
			}
			if parent, ok := parents[node].(*ast.CallExpr); ok && parent.Fun == node {
				return true // the callee's reads are not visible
			}
			if selection, ok := info.Selections[node]; ok && (selection.Kind() == types.MethodVal || selection.Kind() == types.MethodExpr) {
				return true
			}
			path, ok := exprPath(node)
			if !ok {
				return true
			}
			root := leftmostIdent(node)
			if root == nil {
				return true
			}
			if object, ok := info.Uses[root].(*types.Var); ok && !object.IsField() && !isContext(object.Type()) && object.Pos() < fn.Pos() {
				reads = append(reads, path)
			}
		case *ast.Ident:
			if parent, ok := parents[node].(*ast.SelectorExpr); ok && (parent.X == node || parent.Sel == node) {
				return true
			}
			object, ok := info.Uses[node].(*types.Var)
			if !ok || object.IsField() || isContext(object.Type()) || object.Pos() >= fn.Pos() {
				return true
			}
			reads = append(reads, node.Name)
		}
		return true
	})
	return reads
}

// exprPath is the dotted path of an expression made of identifiers and
// selectors, such as m.host.Name; a dereference has the path of what it points
// to.
func exprPath(e ast.Expr) (string, bool) {
	switch e := ast.Unparen(e).(type) {
	case *ast.Ident:
		return e.Name, true
	case *ast.SelectorExpr:
		if path, ok := exprPath(e.X); ok {
			return path + "." + e.Sel.Name, true
		}
	case *ast.StarExpr:
		return exprPath(e.X)
	}
	return "", false
}

// leftmostIdent is the identifier the path of a selector starts at.
func leftmostIdent(expr ast.Expr) *ast.Ident {
	for {
		switch node := ast.Unparen(expr).(type) {
		case *ast.Ident:
			return node
		case *ast.SelectorExpr:
			expr = node.X
		case *ast.StarExpr:
			expr = node.X
		default:
			return nil
		}
	}
}

// innermostLoop is the innermost for or range statement around the call, inside
// the function the call is in.
func innermostLoop(stack []ast.Node) ast.Node {
	for i := len(stack) - 1; i >= 0; i-- {
		switch node := stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return node
		case *ast.FuncDecl:
			return nil
		}
	}
	return nil
}

// keyedIn reports whether the loop's body names a row with Key, KeyInt or a
// Memo's key before the call: c.Key(...) or c.KeyInt(...) anywhere in it.
func (checker *vetChecker) keyedIn(loop ast.Node, call *ast.CallExpr) bool {
	keyed := false
	ast.Inspect(loop, func(n ast.Node) bool {
		if keyed || n == nil {
			return false
		}
		if inner, ok := n.(*ast.CallExpr); ok && inner.Pos() < call.Pos() {
			if selector, ok := inner.Fun.(*ast.SelectorExpr); ok && (selector.Sel.Name == "Key" || selector.Sel.Name == "KeyInt") {
				if t := checker.info.Types[selector.X].Type; t != nil && isContext(t) {
					keyed = true
				}
			}
		}
		return true
	})
	return keyed
}

func identOf(expression ast.Expr) *ast.Ident {
	ident, _ := ast.Unparen(expression).(*ast.Ident)
	return ident
}

// inLoop reports whether the innermost node of stack is inside a for or range
// loop within the page body.
func inLoop(stack []ast.Node, body ast.Node) bool {
	if body == nil {
		return false
	}
	for i := len(stack) - 1; i >= 0 && stack[i] != body; i-- {
		switch stack[i].(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			return true
		}
	}
	return false
}

// retained reports struct fields and package variables of a value that belongs
// to one paint: a C, Box or Frame kept past the paint reads a reused node.
func (checker *vetChecker) retained(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				if t := checker.info.Types[field.Type].Type; t != nil && isRetainedType(t) {
					checker.report(field.Type, "kept-context", retainedCheck(t))
				}
			}
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				return true
			}
			for _, spec := range node.Specs {
				value := spec.(*ast.ValueSpec)
				for _, name := range value.Names {
					if variable, ok := checker.info.Defs[name].(*types.Var); ok && variable.Parent() == variable.Pkg().Scope() && isRetainedType(variable.Type()) {
						checker.report(name, "kept-context", retainedCheck(variable.Type()))
					}
				}
			}
		}
		return true
	})
}

// goroutines reports a per-paint value passed to, or captured by, a goroutine: the
// goroutine outlives the paint the value names.
func (checker *vetChecker) goroutines(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		statement, ok := n.(*ast.GoStmt)
		if !ok {
			return true
		}
		for _, argument := range statement.Call.Args {
			if t := checker.info.Types[argument].Type; t != nil && isRetainedType(t) {
				checker.report(argument, "kept-context", retainedCheck(t))
			}
		}
		fn, ok := ast.Unparen(statement.Call.Fun).(*ast.FuncLit)
		if !ok {
			return true
		}
		seen := map[types.Object]bool{}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			object, ok := checker.info.Uses[ident].(*types.Var)
			if !ok || object.IsField() || seen[object] || object.Pos() >= statement.Pos() {
				return true
			}
			if isRetainedType(object.Type()) {
				seen[object] = true
				checker.report(ident, "kept-context", retainedCheck(object.Type()))
			}
			return true
		})
		return true
	})
}

// report records a finding unless the line, or the one above it, says
// "viva:ignore" with the check's name.
func (checker *vetChecker) report(node ast.Node, check, message string) {
	position := checker.fset.Position(node.Pos())
	for _, group := range checker.file.Comments {
		line := checker.fset.Position(group.Pos()).Line
		if (line == position.Line || line == position.Line-1) && strings.Contains(group.Text(), "viva:ignore "+check) {
			return
		}
	}
	checker.findings = append(checker.findings, vetFinding{pos: position, check: check, message: message})
}

// checkTypes is config.Check with a panic of the importer (export data it cannot
// read) returned as an error.
func checkTypes(config types.Config, path string, fset *token.FileSet, files []*ast.File, info *types.Info) (pkg *types.Package, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%s: reading its dependencies: %v (run go tool viva vet, the viva built by the project's Go)", path, r)
		}
	}()
	return config.Check(path, fset, files, info)
}

// goEnv returns one value of go env, as the Go that builds directory has it.
func goEnv(directory, name string) (string, error) {
	command := exec.Command("go", "env", name)
	command.Dir = directory
	out, err := command.Output()
	return strings.TrimSpace(string(out)), err
}
