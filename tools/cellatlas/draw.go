package main

import (
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// sprite is a whole reference atlas drawn from gfx's geometry: how much of each pixel a cell glyph
// fills, from 0 to 1.
type sprite struct {
	atlas         atlas
	width, height int
	padX, padY    int
	coverage      []float64
	samples       int
	canvas        gfx.Canvas
	skipped       []rune // the characters gfx leaves to the font
}

func newSprite(a atlas, samples int) *sprite {
	strideX, strideY := a.stripe()
	return &sprite{
		atlas:    a,
		width:    16 * strideX,
		height:   16 * strideY,
		padX:     a.width / 4,
		padY:     a.height / 4,
		coverage: make([]float64, 16*strideX*16*strideY),
		samples:  max(samples, 1),
	}
}

// draw fills the atlas from the geometry of every character it holds.
func (s *sprite) draw() {
	for index := 0; index < 256; index++ {
		s.drawAt(index)
	}
}

// drawAt fills the atlas cell at index from the geometry of the character it holds.
func (s *sprite) drawAt(index int) {
	r := s.atlas.first + rune(index)
	if !gfx.IsCellGlyph(r) {
		s.skipped = append(s.skipped, r)
		return
	}
	s.add(index, s.shapes(r))
}

// shapes is what gfx draws for r in a cell of the atlas's size: device pixels from the cell's
// corner. The rule a cell glyph is drawn with is 5% of the text's size until a text engine reports
// the underline thickness (see gfx cellLine), so the atlas's rule asks for the size that gives it.
func (s *sprite) shapes(r rune) gfx.CellGlyphShapes {
	value := string(r)
	bounds := gfx.Rect{Width: float32(s.atlas.width), Height: float32(s.atlas.height)}
	s.canvas.Reset()
	s.canvas.LabelRuns(value, bounds, gfx.TextStyle{Size: float32(s.atlas.rule) * 20}, bounds.Width,
		[]gfx.TextRun{{Bytes: int32(len(value)), Cells: 1, Color: gfx.Color{Alpha: 1}}})
	texts := s.canvas.Texts()
	return s.canvas.CellGlyphGeometry(gfx.CellGlyphShapes{}, texts[len(texts)-1])
}

// add fills the atlas cell at index from shapes, in the atlas's pixels.
func (s *sprite) add(index int, shapes gfx.CellGlyphShapes) {
	strideX, strideY := s.atlas.stripe()
	originX, originY := (index%16)*strideX+s.padX, (index/16)*strideY+s.padY
	// The shapes of one cell glyph are drawn once each, side by side or overlapping: their coverage
	// adds up and is clamped to a whole pixel rather than compositing over itself.
	add := func(x, y int, cover float64) {
		s.coverage[y*s.width+x] += cover
	}
	for _, rectangle := range shapes.Rectangles {
		bounds := rectangle.Bounds
		alpha := float64(rectangle.Color.Alpha)
		left, top, right, bottom := s.clip(float64(bounds.X), float64(bounds.Y),
			float64(bounds.X+bounds.Width), float64(bounds.Y+bounds.Height))
		for y := int(math.Floor(top)); y < int(math.Ceil(bottom)); y++ {
			for x := int(math.Floor(left)); x < int(math.Ceil(right)); x++ {
				columns := math.Min(right, float64(x+1)) - math.Max(left, float64(x))
				rows := math.Min(bottom, float64(y+1)) - math.Max(top, float64(y))
				if columns > 0 && rows > 0 {
					add(originX+x, originY+y, columns*rows*alpha)
				}
			}
		}
	}
	for _, triangle := range shapes.Triangles {
		s.triangle(originX, originY, triangle)
	}
}

// clip cuts a box of the atlas to what a shape may draw in: a quarter cell past its own, the
// padding the reference atlas gives it, and nothing past the sprite.
func (s *sprite) clip(left, top, right, bottom float64) (float64, float64, float64, float64) {
	return max(left, -float64(s.padX)), max(top, -float64(s.padY)),
		min(right, float64(s.atlas.width+s.padX)), min(bottom, float64(s.atlas.height+s.padY))
}

// triangle adds the coverage of one triangle to the atlas cell at a corner: a pixel counts the
// samples of it the triangle holds.
func (s *sprite) triangle(originX, originY int, triangle gfx.Triangle) {
	a, b, c := triangle.Points[0], triangle.Points[1], triangle.Points[2]
	// One side of the triangle as seen from a point: on one side of the line through them or the
	// other, whatever the triangle's winding.
	side := func(from, to gfx.Point, x, y float64) float64 {
		return (float64(to.X)-float64(from.X))*(y-float64(from.Y)) -
			(float64(to.Y)-float64(from.Y))*(x-float64(from.X))
	}
	left, top, right, bottom := s.clip(
		math.Min(float64(a.X), math.Min(float64(b.X), float64(c.X))),
		math.Min(float64(a.Y), math.Min(float64(b.Y), float64(c.Y))),
		math.Max(float64(a.X), math.Max(float64(b.X), float64(c.X))),
		math.Max(float64(a.Y), math.Max(float64(b.Y), float64(c.Y))))
	alpha := float64(triangle.Color.Alpha)
	samples := float64(s.samples * s.samples)
	for y := int(math.Floor(top)); y < int(math.Ceil(bottom)); y++ {
		for x := int(math.Floor(left)); x < int(math.Ceil(right)); x++ {
			inside := 0
			for row := 0; row < s.samples; row++ {
				for column := 0; column < s.samples; column++ {
					px := float64(x) + (float64(column)+0.5)/float64(s.samples)
					py := float64(y) + (float64(row)+0.5)/float64(s.samples)
					one, two, three := side(a, b, px, py), side(b, c, px, py), side(c, a, px, py)
					if (one >= 0 && two >= 0 && three >= 0) || (one <= 0 && two <= 0 && three <= 0) {
						inside++
					}
				}
			}
			if inside > 0 {
				s.coverage[(originY+y)*s.width+originX+x] += float64(inside) / samples * alpha
			}
		}
	}
}

// image is the atlas as an image, a pixel's value the share of it the glyphs fill.
func (s *sprite) image() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, s.width, s.height))
	for index, cover := range s.coverage {
		img.Pix[index] = uint8(math.Round(math.Min(cover, 1) * 255))
	}
	return img
}

// glyphDiff is how one character of an atlas differs: the pixels that differ by more than the
// threshold, and the worst of them.
type glyphDiff struct {
	r             rune
	pixels, worst int
}

// compare is the characters of an atlas whose two drawings differ, the worst first.
func compare(ours, reference *image.Gray, a atlas, threshold int) []glyphDiff {
	strideX, strideY := a.stripe()
	var diffs []glyphDiff
	for index := 0; index < 256; index++ {
		r := a.first + rune(index)
		if !gfx.IsCellGlyph(r) {
			continue
		}
		diff := glyphDiff{r: r}
		for y := (index / 16) * strideY; y < (index/16+1)*strideY; y++ {
			for x := (index % 16) * strideX; x < (index%16+1)*strideX; x++ {
				at := ours.PixOffset(x, y)
				difference := abs(int(ours.Pix[at]) - int(reference.Pix[reference.PixOffset(x, y)]))
				if difference > threshold {
					diff.pixels++
				}
				diff.worst = max(diff.worst, difference)
			}
		}
		if diff.pixels > 0 {
			diffs = append(diffs, diff)
		}
	}
	sort.Slice(diffs, func(i, j int) bool {
		if diffs[i].pixels != diffs[j].pixels {
			return diffs[i].pixels > diffs[j].pixels
		}
		return diffs[i].worst > diffs[j].worst
	})
	return diffs
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

// report prints what differs in one atlas.
func report(a atlas, s *sprite, diffs []glyphDiff, worst int) {
	fmt.Printf("%s: %d of %d drawn differ", a.label(), len(diffs), 256-len(s.skipped))
	if len(s.skipped) > 0 {
		fmt.Printf("; the font draws %s", spans(s.skipped))
	}
	fmt.Println()
	for index, diff := range diffs {
		if index == worst {
			fmt.Printf("  ... %d more\n", len(diffs)-worst)
			break
		}
		fmt.Printf("  U+%04X  %d px  max %d\n", diff.r, diff.pixels, diff.worst)
	}
}

// spans is a run of characters as the ranges it holds, "U+1FBBD-U+1FBBF, U+1FBE0".
func spans(characters []rune) string {
	var parts []string
	for index := 0; index < len(characters); {
		end := index
		for end+1 < len(characters) && characters[end+1] == characters[end]+1 {
			end++
		}
		if end == index {
			parts = append(parts, fmt.Sprintf("U+%04X", characters[index]))
		} else {
			parts = append(parts, fmt.Sprintf("U+%04X-U+%04X", characters[index], characters[end]))
		}
		index = end + 1
	}
	return strings.Join(parts, ", ")
}

// show prints one character's sprite as the reference draws it, as gfx draws it, and where the two
// differ.
func (s *sprite) show(reference *image.Gray, r rune, threshold int) {
	index := int(r - s.atlas.first)
	strideX, strideY := s.atlas.stripe()
	ours := s.image()
	left, top := (index%16)*strideX, (index/16)*strideY
	ink := func(value uint8) byte {
		switch {
		case value < 40:
			return ' '
		case value < 120:
			return '.'
		case value < 210:
			return 'o'
		}
		return '#'
	}
	fmt.Printf("%s  U+%04X\n", s.atlas.label(), r)
	fmt.Println("  reference                    gfx                          difference")
	for y := top; y < top+strideY; y++ {
		var drawn, mine, differ []byte
		for x := left; x < left+strideX; x++ {
			theirs := reference.Pix[reference.PixOffset(x, y)]
			theirsDrawn := ours.Pix[ours.PixOffset(x, y)]
			drawn = append(drawn, ink(theirs))
			mine = append(mine, ink(theirsDrawn))
			if abs(int(theirs)-int(theirsDrawn)) > threshold {
				differ = append(differ, '#')
			} else {
				differ = append(differ, ' ')
			}
		}
		fmt.Printf("  %s   %s   %s\n", drawn, mine, differ)
	}
}

// writeImages saves our atlas, and the difference between the two, as PNGs named after the
// reference's.
func writeImages(dir string, a atlas, ours, reference *image.Gray) error {
	name := strings.TrimSuffix(filepath.Base(a.path), ".png")
	if err := writePNG(filepath.Join(dir, "ours-"+name+".png"), ours); err != nil {
		return err
	}
	differ := image.NewGray(ours.Bounds())
	for index := range differ.Pix {
		difference := abs(int(ours.Pix[index]) - int(reference.Pix[index]))
		differ.Pix[index] = uint8(min(difference*2, 255))
	}
	return writePNG(filepath.Join(dir, "diff-"+name+".png"), differ)
}

// writePNG writes an image to a file.
func writePNG(path string, img image.Image) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return png.Encode(file, img)
}

// load is the reference atlas as an image, a pixel's value the share of it the glyphs fill.
func (a atlas) load() (*image.Gray, error) {
	file, err := os.Open(a.path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	source, err := png.Decode(file)
	if err != nil {
		return nil, err
	}
	gray, ok := source.(*image.Gray)
	if !ok {
		gray = image.NewGray(source.Bounds())
		draw.Draw(gray, gray.Bounds(), source, source.Bounds().Min, draw.Src)
	}
	strideX, strideY := a.stripe()
	if gray.Bounds().Dx() != 16*strideX || gray.Bounds().Dy() != 16*strideY {
		return nil, fmt.Errorf("%s is %v, not 16 sprites of %dx%d", a.path, gray.Bounds(), a.width, a.height)
	}
	return gray, nil
}
