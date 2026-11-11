// Command cellatlas compares the cell glyphs of gfx with a reference sprite atlas, character by
// character.
//
//	go run ./tools/cellatlas -ref ../ghostty/src/font/sprite/testdata
//
// A reference atlas is a PNG named after the characters it holds, U+1FB00...U+1FBFF-12x24+3.png:
// 256 sprites in a 16 by 16 grid, each a cell of 12x24 device pixels with a rule 3 thick and a
// quarter cell of padding around it, which is what ghostty's sprite font draws. The command draws
// the same grid from gfx's geometry, compares the two, and names the characters that differ.
//
// A character that gfx leaves to the font is not a difference: those are listed apart, so a
// deliberate gap stays visible instead of hiding in the count. Run with -show to see one
// character's two sprites side by side.
//
// A character gfx deliberately draws differently does differ: Braille spreads its dots evenly over
// the cell where the reference leaves the wider gap inside it, so most patterns land a pixel off.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "cellatlas:", err)
		os.Exit(1)
	}
}

// run compares every atlas the flags in args ask for.
func run(args []string) error {
	flags := flag.NewFlagSet("cellatlas", flag.ExitOnError)
	ref := flags.String("ref", "", "directory of reference atlases, ghostty's src/font/sprite/testdata")
	cell := flags.String("cell", "", "only atlases of this cell, as widthxheight+rule, e.g. 12x24+3")
	block := flags.String("block", "", "only the atlas of this block, as the code point it starts at, e.g. 1FB00")
	show := flags.String("show", "", "print this character's sprites instead of the summary, e.g. U+1FB98")
	threshold := flags.Int("threshold", 40, "difference in a pixel's gray level that counts as one")
	worst := flags.Int("worst", 8, "how many characters of an atlas to name")
	samples := flags.Int("samples", 24, "samples per axis of a pixel filled by a triangle")
	write := flags.String("write", "", "directory to write our atlases and the difference images to")
	flags.Parse(args)

	if *ref == "" {
		return fmt.Errorf("-ref is the directory of the reference atlases")
	}
	atlases, err := findAtlases(*ref)
	if err != nil {
		return err
	}
	if len(atlases) == 0 {
		return fmt.Errorf("no atlas named U+xxxx...U+yyyy-WxH+r.png in %s", *ref)
	}
	var shown, blocked rune
	if *show != "" {
		if shown, err = codePoint(*show); err != nil {
			return fmt.Errorf("-show: %w", err)
		}
	}
	if *block != "" {
		if blocked, err = codePoint(*block); err != nil {
			return fmt.Errorf("-block: %w", err)
		}
	}
	if *write != "" {
		if err := os.MkdirAll(*write, 0o755); err != nil {
			return err
		}
	}
	found := false
	for _, a := range atlases {
		if *cell != "" && a.cell() != *cell {
			continue
		}
		if blocked != 0 && a.first != blocked {
			continue
		}
		if shown != 0 && (shown < a.first || shown > a.first+0xFF) {
			continue
		}
		reference, err := a.load()
		if err != nil {
			return err
		}
		if shown != 0 {
			found = true
			ours := newSprite(a, *samples)
			ours.drawAt(int(shown - a.first))
			ours.show(reference, shown, *threshold)
			continue
		}
		ours := newSprite(a, *samples)
		ours.draw()
		drawn := ours.image()
		report(a, ours, compare(drawn, reference, a, *threshold), *worst)
		if *write != "" {
			if err := writeImages(*write, a, drawn, reference); err != nil {
				return err
			}
		}
	}
	if shown != 0 && !found {
		return fmt.Errorf("no atlas in %s holds U+%04X", *ref, shown)
	}
	return nil
}

// codePoint parses a character a flag names, as U+1FB98 or 1fb98.
func codePoint(value string) (rune, error) {
	number, err := strconv.ParseUint(strings.TrimPrefix(strings.ToUpper(value), "U+"), 16, 32)
	if err != nil {
		return 0, fmt.Errorf("%q is not a code point, as U+1FB98", value)
	}
	return rune(number), nil
}

// atlas is one reference PNG: the characters it holds, and the cell it draws them in.
type atlas struct {
	path                string
	first               rune
	width, height, rule int
}

// cell is the atlas's cell as widthxheight+rule, the way its name says it.
func (a atlas) cell() string {
	return fmt.Sprintf("%dx%d+%d", a.width, a.height, a.rule)
}

// label is the atlas in a line of output.
func (a atlas) label() string {
	return fmt.Sprintf("%s U+%04X", a.cell(), a.first)
}

// stripe is how many pixels an atlas cell takes, its padding included.
func (a atlas) stripe() (x, y int) {
	return a.width + 2*(a.width/4), a.height + 2*(a.height/4)
}

// findAtlases is every reference atlas in a directory, in the order of their names.
func findAtlases(dir string) ([]atlas, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.png"))
	if err != nil {
		return nil, err
	}
	var atlases []atlas
	for _, name := range names {
		atlas, ok := parseAtlas(name)
		if ok {
			atlases = append(atlases, atlas)
		}
	}
	sort.Slice(atlases, func(i, j int) bool { return atlases[i].first < atlases[j].first })
	return atlases, nil
}

// parseAtlas reads the name of a reference atlas, U+1FB00...U+1FBFF-12x24+3.png, and reports
// whether the name is one.
func parseAtlas(path string) (atlas, bool) {
	var first, last rune
	var width, height, rule int
	_, err := fmt.Sscanf(filepath.Base(path), "U+%X...U+%X-%dx%d+%d.png",
		&first, &last, &width, &height, &rule)
	if err != nil || width <= 0 || height <= 0 || rule <= 0 || last-first != 0xFF {
		return atlas{}, false
	}
	return atlas{path: path, first: first, width: width, height: height, rule: rule}, true
}
