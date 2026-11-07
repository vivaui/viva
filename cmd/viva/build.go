package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/tools/manifest"
)

const mebibyte = 1 << 20

var (
	buildTargets = []string{"darwin", "windows", "linux", "web"}
	buildArches  = []string{"arm64", "amd64"}
)

// buildPlan is everything needed to run one go build.
type buildPlan struct {
	manifest *manifest.Manifest
	// pkg is the main package, as the go tool takes it.
	pkg    string
	target string
	arch   string
	tags   []string
	// out is the absolute output: a file, or for the web target a directory.
	out        string
	executable string
	debug      bool
}

// goTarget is the GOOS/GOARCH pair for a target.
func (plan buildPlan) goTarget() (goos, goarch string) {
	if plan.target == "web" {
		return "js", "wasm"
	}
	return plan.target, plan.arch
}

// ldflags are the linker flags: stripped unless debug, a fixed build ID so the
// same sources give the same bytes, and on Windows the GUI subsystem for
// release builds (a debug build keeps the console so output is visible).
func (plan buildPlan) ldflags() string {
	flags := []string{"-s", "-w", "-buildid="}
	if plan.debug {
		flags = []string{"-buildid="}
	}
	if plan.target == "windows" && !plan.debug {
		flags = append(flags, "-H=windowsgui")
	}
	return strings.Join(flags, " ")
}

// arguments are the go build arguments.
func (plan buildPlan) arguments(output string) []string {
	arguments := []string{"build", "-trimpath", "-ldflags", plan.ldflags()}
	if len(plan.tags) > 0 {
		arguments = append(arguments, "-tags", strings.Join(plan.tags, ","))
	}
	return append(arguments, "-o", output, plan.pkg)
}

// environment is the process environment for go build: the target, and on
// macOS the deployment target for the Objective-C bridge. Without it the
// program is built for the macOS of the machine that built it and refuses to
// start on older systems.
func (plan buildPlan) environment(base []string) []string {
	goos, goarch := plan.goTarget()
	environment := append([]string(nil), base...)
	environment = append(environment, "GOOS="+goos, "GOARCH="+goarch)
	if plan.target == "darwin" {
		// The Metal bridge is Objective-C. Go turns cgo off when GOARCH differs
		// from the host's, but on a Mac clang builds either architecture.
		environment = append(environment, "CGO_ENABLED=1")
		flag := "-mmacosx-version-min=" + plan.manifest.Mac.MinSystem
		for _, name := range []string{"CGO_CFLAGS", "CGO_LDFLAGS"} {
			value := flag
			if existing := lastEnvironment(base, name); existing != "" {
				value = existing + " " + flag
			}
			environment = append(environment, name+"="+value)
		}
	}
	return environment
}

func lastEnvironment(environment []string, name string) string {
	value := ""
	for _, entry := range environment {
		if strings.HasPrefix(entry, name+"=") {
			value = strings.TrimPrefix(entry, name+"=")
		}
	}
	return value
}

// run builds into output.
func (plan buildPlan) run(output string, errorOutput io.Writer) error {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := requireGo(errorOutput); err != nil {
		return err
	}
	command := exec.Command("go", plan.arguments(output)...)
	command.Dir = plan.manifest.Dir
	command.Env = plan.environment(os.Environ())
	command.Stdout, command.Stderr = errorOutput, errorOutput
	if err := command.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return errors.New("the go tool was not found in PATH")
		}
		return fmt.Errorf("go build failed: %w", err)
	}
	return plan.addWindowsResources(output, errorOutput)
}

// buildFlags holds the flags build and package share.
type buildFlags struct {
	dir    string
	out    string
	target string
	arch   string
	tags   string
	debug  bool
}

func (shared *buildFlags) register(flags *flag.FlagSet, defaultTarget string) {
	flags.StringVar(&shared.dir, "C", ".", "directory holding viva.toml and the Go module")
	flags.StringVar(&shared.dir, "dir", ".", "same as -C")
	flags.StringVar(&shared.target, "target", defaultTarget, "darwin, windows, linux or web")
	flags.StringVar(&shared.arch, "arch", runtime.GOARCH, "arm64 or amd64 (not used by web)")
	flags.StringVar(&shared.tags, "tags", "", "comma-separated build tags, added to [build] tags")
	flags.BoolVar(&shared.debug, "debug", false, "keep symbols and debug information (not the release size)")
}

// newPlan resolves flags, viva.toml and the package argument into a plan.
func newPlan(shared buildFlags, packageArgument string) (buildPlan, error) {
	if !contains(buildTargets, shared.target) {
		return buildPlan{}, fmt.Errorf("unknown target %q; use one of: %s", shared.target, strings.Join(buildTargets, ", "))
	}
	if shared.target != "web" && !contains(buildArches, shared.arch) {
		return buildPlan{}, fmt.Errorf("unknown architecture %q; use one of: %s", shared.arch, strings.Join(buildArches, ", "))
	}
	if shared.target == "darwin" && runtime.GOOS != "darwin" {
		return buildPlan{}, errors.New("building for darwin needs a macOS host: the Metal bridge is Objective-C and cannot be cross-compiled")
	}
	loaded, err := manifest.Load(shared.dir)
	if err != nil {
		return buildPlan{}, err
	}
	plan := buildPlan{manifest: loaded, target: shared.target, arch: shared.arch, debug: shared.debug}

	plan.pkg = packageArgument
	if plan.pkg == "" {
		plan.pkg = loaded.App.Package
		if shared.target == "web" {
			plan.pkg = loaded.Web.Package
			if plan.pkg == "" {
				return buildPlan{}, fmt.Errorf("the web target needs a browser entry point: set [web] package in %s or name the package", manifest.FileName)
			}
		}
	}

	seen := map[string]bool{}
	for _, list := range [][]string{loaded.Build.Tags, strings.Split(shared.tags, ",")} {
		for _, tag := range list {
			if tag = strings.TrimSpace(tag); tag != "" && !seen[tag] {
				seen[tag] = true
				plan.tags = append(plan.tags, tag)
			}
		}
	}

	plan.executable = executableName(loaded, plan.pkg)
	return plan, nil
}

// executableName is the manifest's choice, or when the project file says nothing
// about the application, the name of the package being built.
func executableName(loaded *manifest.Manifest, pkg string) string {
	if loaded.App.Executable != "" || loaded.App.Name != "" || loaded.App.ID != "" {
		return loaded.ExecutableName()
	}
	base := filepath.Base(pkg)
	if pkg == "." || pkg == "./" {
		if absolute, err := filepath.Abs(loaded.Dir); err == nil {
			base = filepath.Base(absolute)
		}
	}
	if slug := manifest.Slug(base); slug != "" {
		return slug
	}
	return "app"
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// outputPath decides where a build goes. -o names a file (or an existing
// directory to put the program in; for the web target the directory itself);
// without it the program goes to [build] out, with the target in its name when
// it is not the machine's own so cross builds do not overwrite each other.
func (plan *buildPlan) outputPath(requested string) error {
	name := plan.executable
	switch {
	case plan.target == "web":
		if requested != "" {
			absolute, err := filepath.Abs(requested)
			plan.out = absolute
			return err
		}
		plan.out = filepath.Join(plan.manifest.Path(plan.manifest.Build.Out), "web")
	default:
		if plan.target != runtime.GOOS || plan.arch != runtime.GOARCH {
			name += "-" + plan.target + "-" + plan.arch
		}
		if plan.target == "windows" {
			name += ".exe"
		}
		if requested != "" {
			absolute, err := filepath.Abs(requested)
			if err != nil {
				return err
			}
			if info, statErr := os.Stat(absolute); statErr == nil && info.IsDir() {
				absolute = filepath.Join(absolute, name)
			}
			plan.out = absolute
			return nil
		}
		plan.out = filepath.Join(plan.manifest.Path(plan.manifest.Build.Out), name)
	}
	if !filepath.IsAbs(plan.out) {
		absolute, err := filepath.Abs(plan.out)
		if err != nil {
			return err
		}
		plan.out = absolute
	}
	return nil
}

func runBuild(arguments []string, output, errorOutput io.Writer) error {
	c, _ := findCommand("build")
	flags := flag.NewFlagSet("build", flag.ContinueOnError)
	var shared buildFlags
	shared.register(flags, runtime.GOOS)
	var requested string
	flags.StringVar(&requested, "o", "", "output file (the web target: output directory); default is [build] out")
	flags.StringVar(&requested, "out", "", "same as -o")
	positional, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(positional) > 1 {
		return errors.New("build takes at most one package; run 'viva help build'")
	}
	argument := ""
	if len(positional) == 1 {
		argument = positional[0]
	}
	plan, err := newPlan(shared, argument)
	if err != nil {
		return err
	}
	if err := plan.outputPath(requested); err != nil {
		return err
	}
	if plan.target == "web" {
		return buildWeb(plan, output, errorOutput)
	}
	if err := plan.run(plan.out, errorOutput); err != nil {
		return err
	}
	info, err := os.Stat(plan.out)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "built %s\n", displayPath(plan.out))
	printBuildDetails(output, plan, info.Size())
	return nil
}

// displayPath shows a path relative to the working directory when it is inside.
func displayPath(path string) string {
	if working, err := os.Getwd(); err == nil {
		if relative, err := filepath.Rel(working, path); err == nil && !strings.HasPrefix(relative, "..") {
			return relative
		}
	}
	return path
}

func printBuildDetails(output io.Writer, plan buildPlan, size int64) {
	goos, goarch := plan.goTarget()
	target := goos + "/" + goarch
	if plan.target == "darwin" {
		target += " (macOS " + plan.manifest.Mac.MinSystem + " or later)"
	}
	fmt.Fprintf(output, "  target   %s\n", target)
	if len(plan.tags) > 0 {
		fmt.Fprintf(output, "  tags     %s\n", strings.Join(plan.tags, ","))
	}
	fmt.Fprintf(output, "  size     %s%s\n", formatSize(size), sizeNote(plan, size))
}

func sizeNote(plan buildPlan, size int64) string {
	switch {
	case plan.debug:
		return ", with symbols (not the release size)"
	}
	return ", stripped"
}

// formatSize writes bytes with thousands separators and in MiB.
func formatSize(size int64) string {
	digits := fmt.Sprint(size)
	var grouped []string
	for len(digits) > 3 {
		grouped = append([]string{digits[len(digits)-3:]}, grouped...)
		digits = digits[:len(digits)-3]
	}
	grouped = append([]string{digits}, grouped...)
	return fmt.Sprintf("%s B (%.2f MiB)", strings.Join(grouped, ","), float64(size)/mebibyte)
}

// webPage renders the page that starts a WebAssembly program. The title is HTML
// text and the program's file name is a JavaScript string, so each is escaped
// for where it lands.
func webPage(title, wasm string) ([]byte, error) {
	quoted, err := json.Marshal(wasm)
	if err != nil {
		return nil, err
	}
	var page bytes.Buffer
	fmt.Fprintf(&page, `<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1, viewport-fit=cover" />
  <title>%s</title>
  <style>
    html, body { margin: 0; height: 100%%; background: #0e1117; overscroll-behavior: none; }
    body { color-scheme: dark; }
    canvas { display: block; touch-action: none; -webkit-tap-highlight-color: transparent; }
  </style>
</head>
<body>
  <script src="wasm_exec.js"></script>
  <script>
    const go = new Go();
    async function start() {
      let result;
      try {
        result = await WebAssembly.instantiateStreaming(fetch(%s), go.importObject);
      } catch (streamingError) {
        // Static hosts often serve .wasm as application/octet-stream. Fall back
        // to bytes instead of failing the whole page on a MIME mismatch.
        const response = await fetch(%s);
        if (!response.ok) throw new Error("HTTP " + response.status + " for " + %s);
        result = await WebAssembly.instantiate(await response.arrayBuffer(), go.importObject);
      }
      await go.run(result.instance);
    }
    start().catch((e) => {
      document.body.textContent = "Failed to load the application: " + e;
    });
  </script>
</body>
</html>
`, html.EscapeString(title), quoted, quoted, quoted)
	return page.Bytes(), nil
}

// buildWeb writes the browser build: the wasm program, the Go runtime's loader
// script and a page that starts it.
func buildWeb(plan buildPlan, output, errorOutput io.Writer) error {
	wasm := filepath.Join(plan.out, plan.executable+".wasm")
	if err := plan.run(wasm, errorOutput); err != nil {
		return err
	}
	root, err := exec.Command("go", "env", "GOROOT").Output()
	if err != nil {
		return fmt.Errorf("go env GOROOT: %w", err)
	}
	loader, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(root)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return fmt.Errorf("reading wasm_exec.js from the Go installation: %w", err)
	}
	if err := os.WriteFile(filepath.Join(plan.out, "wasm_exec.js"), loader, 0o644); err != nil {
		return err
	}
	title := plan.manifest.App.Name
	if title == "" {
		title = plan.executable
	}
	page, err := webPage(title, plan.executable+".wasm")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(plan.out, "index.html"), page, 0o644); err != nil {
		return err
	}

	program, err := os.ReadFile(wasm)
	if err != nil {
		return err
	}
	var compressed bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	writer.Write(program)
	writer.Close()

	fmt.Fprintf(output, "built %s/\n", displayPath(plan.out))
	fmt.Fprintf(output, "  files    %s.wasm, wasm_exec.js, index.html\n", plan.executable)
	if len(plan.tags) > 0 {
		fmt.Fprintf(output, "  tags     %s\n", strings.Join(plan.tags, ","))
	}
	note := ""
	if plan.debug {
		note = ", with symbols (not the release size)"
	}
	fmt.Fprintf(output, "  size     %s%s\n", formatSize(int64(len(program))), note)
	fmt.Fprintf(output, "  gzip     %s\n", formatSize(int64(compressed.Len())))
	return nil
}
