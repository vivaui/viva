// Package webbuild compiles a Go program for the browser and writes the
// static files a web host serves: the WebAssembly program, the Go runtime's
// loader, and optionally a page that starts it. It is host-side build tooling;
// nothing here is linked into an application.
package webbuild

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Options describe one browser build.
type Options struct {
	Context context.Context
	// Dir is the module directory the go tool runs in.
	Dir string
	// OutputDir receives the program, wasm_exec.js and, with Page, index.html.
	OutputDir string
	// Executable names the .wasm file, without its extension.
	Executable string
	// Package is the main package to build.
	Package string
	// Flags are the go build flags before -o, such as -trimpath and -ldflags.
	Flags []string
	// Tags are added with -tags.
	Tags []string
	// Environment is the base process environment. WebEnvironment makes it the
	// browser target's; nil uses os.Environ.
	Environment []string
	// Title names the generated page.
	Title string
	// Page writes index.html beside the program.
	Page bool
	// Stdout and Stderr receive the go tool's output; nil discards it.
	Stdout io.Writer
	Stderr io.Writer
}

// Result reports what the build wrote.
type Result struct {
	WasmPath string
	WasmSize int64
	GzipSize int64
}

// Build compiles the package and writes the browser files into OutputDir.
func Build(options Options) (Result, error) {
	if options.OutputDir == "" || options.Executable == "" || options.Package == "" {
		return Result{}, errors.New("web build needs an output directory, executable name and package")
	}
	ctx := options.Context
	if ctx == nil {
		ctx = context.Background()
	}
	stdout, stderr := options.Stdout, options.Stderr
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if err := os.MkdirAll(options.OutputDir, 0o755); err != nil {
		return Result{}, err
	}
	base := options.Environment
	if base == nil {
		base = os.Environ()
	}
	environment := WebEnvironment(base)
	wasm := filepath.Join(options.OutputDir, options.Executable+".wasm")
	arguments := []string{"build"}
	arguments = append(arguments, options.Flags...)
	if len(options.Tags) > 0 {
		arguments = append(arguments, "-tags", strings.Join(options.Tags, ","))
	}
	arguments = append(arguments, "-o", wasm, options.Package)
	command := exec.CommandContext(ctx, "go", arguments...)
	command.Dir = options.Dir
	command.Env = environment
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return Result{}, errors.New("the go tool was not found in PATH")
		}
		return Result{}, fmt.Errorf("go build failed: %w", err)
	}
	root, err := exec.CommandContext(ctx, "go", "env", "GOROOT").Output()
	if err != nil {
		return Result{}, fmt.Errorf("go env GOROOT: %w", err)
	}
	loader, err := os.ReadFile(filepath.Join(strings.TrimSpace(string(root)), "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		return Result{}, fmt.Errorf("reading wasm_exec.js from the Go installation: %w", err)
	}
	if err := os.WriteFile(filepath.Join(options.OutputDir, "wasm_exec.js"), loader, 0o644); err != nil {
		return Result{}, err
	}
	if options.Page {
		title := options.Title
		if title == "" {
			title = options.Executable
		}
		page, err := pageHTML(title, options.Executable+".wasm")
		if err != nil {
			return Result{}, err
		}
		if err := os.WriteFile(filepath.Join(options.OutputDir, "index.html"), page, 0o644); err != nil {
			return Result{}, err
		}
	}
	program, err := os.ReadFile(wasm)
	if err != nil {
		return Result{}, err
	}
	var compressed bytes.Buffer
	writer, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
	_, _ = writer.Write(program)
	_ = writer.Close()
	return Result{WasmPath: wasm, WasmSize: int64(len(program)), GzipSize: int64(compressed.Len())}, nil
}

// WebEnvironment returns base as the browser target's environment, with any
// existing GOOS, GOARCH and CGO_ENABLED replaced.
func WebEnvironment(base []string) []string {
	environment := make([]string, 0, len(base)+3)
	for _, entry := range base {
		if strings.HasPrefix(entry, "GOOS=") || strings.HasPrefix(entry, "GOARCH=") || strings.HasPrefix(entry, "CGO_ENABLED=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "GOOS=js", "GOARCH=wasm", "CGO_ENABLED=0")
}

// pageHTML is the page that starts the program. The title is HTML text and the
// program's file name is a JavaScript string, so each is escaped for where it
// lands.
func pageHTML(title, wasm string) ([]byte, error) {
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
