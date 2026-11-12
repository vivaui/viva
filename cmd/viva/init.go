package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

//go:embed initfiles/*.txt
var initFiles embed.FS

// initOutputs maps each embedded file to the name it has in a new project.
var initOutputs = [...]struct{ embedded, name string }{
	{"main.go.txt", "main.go"},
	{"main_test.go.txt", "main_test.go"},
	{"viva.toml.txt", "viva.toml"},
	{"AGENTS.md.txt", "AGENTS.md"},
}

// runInit creates a runnable project: a window with a counter laid out with
// github.com/vivaui/viva/ui, a test that drives it, viva.toml, and AGENTS.md, the short guide a person
// or a coding assistant reads before changing the UI.
func runInit(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("init", flag.ContinueOnError)
	source := flags.String("viva", "", "directory of the Viva source to build against (default: the checkout containing the working directory, when there is one)")
	tidy := flags.Bool("tidy", true, "run go mod tidy after writing the files")
	c, _ := findCommand("init")
	names, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(names) != 1 {
		return errors.New("init requires one project directory; usage: viva init [flags] <directory>")
	}
	directory := names[0]
	module := filepath.Base(directory)
	if !validModuleName(module) {
		return fmt.Errorf("%q is not usable as a Go module name; use letters, digits, - and _", module)
	}
	viva, err := vivaSource(*source)
	if err != nil {
		return err
	}
	if entries, err := os.ReadDir(directory); err == nil && len(entries) > 0 {
		return fmt.Errorf("%s is not empty", directory)
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	id := strings.ToLower(strings.NewReplacer("_", "-", " ", "-").Replace(module))
	replacements := strings.NewReplacer("{{NAME}}", module, "{{ID}}", id)
	for _, file := range initOutputs {
		data, err := initFiles.ReadFile("initfiles/" + file.embedded)
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(replacements.Replace(string(data))), 0o644); err != nil {
			return err
		}
	}
	goMod := fmt.Sprintf("module %s\n\ngo 1.26\n", module)
	if viva != "" {
		// A checkout is in play, so the project builds against it directly.
		goMod += fmt.Sprintf("\nrequire github.com/vivaui/viva v0.0.0\n\nreplace github.com/vivaui/viva => %s\n", viva)
	} else {
		// The project takes Viva from the module path, and pins the viva command
		// to the same version: go tool viva runs exactly the library's release,
		// so the recipes viva add writes are the ones that library accepts.
		goMod += "\ntool " + vivaModule + "/cmd/viva\n"
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(goMod), 0o644); err != nil {
		return err
	}
	if *tidy {
		if err := requireGo(os.Stderr); err != nil {
			return err
		}
		command := exec.Command("go", "mod", "tidy")
		command.Dir = directory
		if combined, err := command.CombinedOutput(); err != nil {
			fmt.Fprintf(output, "go mod tidy failed (%v):\n%s\nRun it yourself in %s once the modules can be fetched.\n", err, combined, directory)
		}
	}
	fmt.Fprintf(output, "Created %s.\n  cd %s && viva dev .     run it with hot reload\n  go test ./...          test it without a window\n  AGENTS.md              how to write the UI (for you and for a coding assistant)\n", directory, directory)
	if viva == "" {
		fmt.Fprintln(output, "  go tool viva ...       the viva command pinned to this project's Viva version")
	}
	return nil
}

func validModuleName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_') {
			return false
		}
	}
	return true
}

// vivaSource finds the Viva checkout a new project should build against: the one
// given, else the nearest directory above the working directory whose go.mod
// declares module github.com/vivaui/viva.
func vivaSource(given string) (string, error) {
	if given != "" {
		return checkVivaSource(given)
	}
	directory, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if root, err := checkVivaSource(directory); err == nil {
			return root, nil
		}
		parent := filepath.Dir(directory)
		if parent == directory {
			// Running from an installed viva, with no checkout to point at: the
			// project takes Viva from the module path like any other dependency.
			return "", nil
		}
		directory = parent
	}
}

func checkVivaSource(directory string) (string, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(filepath.Join(absolute, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("%s has no go.mod", absolute)
	}
	if first, _, _ := strings.Cut(string(data), "\n"); strings.TrimSpace(first) != "module github.com/vivaui/viva" {
		return "", fmt.Errorf("%s is not the Viva source (its go.mod does not say module github.com/vivaui/viva)", absolute)
	}
	return absolute, nil
}
