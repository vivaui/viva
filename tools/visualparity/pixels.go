//go:build darwin || windows || linux

package main

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Pixel classes. A differing pixel is classified by where it is: on an edge
// (either image has a contrast step within one pixel) a small difference is
// anti-aliasing and a large one is a shape that sits somewhere else; away from
// edges a difference is a fill that is not the same color.
const (
	classNoise = iota // differs by at most 2 levels away from an edge: color quantization
	classFlat         // differs by more away from an edge: a fill or gradient of another color
	classAA           // differs on an edge by at most aaLimit: anti-aliasing
	classShape        // differs on an edge by more: the edge is in another place
	classes
)

const (
	aaLimit   = 96
	edgeStep  = 6
	tileSize  = 16
	maskInset = 1 // logical pixels added around a text's bounds
)

var classNames = [classes]string{"noise", "flat", "aa", "shape"}

func loadPNG(path string) (*image.RGBA, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	decoded, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	if rgba, ok := decoded.(*image.RGBA); ok {
		return rgba, nil
	}
	bounds := decoded.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, decoded.At(x, y))
		}
	}
	return rgba, nil
}

// pixelResult is the comparison of one pair of images outside the text.
type pixelResult struct {
	name             string
	width, height    int
	considered       int
	differing        int
	sum              float64
	max              int
	counts           [classes]int
	worstTile        [2]int
	worstTileCount   int
	masked           int
	dimensionsDiffer bool
}

func edgeAt(img *image.RGBA, x, y int) bool {
	b := img.Bounds()
	var lo, hi [3]int
	for c := range lo {
		lo[c], hi[c] = 255, 0
	}
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			px, py := x+dx, y+dy
			if px < b.Min.X || py < b.Min.Y || px >= b.Max.X || py >= b.Max.Y {
				continue
			}
			o := img.PixOffset(px, py)
			for c := 0; c < 3; c++ {
				v := int(img.Pix[o+c])
				lo[c], hi[c] = min(lo[c], v), max(hi[c], v)
			}
		}
	}
	for c := 0; c < 3; c++ {
		if hi[c]-lo[c] > edgeStep {
			return true
		}
	}
	return false
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func textMask(width, height int, scale float64, facts ...*Facts) []bool {
	mask := make([]bool, width*height)
	for _, f := range facts {
		for _, t := range f.Texts {
			x0 := int(math.Floor(float64(t.Bounds.X-maskInset) * scale))
			y0 := int(math.Floor(float64(t.Bounds.Y-maskInset) * scale))
			x1 := int(math.Ceil(float64(t.Bounds.X+t.Bounds.W+maskInset) * scale))
			y1 := int(math.Ceil(float64(t.Bounds.Y+t.Bounds.H+maskInset) * scale))
			for y := max(y0, 0); y < min(y1, height); y++ {
				for x := max(x0, 0); x < min(x1, width); x++ {
					mask[y*width+x] = true
				}
			}
		}
	}
	return mask
}

func diffImages(name string, a, b *image.RGBA, mask []bool, scale float64) (pixelResult, *image.RGBA) {
	res := pixelResult{name: name, width: a.Bounds().Dx(), height: a.Bounds().Dy()}
	diff := image.NewRGBA(a.Bounds())
	tiles := map[[2]int]int{}
	palette := [classes]color.RGBA{{90, 90, 0, 255}, {255, 160, 0, 255}, {40, 120, 255, 255}, {255, 0, 0, 255}}
	for y := 0; y < res.height; y++ {
		for x := 0; x < res.width; x++ {
			oa, ob := a.PixOffset(x, y), b.PixOffset(x, y)
			grey := uint8((int(a.Pix[oa]) + int(a.Pix[oa+1]) + int(a.Pix[oa+2])) / 3 / 4)
			diff.SetRGBA(x, y, color.RGBA{grey, grey, grey, 255})
			if mask[y*res.width+x] {
				res.masked++
				diff.SetRGBA(x, y, color.RGBA{0, 0, 0, 255})
				continue
			}
			res.considered++
			d := 0
			for c := 0; c < 3; c++ {
				d = max(d, absInt(int(a.Pix[oa+c])-int(b.Pix[ob+c])))
			}
			res.sum += float64(d)
			if d == 0 {
				continue
			}
			res.differing++
			res.max = max(res.max, d)
			edge := edgeAt(a, x, y) || edgeAt(b, x, y)
			var class int
			switch {
			case !edge && d <= 2:
				class = classNoise
			case !edge:
				class = classFlat
			case d <= aaLimit:
				class = classAA
			default:
				class = classShape
			}
			res.counts[class]++
			diff.SetRGBA(x, y, palette[class])
			if class == classFlat || class == classShape {
				tiles[[2]int{x / tileSize, y / tileSize}]++
			}
		}
	}
	for tile, count := range tiles {
		if count > res.worstTileCount {
			res.worstTileCount, res.worstTile = count, tile
		}
	}
	return res, diff
}

func comparePixels(w io.Writer, a, b *side, out string) error {
	fmt.Fprintf(w, "\n== Pixels outside text (text bounds masked; classes: noise <=2 off an edge, flat = fill of another color, aa = edge pixel off by <=%d, shape = edge pixel off by more) ==\n", aaLimit)
	names, err := filepath.Glob(filepath.Join(a.dir, "*@*x.png"))
	if err != nil {
		return err
	}
	sort.Strings(names)
	var results []pixelResult
	for _, path := range names {
		base := filepath.Base(path)
		other := filepath.Join(b.dir, base)
		if _, err := os.Stat(other); err != nil {
			continue
		}
		imgA, err := loadPNG(path)
		if err != nil {
			return err
		}
		imgB, err := loadPNG(other)
		if err != nil {
			return err
		}
		stem := strings.TrimSuffix(base, ".png")
		key := stem[:strings.Index(stem, "@")]
		var scale float64
		fmt.Sscanf(stem[strings.Index(stem, "@")+1:], "%fx", &scale)
		if imgA.Bounds() != imgB.Bounds() {
			results = append(results, pixelResult{name: stem, dimensionsDiffer: true})
			continue
		}
		mask := textMask(imgA.Bounds().Dx(), imgA.Bounds().Dy(), scale, a.facts[key], b.facts[key])
		res, diff := diffImages(stem, imgA, imgB, mask, scale)
		results = append(results, res)
		if out != "" && res.differing > 0 {
			file, err := os.Create(filepath.Join(out, stem+".diff.png"))
			if err != nil {
				return err
			}
			png.Encode(file, diff)
			file.Close()
		}
	}
	if len(results) == 0 {
		fmt.Fprintln(w, "no PNG in common")
		return nil
	}
	fmt.Fprintf(w, "%-36s %9s %7s %6s | %8s %8s %8s %8s | worst tile (logical x,y; px)\n", "image", "differing", "mean", "max", "noise", "flat", "aa", "shape")
	var total pixelResult
	byScale := map[string]*pixelResult{}
	for _, r := range results {
		if r.dimensionsDiffer {
			fmt.Fprintf(w, "%-36s sizes differ\n", r.name)
			continue
		}
		scale := r.name[strings.Index(r.name, "@"):]
		if byScale[scale] == nil {
			byScale[scale] = &pixelResult{}
		}
		for _, t := range []*pixelResult{&total, byScale[scale]} {
			t.considered += r.considered
			t.differing += r.differing
			t.sum += r.sum
			t.max = max(t.max, r.max)
			for c := range t.counts {
				t.counts[c] += r.counts[c]
			}
		}
		worst := ""
		if r.worstTileCount > 0 {
			sc := 1.0
			fmt.Sscanf(scale, "@%fx", &sc)
			worst = fmt.Sprintf("(%d,%d) %d", int(float64(r.worstTile[0]*tileSize)/sc), int(float64(r.worstTile[1]*tileSize)/sc), r.worstTileCount)
		}
		fmt.Fprintf(w, "%-36s %8.3f%% %7.3f %6d | %8d %8d %8d %8d | %s\n", r.name, 100*float64(r.differing)/float64(max(r.considered, 1)), r.sum/float64(max(r.considered, 1)), r.max,
			r.counts[classNoise], r.counts[classFlat], r.counts[classAA], r.counts[classShape], worst)
	}
	var scales []string
	for s := range byScale {
		scales = append(scales, s)
	}
	sort.Strings(scales)
	for _, s := range scales {
		t := byScale[s]
		fmt.Fprintf(w, "TOTAL %-30s %8.3f%% %7.3f %6d | %8d %8d %8d %8d |\n", s, 100*float64(t.differing)/float64(max(t.considered, 1)), t.sum/float64(max(t.considered, 1)), t.max,
			t.counts[classNoise], t.counts[classFlat], t.counts[classAA], t.counts[classShape])
	}
	if out != "" {
		fmt.Fprintf(w, "diff PNGs (dark = unchanged, black = masked text, yellow noise, orange flat, blue aa, red shape) are in %s\n", out)
	}
	return nil
}
