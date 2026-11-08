//go:build darwin || windows || linux

// Command visualparity measures how alike Viva looks on macOS, Windows and Linux.
//
// run renders a fixed catalogue of scenes with the platform's own offscreen
// renderer and writes, for each, the layout facts the canvas captured (JSON) and
// the pixels (PNG); compare reads the outputs of two platforms and reports where
// they drift. See docs/conformance.md, "Visual parity".
//
//	go run -tags vivahead ./tools/visualparity run -out /tmp/parity-mac        # macOS needs vivahead
//	go run ./tools/visualparity run -out /tmp/parity-win -scales 1,2          # Windows
//	go run ./tools/visualparity run -out /tmp/parity-linux -scales 1,1.5,2    # Linux; fractional scales are allowed
//	go run ./tools/visualparity compare -out /tmp/parity-diff /tmp/parity-mac /tmp/parity-win
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		usage()
	}
	var err error
	switch os.Args[1] {
	case "run":
		err = run(os.Args[2:])
	case "compare":
		err = compare(os.Args[2:])
	default:
		usage()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "visualparity:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: visualparity run -out DIR [-scales 1,2] [-only NAME] [-theme light|dark] [-images=false]")
	fmt.Fprintln(os.Stderr, "       visualparity compare [-out DIFFDIR] [-pixels=false] A B")
	os.Exit(2)
}

func run(args []string) error {
	flags := flag.NewFlagSet("run", flag.ExitOnError)
	out := flags.String("out", "", "output directory")
	scales := flags.String("scales", "1,2", "comma-separated backing scales, device pixels per logical pixel (1, 1.5, 2 ...)")
	only := flags.String("only", "", "only scenes whose name contains this")
	theme := flags.String("theme", "", "only the light or the dark theme")
	images := flags.Bool("images", true, "render PNGs (layout facts are written either way)")
	flags.Parse(args)
	if *out == "" {
		return fmt.Errorf("-out is required")
	}
	options := runOptions{out: *out, only: *only, images: *images, themeOnly: *theme}
	for _, part := range strings.Split(*scales, ",") {
		scale, err := strconv.ParseFloat(strings.TrimSpace(part), 64)
		if err != nil || scale < 1 || scale > 8 {
			return fmt.Errorf("bad scale %q", part)
		}
		options.scales = append(options.scales, scale)
	}
	return runCommand(options)
}
