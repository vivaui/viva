package main

import (
	"context"
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/vivaui/viva/internal/devreload"
)

// The recipes `viva add` installs are generated from the components package, one
// per file, together with the recipes each one calls into: componentFiles and
// componentRequires live in components_gen.go and the sources in templates.
//
//go:generate go run ../../tools/componentgen -root ../..

//go:embed templates/*.go.txt
var componentTemplates embed.FS

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "viva:", err)
		os.Exit(1)
	}
}

func run(arguments []string, output io.Writer) error {
	return runCLI(arguments, output, io.Discard)
}

func runCLI(arguments []string, output, errorOutput io.Writer) error {
	if len(arguments) == 0 {
		printOverview(errorOutput)
		return errors.New("no command given")
	}
	if isHelpFlag(arguments[0]) {
		printOverview(output)
		return nil
	}
	if arguments[0] == "--version" || arguments[0] == "-version" {
		return runVersion(nil, output)
	}
	c, ok := findCommand(arguments[0])
	if !ok {
		return fmt.Errorf("unknown command %q; run 'viva help' for the list", arguments[0])
	}
	return c.run(arguments[1:], output, errorOutput)
}

func runDev(arguments []string, output, errorOutput io.Writer) error {
	var appArguments []string
	for index, argument := range arguments {
		if argument == "--" {
			appArguments = append([]string(nil), arguments[index+1:]...)
			arguments = arguments[:index]
			break
		}
	}
	flags := flag.NewFlagSet("dev", flag.ContinueOnError)
	directory := "."
	flags.StringVar(&directory, "C", directory, "module directory")
	flags.StringVar(&directory, "dir", directory, "module directory")
	debounce := flags.Duration("debounce", 150*time.Millisecond, "change debounce interval")
	poll := flags.Duration("poll", 120*time.Millisecond, "source scan interval")
	buildTimeout := flags.Duration("build-timeout", time.Minute, "worker build timeout")
	workerTimeout := flags.Duration("worker-timeout", 8*time.Second, "worker startup and request timeout")
	c, _ := findCommand("dev")
	packages, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(packages) != 1 {
		return errors.New("dev requires exactly one Go main package; usage: viva dev [flags] ./cmd/app [-- app-args]")
	}
	if *debounce <= 0 || *poll <= 0 || *buildTimeout <= 0 || *workerTimeout <= 0 {
		return errors.New("dev durations must be positive")
	}
	if err := requireGo(errorOutput); err != nil {
		return err
	}
	return devreload.Run(context.Background(), devreload.Options{
		Directory: directory, Package: packages[0], Arguments: appArguments,
		Debounce: *debounce, PollInterval: *poll, BuildTimeout: *buildTimeout, WorkerTimeout: *workerTimeout,
		Output: output, ErrorOutput: errorOutput,
	})
}

func addComponents(arguments []string, output, errorOutput io.Writer) error {
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	destination := flags.String("out", "components", "component output directory")
	packageName := flags.String("package", "components", "generated Go package name")
	force := flags.Bool("force", false, "replace existing component files")
	c, _ := findCommand("add")
	names, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if !validPackageName(*packageName) {
		return fmt.Errorf("invalid package name %q", *packageName)
	}
	if len(names) == 0 {
		return errors.New("add requires at least one component name")
	}
	for _, name := range names {
		if _, ok := componentFiles[name]; !ok {
			return fmt.Errorf("unknown component %q; available: %s", name, strings.Join(componentNames(), ", "))
		}
	}
	warnVersionMismatch(errorOutput)
	if err := os.MkdirAll(*destination, 0o755); err != nil {
		return err
	}
	requested := make(map[string]bool, len(names))
	for _, name := range names {
		requested[name] = true
	}
	for _, name := range withRequirements(names) {
		templatePath := "templates/" + componentFiles[name]
		data, err := componentTemplates.ReadFile(templatePath)
		if err != nil {
			return err
		}
		data = []byte(strings.ReplaceAll(string(data), "{{PACKAGE}}", *packageName))
		filename := strings.ReplaceAll(name, "-", "_") + ".go"
		path := filepath.Join(*destination, filename)
		if !requested[name] && !*force {
			// A recipe pulled in only because another needs it is not worth
			// failing over, nor replacing, when it is already there.
			if _, err := os.Stat(path); err == nil {
				continue
			}
		}
		if err := writeComponent(path, data, *force); err != nil {
			return err
		}
		fmt.Fprintln(output, path)
	}
	return nil
}

// withRequirements returns names with every recipe they need added before the
// one that needs it, each listed once.
func withRequirements(names []string) []string {
	ordered := make([]string, 0, len(names))
	seen := make(map[string]bool, len(names))
	var add func(name string)
	add = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, required := range componentRequires[name] {
			add(required)
		}
		ordered = append(ordered, name)
	}
	for _, name := range names {
		add(name)
	}
	return ordered
}

func writeComponent(path string, data []byte, force bool) error {
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	file, err := os.OpenFile(path, flags, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s already exists; use --force to replace it", path)
		}
		return err
	}
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	return file.Close()
}

func componentNames() []string {
	names := make([]string, 0, len(componentFiles))
	for name := range componentFiles {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func validPackageName(value string) bool {
	if value == "" || (value[0] < 'A' || value[0] > 'Z') && (value[0] < 'a' || value[0] > 'z') && value[0] != '_' {
		return false
	}
	for index := 1; index < len(value); index++ {
		character := value[index]
		if (character < 'A' || character > 'Z') && (character < 'a' || character > 'z') &&
			(character < '0' || character > '9') && character != '_' {
			return false
		}
	}
	return true
}

// warnVersionMismatch tells the user when the project in the working directory
// builds against a different Viva than this command's recipes were written for.
func warnVersionMismatch(errorOutput io.Writer) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return
	}
	if result := vivaVersionCheck(parseVivaDependency(string(data))); result.level == checkWarn {
		fmt.Fprintf(errorOutput, "viva: warning: %s\n  %s\n", result.detail, result.fix)
	}
}
