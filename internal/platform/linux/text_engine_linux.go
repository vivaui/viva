package linux

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"

	"github.com/vivaui/viva/internal/grapheme"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/textcache"
	"github.com/vivaui/viva/ui"
)

// The Linux text engine: every proportional line is shaped once by Pango into a
// retained layout, and the answers layout needs (the width, the caret at each character
// boundary, the extent of each character) are read from that layout once and kept in Go. A
// width, a caret, a click or a selection of a line seen before is then a table lookup, which
// calls into no library and allocates nothing; the same layout is the one the label is drawn
// from, so what is measured is what is on screen.

// engineCacheLimit is the number of shaped lines a generation of the cache holds.
const engineCacheLimit = 512

// shapedLine is one line as Pango shaped it, at one font size. The size is a logical one
// when layout asked and a device one when a label is drawn; the metrics are not hinted, so
// both are the same line scaled (font_settings_linux.go).
type shapedLine struct {
	layout uintptr
	// width and ascent are in pixels of the size the line was shaped at, as are the tables.
	width, ascent float32

	// The tables are read from the layout when a query first needs them: a line only
	// drawn has none.
	hasTables bool
	// offsets are the byte offsets of the grapheme cluster boundaries, from 0 to len(value);
	// carets the x of the caret at each (the strong cursor: where a character of the
	// paragraph's direction is typed). extents holds, for the cluster that starts at
	// offsets[i], its left and right edge on the screen at extents[2*i] and extents[2*i+1].
	offsets []int32
	carets  []float32
	extents []float32
}

// textEngine is the ui.TextEngine over Pango. It is safe for concurrent use: the Pango
// context it shapes with is not, so every use of it, including drawing a line, is
// under mu.
type textEngine struct {
	mu          sync.Mutex
	services    *textServices
	cache       *textcache.Cache[*shapedLine]
	italicCache *textcache.Cache[*shapedLine]
	reference   ui.TextEngine
	// family is the family the interface's text is shaped in, "" for the sans
	// alias, which is the desktop's own interface font. Guarded by mu; a change
	// forgets the cache.
	family string
	// generation moves whenever a line shapes differently from before (ui.FontFamilyAware).
	generation atomic.Uint32

	strong, weak, cell pangoRectangle
}

func newTextEngine() (*textEngine, error) {
	services, err := newTextServices()
	if err != nil {
		return nil, err
	}
	engine := &textEngine{services: services, reference: ui.ReferenceTextEngine()}
	engine.cache = textcache.New(engineCacheLimit, engine.release)
	return engine, nil
}

func (engine *textEngine) release(line *shapedLine) {
	if line.layout != 0 {
		engine.services.objectUnref(line.layout)
		line.layout = 0
	}
}

func (engine *textEngine) close() {
	engine.mu.Lock()
	defer engine.mu.Unlock()
	engine.cache.Clear()
	if engine.italicCache != nil {
		engine.italicCache.Clear()
	}
	engine.services.close()
}

var systemEngine struct {
	once   sync.Once
	engine *textEngine
}

var _ ui.FontFamilyAware = (*textEngine)(nil)
var _ ui.TextRasterizer = (*textEngine)(nil)
var _ ui.MonoCellEngine = (*textEngine)(nil)

// RasterizeText implements ui.TextRasterizer using the same Pango shaped line
// and Cairo text path as the native label renderer.
func (engine *textEngine) RasterizeText(value string, style ui.TextStyle, scale float32, width, height int) (ui.Bitmap, error) {
	pixels, err := engine.services.draw(platform.TextLabel{
		Value: value, FontSize: style.Size, Weight: style.Weight, Italic: style.Italic,
		Color: platform.Color{
			Red: float64(style.Color.Red), Green: float64(style.Color.Green),
			Blue: float64(style.Color.Blue), Alpha: float64(style.Color.Alpha),
		},
	}, scale, width, height, false)
	if err != nil {
		return ui.Bitmap{}, err
	}
	return ui.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4}, nil
}

// SetFontFamily implements ui.FontFamilyAware: the family the interface's text is
// shaped in, "" for the desktop's own interface font (Pango's sans alias).
// Everything shaped with another family is forgotten.
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
	metrics, err := engine.services.configureLayout("M", monoFamily(face), size, 0, false)
	if err != nil {
		return 0, 0
	}
	return metrics.Width, metrics.Height
}

// FontGeneration implements ui.FontFamilyAware.
func (engine *textEngine) FontGeneration() uint32 { return engine.generation.Load() }

// sharedTextEngine is the process's engine, created when text is first measured or drawn;
// nil when Pango cannot be loaded.
func sharedTextEngine() *textEngine {
	systemEngine.once.Do(func() {
		if engine, err := newTextEngine(); err == nil {
			systemEngine.engine = engine
		}
	})
	return systemEngine.engine
}

// TextEngine is the Linux ui.TextEngine, or nil when the system's Pango is not available.
func TextEngine() ui.TextEngine {
	if engine := sharedTextEngine(); engine != nil {
		return engine
	}
	return nil
}

// lineLocked is the shaped line of value at a font size, shaped now when it has not been. mu
// is held.
func (engine *textEngine) lineLocked(value string, size, weight float32) (*shapedLine, error) {
	return engine.lineStyleLocked(value, size, weight, false)
}

func (engine *textEngine) lineStyleLocked(value string, size, weight float32, italic bool) (*shapedLine, error) {
	cache := engine.cache
	if italic {
		if engine.italicCache == nil {
			engine.italicCache = textcache.New(engineCacheLimit, engine.release)
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
	services := engine.services
	if !validFontSize(size) {
		return nil, fmt.Errorf("linux: invalid font size")
	}
	if !utf8.ValidString(value) {
		return nil, fmt.Errorf("linux: text is not valid UTF-8")
	}
	family := engine.family
	if family == "" {
		family = proportionalFamily
	}
	description, err := services.description(family, size, weight, italic)
	if err != nil {
		return nil, err
	}
	defer services.fontFree(description)
	layout := services.layoutNew(services.context)
	if layout == 0 {
		return nil, fmt.Errorf("linux: create Pango text layout")
	}
	valueBytes := cString(value)
	services.layoutSetSingleParagraph(layout, 1)
	services.layoutSetWidth(layout, -1)
	services.layoutSetFont(layout, description)
	services.layoutSetText(layout, &valueBytes[0], int32(len(value)))
	runtime.KeepAlive(valueBytes)
	engine.cell = pangoRectangle{}
	services.layoutGetExtents(layout, nil, &engine.cell)
	return &shapedLine{
		layout: layout,
		width:  float32(engine.cell.Width) / pangoScale,
		ascent: float32(services.layoutGetBaseline(layout)) / pangoScale,
	}, nil
}

// tablesLocked reads the carets and the extents of the characters of value from its layout.
func (engine *textEngine) tablesLocked(line *shapedLine, value string) {
	if line.hasTables {
		return
	}
	services := engine.services
	line.offsets = append(line.offsets[:0], 0)
	for at := 0; at < len(value); {
		at = grapheme.Next(value, at)
		line.offsets = append(line.offsets, int32(at))
	}
	clusters := len(line.offsets) - 1
	line.carets = line.carets[:0]
	line.extents = line.extents[:0]
	for index, offset := range line.offsets {
		engine.strong = pangoRectangle{}
		engine.weak = pangoRectangle{}
		services.layoutGetCursorPos(line.layout, offset, &engine.strong, &engine.weak)
		line.carets = append(line.carets, float32(engine.strong.X)/pangoScale)
		if index == clusters {
			break
		}
		engine.cell = pangoRectangle{}
		services.layoutIndexToPos(line.layout, offset, &engine.cell)
		left, right := engine.cell.X, engine.cell.X+engine.cell.Width
		if right < left {
			left, right = right, left
		}
		line.extents = append(line.extents, float32(left)/pangoScale, float32(right)/pangoScale)
	}
	line.hasTables = true
}

// shaped is the line of value at the logical size, with its tables, or nil when Pango
// cannot shape it (the caller then answers with the reference metrics). mu is held.
func (engine *textEngine) shaped(value string, size, weight float32) *shapedLine {
	return engine.shapedStyle(value, size, weight, false)
}

func (engine *textEngine) shapedStyle(value string, size, weight float32, italic bool) *shapedLine {
	line, err := engine.lineStyleLocked(value, size, weight, italic)
	if err != nil {
		return nil
	}
	engine.tablesLocked(line, value)
	return line
}

// boundary is the index in line.offsets of the cluster boundary at or before offset.
func (line *shapedLine) boundary(offset int) int {
	low, high := 0, len(line.offsets)-1
	for low < high {
		middle := (low + high + 1) / 2
		if int(line.offsets[middle]) <= offset {
			low = middle
		} else {
			high = middle - 1
		}
	}
	return low
}

// Width implements ui.TextEngine.
func (engine *textEngine) Width(value string, size, weight float32) float32 {
	return engine.WidthStyle(value, size, weight, false)
}

func (engine *textEngine) WidthStyle(value string, size, weight float32, italic bool) float32 {
	if value == "" {
		return 0
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line, err := engine.lineStyleLocked(value, size, weight, italic)
	if err != nil {
		return engine.reference.Width(value, size, weight)
	}
	return line.width
}

// Caret implements ui.TextEngine.
func (engine *textEngine) Caret(value string, size, weight float32, offset int) float32 {
	return engine.CaretStyle(value, size, weight, false, offset)
}

func (engine *textEngine) CaretStyle(value string, size, weight float32, italic bool, offset int) float32 {
	if value == "" {
		return 0
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line := engine.shapedStyle(value, size, weight, italic)
	if line == nil {
		return engine.reference.Caret(value, size, weight, offset)
	}
	return line.carets[line.boundary(max(0, min(offset, len(value))))]
}

// Selection implements ui.TextEngine: the extents of the characters of [start, end), joined
// where they touch, left to right.
func (engine *textEngine) Selection(dst []ui.TextSpan, value string, size, weight float32, start, end int) []ui.TextSpan {
	return engine.SelectionStyle(dst, value, size, weight, false, start, end)
}

func (engine *textEngine) SelectionStyle(dst []ui.TextSpan, value string, size, weight float32, italic bool, start, end int) []ui.TextSpan {
	if value == "" {
		return append(dst, ui.TextSpan{})
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line := engine.shapedStyle(value, size, weight, italic)
	if line == nil {
		return engine.reference.Selection(dst, value, size, weight, start, end)
	}
	first := line.boundary(max(0, min(start, len(value))))
	last := line.boundary(max(0, min(end, len(value))))
	if first > last {
		first, last = last, first
	}
	if first == last {
		return append(dst, ui.TextSpan{X: line.carets[first]})
	}
	base := len(dst)
	for cluster := first; cluster < last; cluster++ {
		left, right := line.extents[2*cluster], line.extents[2*cluster+1]
		joined := false
		for index := base; index < len(dst); index++ {
			span := &dst[index]
			if left <= span.X+span.Width+0.01 && right >= span.X-0.01 {
				edge := max(span.X+span.Width, right)
				span.X = min(span.X, left)
				span.Width = edge - span.X
				joined = true
				break
			}
		}
		if !joined {
			dst = append(dst, ui.TextSpan{X: left, Width: right - left})
		}
	}
	// A cluster can bridge two spans that did not touch before it: join them, then order them.
	for changed := true; changed; {
		changed = false
		for index := base; index < len(dst) && !changed; index++ {
			for other := index + 1; other < len(dst); other++ {
				a, b := dst[index], dst[other]
				if b.X <= a.X+a.Width+0.01 && b.X+b.Width >= a.X-0.01 {
					dst[index].X = min(a.X, b.X)
					dst[index].Width = max(a.X+a.Width, b.X+b.Width) - dst[index].X
					dst = append(dst[:other], dst[other+1:]...)
					changed = true
					break
				}
			}
		}
	}
	for index := base + 1; index < len(dst); index++ {
		for other := index; other > base && dst[other].X < dst[other-1].X; other-- {
			dst[other], dst[other-1] = dst[other-1], dst[other]
		}
	}
	return dst
}

// OffsetAt implements ui.TextEngine: the cluster boundary whose caret is nearest x.
func (engine *textEngine) OffsetAt(value string, size, weight, x float32) int {
	return engine.OffsetAtStyle(value, size, weight, false, x)
}

func (engine *textEngine) OffsetAtStyle(value string, size, weight float32, italic bool, x float32) int {
	if value == "" {
		return 0
	}
	engine.mu.Lock()
	defer engine.mu.Unlock()
	line := engine.shapedStyle(value, size, weight, italic)
	if line == nil {
		return engine.reference.OffsetAt(value, size, weight, x)
	}
	best, bestDistance := 0, float32(-1)
	for index, caret := range line.carets {
		distance := caret - x
		if distance < 0 {
			distance = -distance
		}
		if bestDistance < 0 || distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return int(line.offsets[best])
}
