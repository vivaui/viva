package windows

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
)

// A terminal row of many colors is drawn as one run per color. Rasterizing it
// would otherwise make a DirectWrite text format and a text layout per run, and
// a screen being scrolled into view is dozens of such rows: the frame blows its
// budget exactly on the colored stretches. Both objects are immutable once the
// run's text, font and decorations are fixed, so the rasterizer keeps them and a
// repeated run costs a map lookup, as a text engine's layout cache does.

// gridFormatKey names a fixed-pitch text format. fonts is the registered-fonts
// revision: a font registration can change which face a family resolves to.
type gridFormatKey struct {
	family   string
	size     float32
	weight   uint32 // the DirectWrite weight the run is drawn at
	italic   bool
	fallback uintptr
	fonts    uint32
}

// gridFormatLimit bounds the formats kept. A terminal uses a handful of weights
// and sizes; past the limit the cache starts over rather than growing forever.
const gridFormatLimit = 64

// gridLayoutKey names a shaped run: everything drawRun passes to DirectWrite
// that changes the layout, its metrics, or the character spacing applied to it.
type gridLayoutKey struct {
	format    gridFormatKey
	text      string
	span      float32 // device pixels the run covers
	pitch     float32 // device pixels per cell
	ligatures bool
	underline bool
	strike    bool
}

// gridLayoutLimit bounds the shaped runs kept. A terminal draws its rows from a
// small alphabet, so a screen of colored text fits many times over.
const gridLayoutLimit = 4096

// gridLayout is a shaped run and the metrics drawRun places it with.
type gridLayout struct {
	layout  uintptr
	metrics textMetrics
	line    lineMetrics
	chars   uint32 // UTF-16 units the run's decorations cover
}

// gridFormat returns the text format for a run, making it once. The handle is
// owned by the rasterizer: callers must not release it.
func (rasterizer *directRasterizer) gridFormat(family string, size, weight float32, italic bool, fallback uintptr) (gridFormatKey, uintptr, error) {
	customFonts.RLock()
	fonts := customFonts.revision
	customFonts.RUnlock()
	key := gridFormatKey{
		family: family, size: size, weight: rasterizer.services.monoWeight(weight),
		italic: italic, fallback: fallback, fonts: fonts,
	}
	if format, ok := rasterizer.formats[key]; ok {
		return key, format, nil
	}
	format, err := rasterizer.services.formatWeight(family, size, key.weight, italic)
	if err != nil {
		return key, 0, err
	}
	setFontFallback(format, fallback)
	if rasterizer.formats == nil || len(rasterizer.formats) >= gridFormatLimit {
		for _, old := range rasterizer.formats {
			release(&old)
		}
		rasterizer.formats = make(map[gridFormatKey]uintptr, gridFormatLimit)
	}
	rasterizer.formats[key] = format
	return key, format, nil
}

// gridShaped returns the shaped run for piece, making it once. The layout is
// owned by the rasterizer: callers must not release it.
func (rasterizer *directRasterizer) gridShaped(format gridFormatKey, handle uintptr, piece string, run gfx.TextRun, span, pitch float32) (*gridLayout, error) {
	key := gridLayoutKey{
		format: format, text: piece, span: span, pitch: pitch,
		ligatures: run.Ligatures, underline: run.Underline, strike: run.Strike,
	}
	if shaped, ok := rasterizer.shaped[key]; ok {
		return shaped, nil
	}
	units, err := syscall.UTF16FromString(piece)
	if err != nil {
		return nil, err
	}
	var layout uintptr
	if err := hresult(rasterizer.services.createLayout(rasterizer.services.write, &units[0], uint32(len(units)-1), handle, 1<<20, 1<<20, &layout)); err != nil {
		return nil, fmt.Errorf("windows: lay out grid text: %w", err)
	}
	shaped := &gridLayout{layout: layout, chars: uint32(len(units) - 1)}
	ok := false
	defer func() {
		if !ok {
			release(&layout)
		}
	}()
	if err := hresult(comCall(layout, writeGetMetrics, uintptr(unsafe.Pointer(&shaped.metrics)))); err != nil {
		return nil, err
	}
	var lines uint32
	if err := hresult(comCall(layout, writeGetLineMetrics, uintptr(unsafe.Pointer(&shaped.line)), 1, uintptr(unsafe.Pointer(&lines)))); err != nil && lines == 0 {
		return nil, err
	}
	all := uint64(shaped.chars) << 32
	if run.Underline {
		comCall(layout, writeSetUnderline, 1, uintptr(all))
	}
	if run.Strike {
		comCall(layout, writeSetStrikethrough, 1, uintptr(all))
	}
	if !run.Ligatures && pitch > 0 && printableASCII(piece) {
		// Every character stands on its cell, whatever the advance of the font.
		advance := shaped.metrics.WidthTrailing / float32(shaped.chars)
		if reduction := advance - pitch; reduction != 0 {
			rasterizer.setCharacterSpacing(layout, -reduction, all)
		}
	}
	if rasterizer.shaped == nil || len(rasterizer.shaped) >= gridLayoutLimit {
		for _, old := range rasterizer.shaped {
			release(&old.layout)
		}
		rasterizer.shaped = make(map[gridLayoutKey]*gridLayout, gridLayoutLimit)
	}
	rasterizer.shaped[key] = shaped
	ok = true
	return shaped, nil
}

// setCharacterSpacing gives a layout direct-write's per-range character spacing.
func (rasterizer *directRasterizer) setCharacterSpacing(layout uintptr, trailing float32, span uint64) {
	var extended uintptr
	if comCall(layout, 0, uintptr(unsafe.Pointer(&writeLayout1IID)), uintptr(unsafe.Pointer(&extended))) != 0 || extended == 0 {
		return
	}
	defer release(&extended)
	if rasterizer.spacing == nil {
		purego.RegisterFunc(&rasterizer.spacing, vtable(extended, writeSetCharacterSpacing))
	}
	rasterizer.spacing(extended, 0, trailing, 0, span)
}

// releaseGridCache frees the formats and layouts the rasterizer keeps.
func (rasterizer *directRasterizer) releaseGridCache() {
	for _, format := range rasterizer.formats {
		release(&format)
	}
	for _, shaped := range rasterizer.shaped {
		release(&shaped.layout)
	}
	rasterizer.formats, rasterizer.shaped = nil, nil
}
