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

// viva vet finds the mistakes in a page written with flex.Screen that compile and
// run but do the wrong thing, before the program runs: the same ones a test under
// app.Driver fails on, found without a test that reaches them. It reads the
// packages with go/types and the export data `go list -export` builds, so it needs
// nothing but the Go toolchain.

const flexPath = "github.com/vivaui/viva/flex"

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
		if path == flexPath {
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

// The Screen methods that bind a value, and which arguments are the bound values.
var boundArguments = map[string][]int{
	"Checkbox": {0}, "Switch": {0}, "Choice": {0}, "Tabs": {0}, "Select": {0, 1},
	"Field": {0}, "Slider": {0}, "Splitter": {0}, "Table": {0},
	"Popover": {1}, "PopoverWith": {2}, "Modal": {0}, "ModalWith": {1},
}

// isScreen reports whether t is flex.Screen or flex.ModalScreen, the values a page
// body is given.
func isScreen(t types.Type) bool {
	named, ok := types.Unalias(t).(*types.Named)
	if !ok || named.Obj().Pkg() == nil || named.Obj().Pkg().Path() != flexPath {
		return false
	}
	name := named.Obj().Name()
	return name == "Screen" || name == "ModalScreen"
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

// call checks one call: a value bound to a Screen control, or a Button.
func (checker *vetChecker) call(call *ast.CallExpr, stack []ast.Node) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	receiver := checker.info.Types[selector.X].Type
	if receiver == nil || !isScreen(receiver) {
		return
	}
	keyed := checker.keyed(selector.X, stack)
	body := screenBody(checker.info, stack)
	if indexes, ok := boundArguments[selector.Sel.Name]; ok && !keyed && body != nil {
		for _, index := range indexes {
			if index < len(call.Args) {
				if name, copied := checker.copied(call.Args[index], body); copied {
					checker.report(call.Args[index], "bound-copy", fmt.Sprintf(
						"%s is bound to %s, a variable made again in every paint, so the control's focus and clicks are lost and what it writes is thrown away: "+
							"bind the model's own field (a pointer into it, such as &m.items[i].Done), or declare the control inside s.Key(...) and write the copy back on Changed()",
						selector.Sel.Name, name))
				}
			}
		}
	}
	if selector.Sel.Name == "Button" && !keyed && len(call.Args) == 1 && inLoop(stack, body) {
		if value := checker.info.Types[call.Args[0]].Value; value != nil {
			checker.report(call, "unkeyed-button", fmt.Sprintf(
				"every row of this loop has a Button labeled %s and none has a Key, so a row added or removed above moves a click to the wrong row: "+
					"declare each row with r := s.Key(item.ID) (or s.KeyIndex(id)) and call r.Button", value.ExactString()))
		}
	}
}

// keyed reports whether the Screen expression is a Key or KeyIndex row: a direct
// s.Key(k) call, or a variable assigned from one.
func (checker *vetChecker) keyed(expression ast.Expr, stack []ast.Node) bool {
	expression = ast.Unparen(expression)
	if isKeyCall(expression) {
		return true
	}
	ident, ok := expression.(*ast.Ident)
	if !ok {
		return false
	}
	object := checker.info.Uses[ident]
	if object == nil {
		return false
	}
	keyed := false
	for _, node := range stack {
		ast.Inspect(node, func(n ast.Node) bool {
			if keyed {
				return false
			}
			switch assign := n.(type) {
			case *ast.AssignStmt:
				for i, left := range assign.Lhs {
					if id, ok := left.(*ast.Ident); ok && i < len(assign.Rhs) && checker.objectOf(id) == object && isKeyCall(ast.Unparen(assign.Rhs[i])) {
						keyed = true
					}
				}
			case *ast.ValueSpec:
				for i, name := range assign.Names {
					if i < len(assign.Values) && checker.objectOf(name) == object && isKeyCall(ast.Unparen(assign.Values[i])) {
						keyed = true
					}
				}
			}
			return true
		})
		if keyed {
			return true
		}
	}
	return false
}

func (checker *vetChecker) objectOf(ident *ast.Ident) types.Object {
	if object := checker.info.Defs[ident]; object != nil {
		return object
	}
	return checker.info.Uses[ident]
}

func isKeyCall(expression ast.Expr) bool {
	call, ok := expression.(*ast.CallExpr)
	if !ok {
		return false
	}
	selector, ok := call.Fun.(*ast.SelectorExpr)
	return ok && (selector.Sel.Name == "Key" || selector.Sel.Name == "KeyIndex")
}

// screenBody is the innermost function around the call that takes a Screen: the
// body of the page, which runs in every paint.
func screenBody(info *types.Info, stack []ast.Node) ast.Node {
	for i := len(stack) - 1; i >= 0; i-- {
		var fields *ast.FieldList
		switch function := stack[i].(type) {
		case *ast.FuncDecl:
			fields = function.Type.Params
		case *ast.FuncLit:
			fields = function.Type.Params
		default:
			continue
		}
		for _, field := range fields.List {
			if t := info.Types[field.Type].Type; t != nil && isScreen(t) {
				return stack[i]
			}
		}
	}
	return nil
}

// copied reports whether &x names storage made in every paint: a variable declared
// in the page body (a range variable, a v := copy), or a part of one reached
// without going through a pointer or a slice.
func (checker *vetChecker) copied(argument ast.Expr, body ast.Node) (string, bool) {
	unary, ok := ast.Unparen(argument).(*ast.UnaryExpr)
	if !ok || unary.Op != token.AND {
		return "", false
	}
	expression := ast.Unparen(unary.X)
	for {
		switch part := expression.(type) {
		case *ast.SelectorExpr:
			base := checker.info.Types[part.X].Type
			if base == nil {
				return "", false
			}
			if _, pointer := base.Underlying().(*types.Pointer); pointer {
				return "", false
			}
			if _, isPackage := checker.info.Uses[identOf(part.X)].(*types.PkgName); isPackage {
				return "", false
			}
			expression = ast.Unparen(part.X)
		case *ast.IndexExpr:
			base := checker.info.Types[part.X].Type
			if base == nil {
				return "", false
			}
			if _, array := base.Underlying().(*types.Array); !array {
				return "", false // a slice's elements live in its backing array, not in the variable
			}
			expression = ast.Unparen(part.X)
		case *ast.Ident:
			variable, ok := checker.info.Uses[part].(*types.Var)
			if !ok || variable.Parent() == nil || variable.Parent() == variable.Pkg().Scope() {
				return "", false
			}
			if variable.Pos() < body.Pos() || variable.Pos() >= body.End() {
				return "", false // declared outside the page body: kept between paints
			}
			return checker.source(unary.X), true
		default:
			return "", false
		}
	}
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

// retained reports struct fields and package variables of type flex.Screen: a
// Screen is the page being declared, valid only inside its body. Keep the
// *flex.Screen NewScreen returns instead.
func (checker *vetChecker) retained(file *ast.File) {
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.StructType:
			for _, field := range node.Fields.List {
				if t := checker.info.Types[field.Type].Type; t != nil && isScreen(t) {
					checker.report(field.Type, "kept-screen", "a flex.Screen is kept in a struct, but it is valid only inside the body it was given to: "+
						"keep the *flex.Screen that NewScreen returns, or pass the Screen down as a parameter")
				}
			}
		case *ast.GenDecl:
			if node.Tok != token.VAR {
				return true
			}
			for _, spec := range node.Specs {
				value := spec.(*ast.ValueSpec)
				for _, name := range value.Names {
					if variable, ok := checker.info.Defs[name].(*types.Var); ok && variable.Parent() == variable.Pkg().Scope() && isScreen(variable.Type()) {
						checker.report(name, "kept-screen", "a flex.Screen is kept in a package variable, but it is valid only inside the body it was given to: "+
							"keep the *flex.Screen that NewScreen returns, or pass the Screen down as a parameter")
					}
				}
			}
		}
		return true
	})
}

func (checker *vetChecker) source(expression ast.Expr) string {
	var buffer bytes.Buffer
	start, end := checker.fset.Position(expression.Pos()), checker.fset.Position(expression.End())
	if data, err := os.ReadFile(start.Filename); err == nil && end.Offset <= len(data) {
		buffer.Write(data[start.Offset:end.Offset])
	}
	if buffer.Len() == 0 {
		return "a local variable"
	}
	return "&" + buffer.String()
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
