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
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
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

// call checks one call: a Button of package ui with a label that is the same in every
// row of a loop whose rows have no Key.
func (checker *vetChecker) call(call *ast.CallExpr, stack []ast.Node) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Button" || len(call.Args) != 2 {
		return
	}
	if pkg, ok := checker.info.Uses[identOf(selector.X)].(*types.PkgName); !ok || pkg.Imported().Path() != uiPath {
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

// retained reports struct fields and package variables of type ui.C: a C is the
// page being declared, valid only inside its function. Keep the *ui.Page
// NewPage returns instead.
func (checker *vetChecker) retained(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				if t := checker.info.Types[field.Type].Type; t != nil && isContext(t) {
					checker.report(field.Type, "kept-context", "a ui.C is kept in a struct, but it is valid only inside the page function it was given to: "+
						"keep the *ui.Page that NewPage returns, or pass c down as a parameter")
				}
			}
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				return true
			}
			for _, spec := range node.Specs {
				value := spec.(*ast.ValueSpec)
				for _, name := range value.Names {
					if variable, ok := checker.info.Defs[name].(*types.Var); ok && variable.Parent() == variable.Pkg().Scope() && isContext(variable.Type()) {
						checker.report(name, "kept-context", "a ui.C is kept in a package variable, but it is valid only inside the page function it was given to: "+
							"keep the *ui.Page that NewPage returns, or pass c down as a parameter")
					}
				}
			}
		}
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
