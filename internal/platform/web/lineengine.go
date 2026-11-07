package web

import (
	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/grapheme"
	"github.com/vivaui/viva/internal/textcache"
	"github.com/vivaui/viva/ui"
)

// The web text engine. The browser measures a line's width (Canvas 2D
// measureText, what fillText draws) and, the first time a caret, a selection or a click
// asks about it, the rectangle of each grapheme cluster (a DOM Range over the same text
// and font: the browser's own shaping, ligatures and bidirectional order). Everything
// else is arithmetic on those rectangles, so a line that has been measured costs no call
// into JavaScript and allocates nothing.
//
// This file has no JavaScript in it, so the geometry is tested on the host; domMeasurer
// (measure_js.go) is the browser.

// lineMeasurer is the browser's text layout.
type lineMeasurer interface {
	// width is the advance of value drawn in the font of size and weight, with rtl its
	// paragraph direction.
	width(value string, size, weight float32, rtl bool) float32
	// rects fills lefts[i] and rights[i] with the horizontal extent, from the left end
	// of the line, of the UTF-16 range [bounds[i], bounds[i+1]) of value.
	rects(value string, size, weight float32, rtl bool, bounds []int32, lefts, rights []float32)
	// setInvalidate gives the measurer the function to call when measurements made so
	// far may no longer hold (a font finished loading).
	setInvalidate(invalidate func())
}

type styledLineMeasurer interface {
	widthStyle(value string, size, weight float32, italic, rtl bool) float32
	rectsStyle(value string, size, weight float32, italic, rtl bool, bounds []int32, lefts, rights []float32)
}

type lineRasterizer interface {
	rasterizeText(value string, style ui.TextStyle, scale float32, width, height int) (ui.Bitmap, error)
}

// lineEngine implements ui.TextEngine over a lineMeasurer.
type lineEngine struct {
	measurer    lineMeasurer
	lines       *textcache.Cache[*measuredLine]
	italicLines *textcache.Cache[*measuredLine]
	resolver    bidi.Resolver
	levels      []uint8
	generation  uint32
}

// interfaceFamily is the CSS family named for proportional text, or "" for the
// browser's system stack. fontGeneration moves whenever the family changes or a
// registered font arrives, so every engine still holding shaped lines knows to
// forget them.
var interfaceFamily string
var fontGeneration uint32

// lineCacheLimit is how many lines a generation holds (twice that many are kept).
const lineCacheLimit = 512

func newLineEngine(measurer lineMeasurer) *lineEngine {
	engine := &lineEngine{measurer: measurer, lines: textcache.New[*measuredLine](lineCacheLimit, nil), generation: fontGeneration}
	measurer.setInvalidate(func() {
		engine.lines.Clear()
		if engine.italicLines != nil {
			engine.italicLines.Clear()
		}
	})
	return engine
}

// SetFontFamily implements ui.FontFamilyAware. The family applies to every
// engine in the page, as the native engines apply theirs to the process.
func (engine *lineEngine) SetFontFamily(family string) {
	if family == interfaceFamily {
		return
	}
	interfaceFamily = family
	fontGeneration++
}

// FontGeneration implements ui.FontFamilyAware.
func (engine *lineEngine) FontGeneration() uint32 { return fontGeneration }

// syncGeneration drops shaped lines when the page's font state moved since
// this engine last measured.
func (engine *lineEngine) syncGeneration() {
	if engine.generation == fontGeneration {
		return
	}
	engine.generation = fontGeneration
	engine.lines.Clear()
	if engine.italicLines != nil {
		engine.italicLines.Clear()
	}
}

func (engine *lineEngine) RasterizeText(value string, style ui.TextStyle, scale float32, width, height int) (ui.Bitmap, error) {
	rasterizer, ok := engine.measurer.(lineRasterizer)
	if !ok {
		return ui.Bitmap{}, ui.ErrTextRasterizerUnavailable
	}
	return rasterizer.rasterizeText(value, style, scale, width, height)
}

type measuredLine struct {
	width float32
	rtl   bool
	// clusters is built when a caret, selection or click first asks about the line.
	clusters *clusterRects
}

// clusterRects is the geometry of a line cluster by cluster. starts has one more entry
// than there are clusters: len(value).
type clusterRects struct {
	starts        []int32
	left, right   []float32
	rtl           []bool
	endsAtRTLEdge bool // the last cluster is right-to-left: the end of the line is its left edge
}

func (engine *lineEngine) line(value string, size, weight float32) *measuredLine {
	return engine.lineStyle(value, size, weight, false)
}

func (engine *lineEngine) lineStyle(value string, size, weight float32, italic bool) *measuredLine {
	engine.syncGeneration()
	lines := engine.lines
	if italic {
		if engine.italicLines == nil {
			engine.italicLines = textcache.New[*measuredLine](lineCacheLimit, nil)
		}
		lines = engine.italicLines
	}
	key := textcache.Key{Value: value, Size: size, Weight: weight}
	if line, ok := lines.Get(key); ok {
		return line
	}
	rtl := bidi.BaseDirection(value) == bidi.RTL
	var width float32
	if styled, ok := engine.measurer.(styledLineMeasurer); ok && italic {
		width = styled.widthStyle(value, size, weight, italic, rtl)
	} else {
		width = engine.measurer.width(value, size, weight, rtl)
	}
	line := &measuredLine{width: width, rtl: rtl}
	lines.Put(key, line)
	return line
}

func (engine *lineEngine) geometry(value string, size, weight float32, line *measuredLine) *clusterRects {
	return engine.geometryStyle(value, size, weight, false, line)
}

func (engine *lineEngine) geometryStyle(value string, size, weight float32, italic bool, line *measuredLine) *clusterRects {
	engine.syncGeneration()
	if line.clusters != nil {
		return line.clusters
	}
	var starts []int32
	for at := 0; at < len(value); at = grapheme.Next(value, at) {
		starts = append(starts, int32(at))
	}
	count := len(starts)
	starts = append(starts, int32(len(value)))

	// One pass over the runes gives each cluster its UTF-16 start (the browser's
	// offsets) and the index of its first rune (the bidi levels').
	bounds := make([]int32, count+1)
	first := make([]int, count)
	units, runeIndex, cluster := int32(0), 0, 0
	for at, r := range value {
		if cluster < count && int(starts[cluster]) == at {
			bounds[cluster], first[cluster] = units, runeIndex
			cluster++
		}
		units += utf16Units(r)
		runeIndex++
	}
	bounds[count] = units
	rects := &clusterRects{starts: starts, left: make([]float32, count), right: make([]float32, count), rtl: make([]bool, count)}
	if styled, ok := engine.measurer.(styledLineMeasurer); ok && italic {
		styled.rectsStyle(value, size, weight, italic, line.rtl, bounds, rects.left, rects.right)
	} else {
		engine.measurer.rects(value, size, weight, line.rtl, bounds, rects.left, rects.right)
	}

	direction := bidi.LTR
	if line.rtl {
		direction = bidi.RTL
	}
	paragraph := engine.resolver.Resolve(value, direction)
	engine.levels = engine.resolver.LineLevels(paragraph, 0, len(value), engine.levels)
	for cluster := range count {
		if first[cluster] < len(engine.levels) {
			rects.rtl[cluster] = engine.levels[first[cluster]]&1 == 1
		}
	}
	if count > 0 {
		rects.endsAtRTLEdge = rects.rtl[count-1]
	}
	line.clusters = rects
	return rects
}

func utf16Units(r rune) int32 {
	if r >= 0x10000 {
		return 2
	}
	return 1
}

// cluster is the index of the cluster that contains byte offset.
func (rects *clusterRects) cluster(offset int) int {
	low, high := 0, len(rects.starts)-1
	for low < high {
		middle := int(uint(low+high) >> 1)
		if int(rects.starts[middle+1]) <= offset {
			low = middle + 1
		} else {
			high = middle
		}
	}
	return low
}

// caret is the x of the caret at a cluster boundary: the leading edge of the cluster that
// starts there, the trailing edge of the last one at the end of the line.
func (rects *clusterRects) caret(cluster int) float32 {
	if cluster >= len(rects.left) {
		if rects.endsAtRTLEdge {
			return rects.left[len(rects.left)-1]
		}
		return rects.right[len(rects.right)-1]
	}
	if rects.rtl[cluster] {
		return rects.right[cluster]
	}
	return rects.left[cluster]
}

// Width implements ui.TextEngine.
func (engine *lineEngine) Width(value string, size, weight float32) float32 {
	return engine.WidthStyle(value, size, weight, false)
}

func (engine *lineEngine) WidthStyle(value string, size, weight float32, italic bool) float32 {
	if value == "" {
		return 0
	}
	return engine.lineStyle(value, size, weight, italic).width
}

// Caret implements ui.TextEngine.
func (engine *lineEngine) Caret(value string, size, weight float32, offset int) float32 {
	return engine.CaretStyle(value, size, weight, false, offset)
}

func (engine *lineEngine) CaretStyle(value string, size, weight float32, italic bool, offset int) float32 {
	if value == "" {
		return 0
	}
	rects := engine.geometryStyle(value, size, weight, italic, engine.lineStyle(value, size, weight, italic))
	offset = max(0, min(offset, len(value)))
	if offset == len(value) {
		return rects.caret(len(rects.left))
	}
	return rects.caret(rects.cluster(offset))
}

// Selection implements ui.TextEngine. The clusters of [start, end) are joined where the
// display puts them side by side, so a selection that crosses a change of direction is
// several spans, left to right.
func (engine *lineEngine) Selection(dst []ui.TextSpan, value string, size, weight float32, start, end int) []ui.TextSpan {
	return engine.SelectionStyle(dst, value, size, weight, false, start, end)
}

func (engine *lineEngine) SelectionStyle(dst []ui.TextSpan, value string, size, weight float32, italic bool, start, end int) []ui.TextSpan {
	if value == "" {
		return dst
	}
	rects := engine.geometryStyle(value, size, weight, italic, engine.lineStyle(value, size, weight, italic))
	first, last := rects.boundary(start, len(value)), rects.boundary(end, len(value))
	if first > last {
		first, last = last, first
	}
	base := len(dst)
	for cluster := first; cluster < last; cluster++ {
		left, right := rects.left[cluster], rects.right[cluster]
		if count := len(dst); count > base {
			previous := &dst[count-1]
			if abuts(previous.X+previous.Width, left) {
				previous.Width = right - previous.X
				continue
			}
			if abuts(right, previous.X) {
				previous.Width += previous.X - left
				previous.X = left
				continue
			}
		}
		dst = append(dst, ui.TextSpan{X: left, Width: right - left})
	}
	// Visual order, then join spans that touch after it.
	added := dst[base:]
	for i := 1; i < len(added); i++ {
		for j := i; j > 0 && added[j].X < added[j-1].X; j-- {
			added[j], added[j-1] = added[j-1], added[j]
		}
	}
	for i := 1; i < len(added); {
		if previous := &added[i-1]; abuts(previous.X+previous.Width, added[i].X) {
			previous.Width = added[i].X + added[i].Width - previous.X
			copy(added[i:], added[i+1:])
			added = added[:len(added)-1]
			dst = dst[:base+len(added)]
			continue
		}
		i++
	}
	return dst
}

// boundary is the index of the cluster that starts at or contains offset, len(clusters)
// for the end of the line.
func (rects *clusterRects) boundary(offset, length int) int {
	if offset >= length {
		return len(rects.left)
	}
	return rects.cluster(max(offset, 0))
}

// abuts reports whether two edges are the same pixel, as the neighbouring clusters of a
// run are: the browser reports each from the same advance.
func abuts(a, b float32) bool {
	return a-b < 0.25 && b-a < 0.25
}

// OffsetAt implements ui.TextEngine: the boundary whose caret is nearest x, the earlier
// one when two are equally near.
func (engine *lineEngine) OffsetAt(value string, size, weight, x float32) int {
	return engine.OffsetAtStyle(value, size, weight, false, x)
}

func (engine *lineEngine) OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int {
	if value == "" {
		return 0
	}
	rects := engine.geometryStyle(value, size, weight, italic, engine.lineStyle(value, size, weight, italic))
	best, bestDistance := 0, float32(-1)
	for cluster := 0; cluster <= len(rects.left); cluster++ {
		distance := rects.caret(cluster) - x
		if distance < 0 {
			distance = -distance
		}
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = cluster, distance
		}
	}
	return int(rects.starts[best])
}

var _ ui.TextEngine = (*lineEngine)(nil)
var _ ui.StyledTextEngine = (*lineEngine)(nil)
