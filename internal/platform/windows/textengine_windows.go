package windows

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textcache"
	"github.com/vivaui/viva/internal/textmetrics"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
)

// The text engine: a line is shaped once by DirectWrite into a retained
// IDWriteTextLayout, and its width, the caret, the selection, the character under a
// click and the glyphs Direct2D draws all come from that layout, with the system's
// font fallback, so they agree.
//
// Layouts live in a cache keyed by the line, the font size and the weight. Viva lays
// text out at its logical size and draws it at the size times the display scale, so
// one line is two entries when the scale is not 1. The tables a caret or a click is
// answered from are computed once, the first time a layout is asked for them: a hit
// is a lookup and allocates nothing.

// Method slots of IDWriteFactory2 and IDWriteTextFormat1, and of IDWriteTextLayout
// (checked against dwrite.h and dwrite_2.h).
const (
	writeFactoryGetSystemFontFallback = 26
	writeFactoryCreateFallbackBuilder = 27
	writeBuilderAddMapping            = 3
	writeBuilderAddMappings           = 4
	writeBuilderCreateFontFallback    = 5
	writeFormatSetFontFallback        = 34
	writeHitTestTextPosition          = 65
	writeHitTestTextRange             = 66
)

var writeFactory2IID = guid{0x0439fc60, 0xca44, 0x4994, [8]byte{0x8d, 0xee, 0x3a, 0x9a, 0xf7, 0xb7, 0x32, 0xec}}
var writeTextFormat1IID = guid{0x5f174b49, 0x0d8b, 0x4cfb, [8]byte{0x8b, 0xca, 0xf1, 0xcc, 0xe9, 0xd0, 0x6c, 0x67}}

// textEngineCapacity is how many shaped lines each generation of the cache holds.
const textEngineCapacity = 512

// hitMetrics is DWRITE_HIT_TEST_METRICS.
type hitMetrics struct {
	Position, Length uint32
	Left, Top        float32
	Width, Height    float32
	BidiLevel        uint32
	IsText, Trimmed  int32
}

// shapedLine is a line DirectWrite has laid out.
type shapedLine struct {
	layout uintptr
	// left is where the glyphs start in the layout; the tables and the drawing take it off.
	left float32
	// width is the advance of the whole line, with its trailing spaces.
	width float32
	// height and ascent are those of the first (only) line of the layout.
	height, ascent float32
	length         int
	tables         *lineTables
}

// lineTables answer the caret, the selection and the click of a line without asking
// DirectWrite again. A cluster is what DirectWrite shapes as one unit (a letter and its
// marks, a ligature, an emoji sequence); an offset inside one means its start.
type lineTables struct {
	// cluster is the byte offset of the cluster each byte offset belongs to.
	cluster []int32
	// next, caret, boxLeft and boxWidth are indexed by the byte offset of a cluster's
	// start: where the next cluster starts, the x of the caret before it, and the
	// extent of its glyphs.
	next              []int32
	caret             []float32
	boxLeft, boxWidth []float32
	// bidi says the line has right-to-left text: its carets are not in logical order.
	bidi bool
}

func (line *shapedLine) close() { release(&line.layout) }

// textEngine is the ui.TextEngine of Windows. Its methods may be called from any
// goroutine, one at a time.
type textEngine struct {
	mu          sync.Mutex
	services    textServices
	fallback    uintptr
	cache       *textcache.Cache[*shapedLine]
	italicCache *textcache.Cache[*shapedLine]
	hit         hitMetrics
	ranges      [4]hitMetrics
	// family is the family the interface's text is shaped in, "" for Segoe UI,
	// the interface font Windows ships. Guarded by mu; a change forgets the cache.
	family string
	// generation moves whenever a line shapes differently from before (ui.FontFamilyAware).
	generation atomic.Uint32

	rasterMu sync.Mutex
	raster   *directRasterizer
}

var (
	sharedEngine     textEngine
	sharedEngineOnce sync.Once
	sharedEngineErr  error
)

// theTextEngine is the process's text engine, created on first use.
func theTextEngine() (*textEngine, error) {
	sharedEngineOnce.Do(func() { sharedEngineErr = sharedEngine.initialize() })
	if sharedEngineErr != nil {
		return nil, sharedEngineErr
	}
	return &sharedEngine, nil
}

var _ ui.FontFamilyAware = (*textEngine)(nil)
var _ ui.TextRasterizer = (*textEngine)(nil)
var _ ui.MonoCellEngine = (*textEngine)(nil)

// RasterizeText implements ui.TextRasterizer with the same DirectWrite/Direct2D
// path the native label renderer uses.
func (engine *textEngine) RasterizeText(value string, style ui.TextStyle, scale float32, width, height int) (ui.Bitmap, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 ||
		scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return ui.Bitmap{}, fmt.Errorf("windows: invalid text bitmap size or scale")
	}

	engine.rasterMu.Lock()
	defer engine.rasterMu.Unlock()

	if engine.raster == nil {
		rasterizer, err := newDirectRasterizer()
		if err != nil {
			return ui.Bitmap{}, err
		}
		engine.raster = rasterizer
	}
	pixels, _, err := engine.raster.rasterize(platform.TextLabel{
		Value: value, FontSize: style.Size, Weight: style.Weight, Italic: style.Italic,
		Color: platform.Color{
			Red: float64(style.Color.Red), Green: float64(style.Color.Green),
			Blue: float64(style.Color.Blue), Alpha: float64(style.Color.Alpha),
		},
	}, scale, width, height, false, 0)
	if err != nil {
		return ui.Bitmap{}, err
	}
	copyPixels := append([]byte(nil), pixels...)
	return ui.Bitmap{Pix: copyPixels, Width: width, Height: height, Stride: width * 4}, nil
}

// SetFontFamily implements ui.FontFamilyAware: the family the interface's text is
// shaped in, "" for Segoe UI. Everything shaped with another family is forgotten.
func (engine *textEngine) SetFontFamily(family string) {
	family = strings.TrimSpace(family)
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if family == engine.family {
		return
	}
	engine.family = family
	engine.cache.Clear()
	if engine.italicCache != nil {
		engine.italicCache.Clear()
	}
	engine.generation.Add(1)
}

// MonoCell implements ui.MonoCellEngine: the advance and line height of the face's own
// font, which is the cell a terminal is laid out on.
func (engine *textEngine) MonoCell(size float32, face ui.MonoFace) (advance, line float32) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	metrics, err := engine.services.measureIn(monoFamily(face), "M", size, 0)
	if err != nil {
		return 0, 0
	}
	return metrics.Width, metrics.Height
}

// FontGeneration implements ui.FontFamilyAware.
func (engine *textEngine) FontGeneration() uint32 { return engine.generation.Load() }

// TextEngine is the platform's text engine, or nil when DirectWrite is not available.
func TextEngine() ui.TextEngine {
	engine, err := theTextEngine()
	if err != nil {
		return nil
	}
	return engine
}

func (engine *textEngine) initialize() error {
	if err := engine.services.initializeWrite(); err != nil {
		return err
	}
	engine.cache = textcache.New(textEngineCapacity, (*shapedLine).close)
	// The system's font fallback, named: a line is measured with the fonts that draw it.
	// IDWriteFactory2 is Windows 8.1; before it, the layouts' default fallback applies.
	var factory2 uintptr
	if comCall(engine.services.write, 0, uintptr(unsafe.Pointer(&writeFactory2IID)), uintptr(unsafe.Pointer(&factory2))) == 0 && factory2 != 0 {
		if hresult(comCall(factory2, writeFactoryGetSystemFontFallback, uintptr(unsafe.Pointer(&engine.fallback)))) != nil {
			engine.fallback = 0
		}
		release(&factory2)
	}
	return nil
}

// setFontFallback makes format take the characters its font lacks from fallback, an
// IDWriteFontFallback. It does nothing without one, or before Windows 8.1, which
// has no IDWriteTextFormat1.
func setFontFallback(format, fallback uintptr) {
	if fallback == 0 {
		return
	}
	var format1 uintptr
	if comCall(format, 0, uintptr(unsafe.Pointer(&writeTextFormat1IID)), uintptr(unsafe.Pointer(&format1))) == 0 && format1 != 0 {
		comCall(format1, writeFormatSetFontFallback, fallback)
		release(&format1)
	}
}

// forget drops every shaped line: the installed fonts or the text settings changed.
func (engine *textEngine) forget() {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.cache.Clear()
	if engine.italicCache != nil {
		engine.italicCache.Clear()
	}
	engine.generation.Add(1)
}

// line is the shaped line of value, creating and caching it. The caller holds mu.
func (engine *textEngine) line(value string, size, weight float32) (*shapedLine, error) {
	return engine.lineStyle(value, size, weight, false)
}

func (engine *textEngine) lineStyle(value string, size, weight float32, italic bool) (*shapedLine, error) {
	if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
		return nil, fmt.Errorf("windows: invalid font size")
	}
	cache := engine.cache
	if italic {
		if engine.italicCache == nil {
			engine.italicCache = textcache.New(textEngineCapacity, (*shapedLine).close)
		}
		cache = engine.italicCache
	}
	key := textcache.Key{Value: value, Size: size, Weight: weight}
	if line, ok := cache.Get(key); ok {
		return line, nil
	}
	line, err := engine.shapeStyle(value, size, weight, italic)
	if err != nil {
		return nil, err
	}
	cache.Put(key, line)
	return line, nil
}

func (engine *textEngine) shape(value string, size, weight float32) (*shapedLine, error) {
	return engine.shapeStyle(value, size, weight, false)
}

func (engine *textEngine) shapeStyle(value string, size, weight float32, italic bool) (*shapedLine, error) {
	units := utf16.Encode([]rune(value))
	if len(units) == 0 {
		return nil, fmt.Errorf("windows: empty line")
	}
	family := engine.family
	if family == "" {
		family = "Segoe UI"
	}
	format, err := engine.services.formatStyle(family, size, weight, italic)
	if err != nil {
		return nil, err
	}
	defer release(&format)
	setFontFallback(format, engine.fallback)
	var layout uintptr
	if err := hresult(engine.services.createLayout(engine.services.write, &units[0], uint32(len(units)), format, 1<<20, 1<<20, &layout)); err != nil {
		return nil, fmt.Errorf("windows: lay out text: %w", err)
	}
	if textmetrics.RightToLeft(value) {
		// The paragraph reads right to left, so that the punctuation around a right-to-left
		// line falls on the side it belongs; trailing alignment keeps its glyphs
		// at the left of the layout, where the label's box starts.
		comCall(layout, writeSetTextAlignment, 1)
		comCall(layout, writeSetReadingDirection, 1)
	}
	line := &shapedLine{layout: layout, length: len(value)}
	var metrics textMetrics
	if err := hresult(comCall(layout, writeGetMetrics, uintptr(unsafe.Pointer(&metrics)))); err != nil {
		line.close()
		return nil, err
	}
	if metrics.Lines > 1024 {
		line.close()
		return nil, fmt.Errorf("windows: text measurement exceeds line limit")
	}
	rows := make([]lineMetrics, max(1, int(metrics.Lines)))
	var count uint32
	if err := hresult(comCall(layout, writeGetLineMetrics, uintptr(unsafe.Pointer(&rows[0])), uintptr(len(rows)), uintptr(unsafe.Pointer(&count)))); err != nil {
		line.close()
		return nil, err
	}
	first := rows[0]
	line.left, line.width = metrics.Left, metrics.WidthTrailing
	line.height, line.ascent = first.Height, first.Baseline
	return line, nil
}

// ensureTables computes the tables of line, which was shaped from value. The caller
// holds mu.
func (engine *textEngine) ensureTables(line *shapedLine, value string) error {
	if line.tables != nil {
		return nil
	}
	n := len(value)
	tables := &lineTables{
		cluster:  make([]int32, n+1),
		next:     make([]int32, n+1),
		caret:    make([]float32, n+1),
		boxLeft:  make([]float32, n+1),
		boxWidth: make([]float32, n+1),
		bidi:     bidi.NeedsBidi(value),
	}
	// The byte offset of every UTF-16 unit, and back: DirectWrite counts in UTF-16.
	units := 0
	for _, r := range value {
		units += utf16.RuneLen(r)
	}
	byteAt := make([]int32, units+1)
	unitAt := make([]int32, n+1)
	unit := 0
	for at, r := range value {
		unitAt[at] = int32(unit)
		byteAt[unit] = int32(at)
		width := utf16.RuneLen(r)
		if width < 1 {
			width = 1
		}
		unit += width
	}
	unitAt[n], byteAt[unit] = int32(unit), int32(n)

	for start := 0; start < n; {
		position := uint32(unitAt[start])
		var x, y float32
		engine.hit = hitMetrics{}
		if err := hresult(comCall(line.layout, writeHitTestTextPosition, uintptr(position), 0, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&engine.hit)))); err != nil {
			return fmt.Errorf("windows: hit test: %w", err)
		}
		// The cluster DirectWrite reports, as bytes; a report that does not start here is
		// not believed, and the character is a cluster of its own.
		end := nextRuneStart(value, start)
		if engine.hit.Position == position && engine.hit.Length > 0 {
			if last := int(engine.hit.Position + engine.hit.Length); last <= units && byteAt[last] > int32(start) {
				end = int(byteAt[last])
			}
		}
		for at := start; at < end; at++ {
			tables.cluster[at] = int32(start)
		}
		tables.next[start] = int32(end)
		tables.caret[start] = x - line.left
		start = end
	}
	tables.cluster[n] = int32(n)
	if n > 0 {
		// The end of the line is the trailing edge of its last cluster.
		last := int(tables.cluster[n-1])
		var x, y float32
		if err := hresult(comCall(line.layout, writeHitTestTextPosition, uintptr(unitAt[last]), 1, uintptr(unsafe.Pointer(&x)), uintptr(unsafe.Pointer(&y)), uintptr(unsafe.Pointer(&engine.hit)))); err != nil {
			return fmt.Errorf("windows: hit test: %w", err)
		}
		tables.caret[n] = x - line.left
	}
	for start := 0; start < n; start = int(tables.next[start]) {
		next := int(tables.next[start])
		if !tables.bidi {
			tables.boxLeft[start] = tables.caret[start]
			tables.boxWidth[start] = tables.caret[next] - tables.caret[start]
			continue
		}
		// A right-to-left cluster's box is not between its carets in logical order.
		var count uint32
		if err := hresult(comCall(line.layout, writeHitTestTextRange, uintptr(unitAt[start]), uintptr(unitAt[next]-unitAt[start]), 0, 0,
			uintptr(unsafe.Pointer(&engine.ranges[0])), uintptr(len(engine.ranges)), uintptr(unsafe.Pointer(&count)))); err != nil && count == 0 {
			return fmt.Errorf("windows: hit test range: %w", err)
		}
		left, right := float32(math.MaxFloat32), float32(-math.MaxFloat32)
		for index := 0; index < int(min(count, uint32(len(engine.ranges)))); index++ {
			box := engine.ranges[index]
			left, right = min(left, box.Left), max(right, box.Left+box.Width)
		}
		if left > right {
			left, right = tables.caret[start]+line.left, tables.caret[start]+line.left
		}
		tables.boxLeft[start], tables.boxWidth[start] = left-line.left, right-left
	}
	line.tables = tables
	return nil
}

func nextRuneStart(value string, from int) int {
	_, size := utf8.DecodeRuneInString(value[from:])
	return from + max(size, 1)
}

// tablesOf is the tables of the line of value, shaping and filling them when they
// are missing. ok is false when DirectWrite failed; the caller then answers as for no
// text. The caller holds mu.
func (engine *textEngine) tablesOf(value string, size, weight float32) (*shapedLine, bool) {
	return engine.tablesOfStyle(value, size, weight, false)
}

func (engine *textEngine) tablesOfStyle(value string, size, weight float32, italic bool) (*shapedLine, bool) {
	if value == "" {
		return nil, false
	}
	line, err := engine.lineStyle(value, size, weight, italic)
	if err != nil || engine.ensureTables(line, value) != nil {
		return nil, false
	}
	return line, true
}

// Width is the advance of the whole line.
func (engine *textEngine) Width(value string, size, weight float32) float32 {
	return engine.WidthStyle(value, size, weight, false)
}

func (engine *textEngine) WidthStyle(value string, size, weight float32, italic bool) float32 {
	if value == "" {
		return 0
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, err := engine.lineStyle(value, size, weight, italic)
	if err != nil {
		return 0
	}
	return line.width
}

// Caret is the x of the caret at byte offset.
func (engine *textEngine) Caret(value string, size, weight float32, offset int) float32 {
	return engine.CaretStyle(value, size, weight, false, offset)
}

func (engine *textEngine) CaretStyle(value string, size, weight float32, italic bool, offset int) float32 {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, ok := engine.tablesOfStyle(value, size, weight, italic)
	if !ok {
		return 0
	}
	offset = max(0, min(offset, len(value)))
	return line.tables.caret[line.tables.cluster[offset]]
}

// Selection appends the extents of [start, end), left to right.
func (engine *textEngine) Selection(dst []ui.TextSpan, value string, size, weight float32, start, end int) []ui.TextSpan {
	return engine.SelectionStyle(dst, value, size, weight, false, start, end)
}

func (engine *textEngine) SelectionStyle(dst []ui.TextSpan, value string, size, weight float32, italic bool, start, end int) []ui.TextSpan {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, ok := engine.tablesOfStyle(value, size, weight, italic)
	if !ok {
		return dst
	}
	tables := line.tables
	start, end = int(tables.cluster[max(0, min(start, len(value)))]), int(tables.cluster[max(0, min(end, len(value)))])
	if start > end {
		start, end = end, start
	}
	if start == end {
		return append(dst, ui.TextSpan{X: tables.caret[start]})
	}
	const touching = 0.01
	base := len(dst)
	for at := start; at < end; at = int(tables.next[at]) {
		left, width := tables.boxLeft[at], tables.boxWidth[at]
		dst = append(dst, ui.TextSpan{X: left, Width: width})
	}
	// Order the pieces left to right and join the ones that touch: the boxes of a
	// right-to-left stretch arrive from the right.
	spans := dst[base:]
	for index := 1; index < len(spans); index++ {
		for back := index; back > 0 && spans[back].X < spans[back-1].X; back-- {
			spans[back], spans[back-1] = spans[back-1], spans[back]
		}
	}
	joined := 0
	for index := 1; index < len(spans); index++ {
		last := &spans[joined]
		if spans[index].X <= last.X+last.Width+touching {
			last.Width = max(last.Width, spans[index].X+spans[index].Width-last.X)
			continue
		}
		joined++
		spans[joined] = spans[index]
	}
	return dst[:base+joined+1]
}

// OffsetAt is the byte offset of the cluster boundary nearest x.
func (engine *textEngine) OffsetAt(value string, size, weight, x float32) int {
	return engine.OffsetAtStyle(value, size, weight, false, x)
}

func (engine *textEngine) OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, ok := engine.tablesOfStyle(value, size, weight, italic)
	if !ok {
		return 0
	}
	tables := line.tables
	best, bestDistance := 0, float32(math.MaxFloat32)
	for at := 0; at < len(value); at = int(tables.next[at]) {
		if distance := float32(math.Abs(float64(x - tables.caret[at]))); distance < bestDistance {
			best, bestDistance = at, distance
		}
	}
	if distance := float32(math.Abs(float64(x - tables.caret[len(value)]))); distance < bestDistance {
		best = len(value)
	}
	return best
}

// metrics are the text.Metrics of a line: the measure behind MeasureText.
func (engine *textEngine) metrics(value string, size, weight float32) (text.Metrics, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	if value == "" {
		if size <= 0 || math.IsNaN(float64(size)) || math.IsInf(float64(size), 0) {
			return text.Metrics{}, fmt.Errorf("windows: invalid font size")
		}
		value = " "
		line, err := engine.line(value, size, weight)
		if err != nil {
			return text.Metrics{}, err
		}
		return text.Metrics{Height: line.height, Ascent: line.ascent, Descent: line.height - line.ascent}, nil
	}
	line, err := engine.line(value, size, weight)
	if err != nil {
		return text.Metrics{}, err
	}
	return text.Metrics{Width: line.width, Height: line.height, Ascent: line.ascent, Descent: line.height - line.ascent}, nil
}

// draw calls paint with the shaped line of value at size, a size in device pixels when
// Direct2D draws it. The line belongs to the cache and is valid until paint returns.
func (engine *textEngine) draw(value string, size, weight float32, paint func(*shapedLine) error) error {
	return engine.drawStyle(value, size, weight, false, paint)
}

func (engine *textEngine) drawStyle(value string, size, weight float32, italic bool, paint func(*shapedLine) error) error {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, err := engine.lineStyle(value, size, weight, italic)
	if err != nil {
		return err
	}
	return paint(line)
}
