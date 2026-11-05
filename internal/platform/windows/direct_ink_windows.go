//go:build windows

package windows

import (
	"fmt"
	"math"
	"strings"
	"syscall"
	"unsafe"

	"github.com/vivaui/viva/internal/platform"
)

// pieceClip is the device columns a grid piece may draw in, in a label height rows high; the
// zero value, with no height, is no clip.
type pieceClip struct {
	columns platform.GridSpan
	height  int
}

// inkKey names a glyph run as drawRun draws it in a cell span: what decides how far its ink goes
// past the cells.
type inkKey struct {
	family   string
	fonts    uint32 // customFonts.revision
	fallback uintptr
	size     float32
	weight   float32
	span     float32 // the width of the cells it is drawn in, in device pixels
	italic   bool
	text     string
}

// inkCacheLimit bounds the glyph runs whose reach is kept. A terminal shows a few hundred
// distinct non-ASCII characters at one size; the cache starts over when a screen of text in a
// dozen scripts has filled it.
const inkCacheLimit = 4096

// inkFloor is the coverage, of 255, below which a pixel of a measured glyph is not ink: the
// faintest thing the eye sees at the edge of a glyph on a dark ground.
const inkFloor = 4

// ownedColumns plans the columns each run of a grid label owns: its cells, and the blank cells
// beside them that its glyphs' ink reaches into (see platform.GridOwner). It returns nil for a
// label without runs, which has no columns to share.
func (rasterizer *directRasterizer) ownedColumns(label platform.TextLabel, pieces []gridPiece, family string, fallback uintptr, size, pitch float32, width int) []platform.GridSpan {
	if len(label.Runs) == 0 {
		return nil
	}
	customFonts.RLock()
	fonts := customFonts.revision
	customFonts.RUnlock()
	reaches := false
	for _, piece := range pieces {
		// ASCII ink stays inside its cell in every font a terminal is used with, and a
		// ligature run is ASCII; only the other characters are measured.
		if piece.index < 0 || piece.run.Cells <= 0 || piece.run.Ligatures || printableASCII(piece.text) {
			continue
		}
		reach, ok := rasterizer.inkReach(inkKey{family: family, fonts: fonts, fallback: fallback, size: size, weight: piece.weight, span: piece.span, italic: piece.run.Italic, text: piece.text})
		if !ok || (reach.Left <= 0 && reach.Right <= 0) {
			continue
		}
		if !reaches {
			reaches = true
			if cap(rasterizer.ink) < len(label.Runs) {
				rasterizer.ink = make([]platform.GridInk, len(label.Runs))
			}
			rasterizer.ink = rasterizer.ink[:len(label.Runs)]
			clear(rasterizer.ink)
		}
		rasterizer.ink[piece.index] = reach
	}
	var ink []platform.GridInk
	if reaches {
		ink = rasterizer.ink
	}
	return rasterizer.owner.Plan(label.Runs, label.Value, float64(pitch), width, ink)
}

// inkReach is how far the ink of a non-ASCII piece goes past its cells, drawn where drawRun puts
// it, from the glyphs themselves.
func (rasterizer *directRasterizer) inkReach(key inkKey) (platform.GridInk, bool) {
	if reach, known := rasterizer.inkCache[key]; known {
		return reach, true
	}
	reach, err := rasterizer.measureInk(key)
	if err != nil {
		return platform.GridInk{}, false
	}
	if rasterizer.inkCache == nil || len(rasterizer.inkCache) >= inkCacheLimit {
		rasterizer.inkCache = make(map[inkKey]platform.GridInk, 64)
	}
	key.text = strings.Clone(key.text) // the label's own string would keep the whole line alive
	rasterizer.inkCache[key] = reach
	return reach, true
}

// measureInk draws the piece alone, white on black, as the grid draws it, and reads off the
// columns that have ink: what a person sees is what the run must own. Measuring the picture
// instead of asking the font for its outline keeps the filter's spill, the hinting and the
// fallback font in the answer.
func (rasterizer *directRasterizer) measureInk(key inkKey) (platform.GridInk, error) {
	format, err := rasterizer.services.formatStyle(key.family, key.size, key.weight, key.italic)
	if err != nil {
		return platform.GridInk{}, err
	}
	defer release(&format)
	setFontFallback(format, key.fallback)
	units, err := syscall.UTF16FromString(key.text)
	if err != nil {
		return platform.GridInk{}, err
	}
	var layout uintptr
	if err := hresult(rasterizer.services.createLayout(rasterizer.services.write, &units[0], uint32(len(units)-1), format, 1<<20, 1<<20, &layout)); err != nil {
		return platform.GridInk{}, fmt.Errorf("windows: lay out grid glyph: %w", err)
	}
	defer release(&layout)
	var metrics textMetrics
	if err := hresult(comCall(layout, writeGetMetrics, uintptr(unsafe.Pointer(&metrics)))); err != nil {
		return platform.GridInk{}, err
	}
	// The cell sits in the middle of the bitmap with a size of room on each side, which is
	// more than a glyph's ink goes past its advance.
	margin := int(math.Ceil(float64(key.size)))
	cell := int(math.Ceil(float64(key.span)))
	width, height := cell+2*margin, int(math.Ceil(float64(key.size)*1.6))
	scratch, err := rasterizer.opaque.bind(width, height)
	if err != nil {
		return platform.GridInk{}, err
	}
	fill(scratch, width, height, d2dColor{A: 1})
	rasterizer.opaque.begin(d2dColor{R: 1, G: 1, B: 1, A: 1}, d2dAntialiasDefault)
	x := float32(margin)
	if isSingleCharacter(key.text) {
		x += (key.span - metrics.Width) / 2
	}
	rasterizer.opaque.draw(x, 0, layout, d2dEnableColorFont)
	if err := rasterizer.opaque.end(); err != nil {
		return platform.GridInk{}, err
	}
	first, last := width, -1
	for row := 0; row < height; row++ {
		line := scratch.Pixels[row*scratch.Width*4 : row*scratch.Width*4+width*4]
		for column := 0; column < width; column++ {
			if max(line[column*4], line[column*4+1], line[column*4+2]) < inkFloor {
				continue
			}
			first, last = min(first, column), max(last, column)
		}
	}
	if last < 0 {
		return platform.GridInk{}, nil
	}
	return platform.GridInk{
		Left:  float32(max(0, margin-first)),
		Right: float32(max(0, last+1-(margin+int(math.Round(float64(key.span)))))),
	}, nil
}
