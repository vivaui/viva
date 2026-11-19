package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/vivaui/viva/tools/manifest"
)

type checkLevel int

const (
	checkOK checkLevel = iota
	checkNote
	checkWarn
	checkFail
)

func (level checkLevel) label() string {
	return [...]string{"ok", "note", "warn", "FAIL"}[level]
}

type checkResult struct {
	level  checkLevel
	name   string
	detail string
	fix    string
}

// doctorHost is everything the checks ask the machine, so a test can answer
// for a machine it is not running on.
type doctorHost struct {
	goos, goarch string
	lookPath     func(name string) (string, error)
	capture      func(name string, arguments ...string) (string, error)
	getenv       func(name string) string
	readFile     func(path string) ([]byte, error)
	cacheDir     string
}

func realDoctorHost() doctorHost {
	return doctorHost{
		goos: runtime.GOOS, goarch: runtime.GOARCH,
		lookPath: exec.LookPath,
		capture: func(name string, arguments ...string) (string, error) {
			command := exec.Command(name, arguments...)
			// Ask the go binary about itself, not about a toolchain a go.mod here would switch to.
			command.Env = append(os.Environ(), "GOTOOLCHAIN=local")
			out, err := command.CombinedOutput()
			return string(out), err
		},
		getenv:   os.Getenv,
		readFile: os.ReadFile,
		cacheDir: goCacheDirectory(),
	}
}

func runDoctor(arguments []string, output io.Writer) error {
	flags := flag.NewFlagSet("doctor", flag.ContinueOnError)
	directory := "."
	flags.StringVar(&directory, "C", directory, "project directory")
	c, _ := findCommand("doctor")
	rest, helped, err := parseFlags(c, flags, arguments, output)
	if helped || err != nil {
		return err
	}
	if len(rest) != 0 {
		return errors.New("doctor takes no arguments; usage: viva doctor [-C dir]")
	}
	return doctor(realDoctorHost(), directory, output)
}

func doctor(host doctorHost, directory string, output io.Writer) error {
	results := host.check(directory)
	failures, warnings := 0, 0
	for _, result := range results {
		fmt.Fprintf(output, "  %-4s  %-10s %s\n", result.level.label(), result.name, result.detail)
		if result.fix != "" && result.level >= checkNote {
			fmt.Fprintf(output, "        %-10s %s\n", "", result.fix)
		}
		switch result.level {
		case checkFail:
			failures++
		case checkWarn:
			warnings++
		}
	}
	switch {
	case failures > 0:
		return fmt.Errorf("%d problem(s) must be fixed before viva can build here", failures)
	case warnings > 0:
		fmt.Fprintf(output, "\n%d warning(s); the rest is ready.\n", warnings)
	default:
		fmt.Fprintln(output, "\nEverything viva needs is here.")
	}
	return nil
}

func (host doctorHost) check(directory string) []checkResult {
	results := []checkResult{{level: checkOK, name: "viva",
		detail: fmt.Sprintf("%s (%s %s/%s)", cliVersion(), runtime.Version(), host.goos, host.goarch)}}
	results = append(results, host.checkGo())
	results = append(results, host.checkPlatform()...)
	results = append(results, host.checkPackaging()...)
	results = append(results, host.checkProject(directory)...)
	return results
}

func (host doctorHost) checkGo() checkResult {
	fix := fmt.Sprintf("install Go 1.%d or newer from https://go.dev/dl/", minGoMinor)
	if path := host.getenv("VIVA_GO"); path != "" {
		return checkResult{level: checkOK, name: "go", detail: "VIVA_GO=" + path}
	}
	if path, err := host.lookPath("go"); err == nil {
		out, runErr := host.capture(path, "env", "GOVERSION")
		version := strings.TrimSpace(out)
		if runErr == nil && goVersionAtLeast(version, minGoMinor) {
			return checkResult{level: checkOK, name: "go", detail: version + " (" + path + ")"}
		}
		if runErr == nil {
			if root := cachedToolchainIn(host.cacheDir); root != "" {
				return checkResult{level: checkNote, name: "go", detail: fmt.Sprintf("%s in PATH is too old; viva uses %s", version, filepath.Base(root))}
			}
			if host.getenv("VIVA_GO_DOWNLOAD") == "0" {
				return checkResult{level: checkFail, name: "go", detail: version + " in PATH is too old and downloading is off", fix: fix}
			}
			return checkResult{level: checkNote, name: "go", detail: fmt.Sprintf("%s in PATH is too old; viva downloads Go 1.%d on first use", version, minGoMinor)}
		}
	}
	if root := cachedToolchainIn(host.cacheDir); root != "" {
		return checkResult{level: checkOK, name: "go", detail: filepath.Base(root) + " (downloaded by viva, " + root + ")"}
	}
	if host.getenv("VIVA_GO_DOWNLOAD") == "0" {
		return checkResult{level: checkFail, name: "go", detail: "not found and downloading is off (VIVA_GO_DOWNLOAD=0)", fix: fix}
	}
	return checkResult{level: checkNote, name: "go", detail: fmt.Sprintf("not found; viva downloads Go 1.%d on first build, dev or init", minGoMinor)}
}

func cachedToolchainIn(cache string) string {
	if cache == "" {
		return ""
	}
	return cachedToolchain(cache)
}

func (host doctorHost) checkPlatform() []checkResult {
	switch host.goos {
	case "darwin":
		var results []checkResult
		if out, err := host.capture("xcode-select", "-p"); err != nil {
			results = append(results, checkResult{level: checkFail, name: "xcode", detail: "the command-line tools are not installed; Viva reaches Cocoa and Metal through cgo",
				fix: "run: xcode-select --install"})
		} else {
			results = append(results, checkResult{level: checkOK, name: "xcode", detail: strings.TrimSpace(out)})
		}
		if path, err := host.lookPath("clang"); err != nil {
			results = append(results, checkResult{level: checkFail, name: "clang", detail: "not found", fix: "run: xcode-select --install"})
		} else {
			results = append(results, checkResult{level: checkOK, name: "clang", detail: path})
		}
		return results
	case "linux":
		out, err := host.capture("ldconfig", "-p")
		if err != nil {
			return []checkResult{{level: checkNote, name: "libraries", detail: "could not list shared libraries (ldconfig -p); XCB and Vulkan are loaded at run time"}}
		}
		var results []checkResult
		for _, library := range []struct {
			file, packages string
			optional       bool
		}{
			{"libxcb.so.1", "libxcb1", false},
			{"libvulkan.so.1", "libvulkan1 (and a Vulkan driver such as mesa-vulkan-drivers)", true},
		} {
			if strings.Contains(out, library.file) {
				results = append(results, checkResult{level: checkOK, name: library.file, detail: "found"})
				continue
			}
			if library.optional {
				results = append(results, checkResult{level: checkNote, name: library.file,
					detail: "not found; windows draw on the CPU without GPU acceleration",
					fix:    "optional: Debian/Ubuntu: sudo apt install " + library.packages})
				continue
			}
			results = append(results, checkResult{level: checkWarn, name: library.file, detail: "not found; windows cannot open without it",
				fix: "Debian/Ubuntu: sudo apt install " + library.packages})
		}
		return results
	}
	return nil
}

// checkPackaging reports the tools only some packaging targets need. A missing
// one is a warning, never a failure: most projects never use it.
func (host doctorHost) checkPackaging() []checkResult {
	type tool struct{ name, use, fix string }
	var tools []tool
	if host.goos == "darwin" {
		tools = append(tools,
			tool{"hdiutil", "viva package --format dmg", ""},
			tool{"codesign", "macOS signing", "run: xcode-select --install"})
	}
	tools = append(tools, tool{"makensis", "viva package --target windows (NSIS installer)",
		"install NSIS (Linux: apt install nsis; Windows: winget install NSIS.NSIS); the Homebrew build on Apple Silicon crashes"})
	if host.goos != "windows" {
		tools = append(tools, tool{"osslsigncode", "signing a Windows program or installer off Windows", "install osslsigncode"})
	}
	var results []checkResult
	for _, t := range tools {
		if path, err := host.lookPath(t.name); err == nil {
			results = append(results, checkResult{level: checkOK, name: t.name, detail: path})
		} else {
			results = append(results, checkResult{level: checkNote, name: t.name, detail: "not found; only needed for " + t.use, fix: t.fix})
		}
	}
	return results
}

func (host doctorHost) checkProject(directory string) []checkResult {
	goModPath := filepath.Join(directory, "go.mod")
	data, err := host.readFile(goModPath)
	if err != nil {
		return []checkResult{{level: checkNote, name: "project", detail: "no go.mod in " + directory + "; run viva doctor inside a project to check it"}}
	}
	var results []checkResult
	loaded, err := manifest.Load(directory)
	switch {
	case err != nil:
		results = append(results, checkResult{level: checkFail, name: manifest.FileName, detail: err.Error()})
	case !loaded.Found:
		results = append(results, checkResult{level: checkNote, name: manifest.FileName, detail: "none; viva build and package use defaults"})
	default:
		results = append(results, checkResult{level: checkOK, name: manifest.FileName, detail: "valid"})
	}
	dep := parseVivaDependency(string(data))
	results = append(results, vivaVersionCheck(dep))
	return results
}

// vivaVersionCheck compares the command's version with the one the project
// builds against. viva add copies recipes embedded in the command, so a command
// older or newer than the library writes code the library may not accept.
func vivaVersionCheck(dep vivaDependency) checkResult {
	const name = "viva lib"
	switch {
	case dep.Replaced:
		return checkResult{level: checkNote, name: name, detail: "replaced by a local checkout; the command and library are not version-matched"}
	case dep.Version == "":
		return checkResult{level: checkNote, name: name, detail: "this project does not require " + vivaModule}
	case !releaseVersion(cliVersion()) || dep.Version == cliVersion():
		return checkResult{level: checkOK, name: name, detail: dep.Version + " (this command is " + cliVersion() + ")"}
	}
	fix := "pin the command to the project: go get -tool " + vivaModule + "/cmd/viva@" + dep.Version + " and run go tool viva"
	if dep.Tool {
		fix = "run the project's own copy: go tool viva ..."
	}
	return checkResult{level: checkWarn, name: name,
		detail: fmt.Sprintf("the project uses %s but this command is %s; viva add would write recipes for a different library", dep.Version, cliVersion()),
		fix:    fix}
}
