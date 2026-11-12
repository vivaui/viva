package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/tools/iconcompile"
)

// runIcons compiles SVG icon sources into one Go source file of gfx.Icon
// values: viva icons -o icons/icons.go -pkg icons icons/
func runIcons(arguments []string, output io.Writer) error {
	if len(arguments) == 1 && isHelpFlag(arguments[0]) {
		c, _ := findCommand("icons")
		printCommandHelp(output, c, nil)
		fmt.Fprint(output, "\nflags:\n  -o, --out file       output Go file (default icons.go)\n"+
			"  -pkg, --package name Go package name (default: the output directory's name)\n"+
			"  -license file        license text to put in the file header\n")
		return nil
	}
	var outputPath, packageName, licensePath string
	positional := make([]string, 0, len(arguments))
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "-o", "--out":
			if index+1 >= len(arguments) {
				return fmt.Errorf("icons: %s needs a value", arguments[index])
			}
			outputPath = arguments[index+1]
			index++
		case "-pkg", "--package":
			if index+1 >= len(arguments) {
				return fmt.Errorf("icons: %s needs a value", arguments[index])
			}
			packageName = arguments[index+1]
			index++
		case "-license":
			if index+1 >= len(arguments) {
				return fmt.Errorf("icons: %s needs a value", arguments[index])
			}
			licensePath = arguments[index+1]
			index++
		default:
			positional = append(positional, arguments[index])
		}
	}
	if len(positional) == 0 {
		return fmt.Errorf("usage: viva icons [-o file] [-pkg name] [-license file] <svg files or directories>")
	}
	if outputPath == "" {
		outputPath = "icons.go"
	}
	if packageName == "" {
		packageName = packageOf(outputPath)
	}

	files := make([]string, 0, len(positional))
	for _, argument := range positional {
		info, err := os.Stat(argument)
		if err != nil {
			return err
		}
		if info.IsDir() {
			entries, err := filepath.Glob(filepath.Join(argument, "*.svg"))
			if err != nil {
				return err
			}
			files = append(files, entries...)
			continue
		}
		if !strings.HasSuffix(argument, ".svg") {
			return fmt.Errorf("icons: %s is not an SVG file", argument)
		}
		files = append(files, argument)
	}
	if len(files) == 0 {
		return fmt.Errorf("icons: no SVG files found")
	}
	sort.Strings(files)

	license := "Icons compiled from their SVG sources."
	if licensePath != "" {
		raw, err := os.ReadFile(licensePath)
		if err != nil {
			return err
		}
		license = string(raw)
	}

	icons := make(map[string]gfx.Icon, len(files))
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		name := strings.TrimSuffix(filepath.Base(file), ".svg")
		icon, err := iconcompile.Compile(name, source)
		if err != nil {
			return err
		}
		icons[name] = icon
	}

	generated, err := iconcompile.Generate(packageName, icons, license)
	if err != nil {
		return err
	}
	if err := os.WriteFile(outputPath, generated, 0644); err != nil {
		return err
	}
	fmt.Fprintf(output, "icons: wrote %d icons to %s\n", len(icons), outputPath)
	return nil
}

// packageOf derives a package name from the output directory.
func packageOf(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "icons"
	}
	base := filepath.Base(filepath.Dir(absolute))
	if base == "" || base == "/" || base == "." {
		return "icons"
	}
	return base
}
