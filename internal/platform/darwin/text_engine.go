//go:build darwin

package darwin

/*
#include <stdlib.h>
#include "native.h"
*/
import "C"

import (
	"strings"
	"sync"
	"sync/atomic"
	"unicode/utf8"
	"unsafe"

	"github.com/vivaui/viva/internal/textcache"
	"github.com/vivaui/viva/ui"
)

// lineCacheLimit is how many shaped lines each generation of the cache keeps (see
// textcache): a screenful of labels and fields is a few hundred.
const lineCacheLimit = 1024

// selectionPieces is the most extents C writes for one selection.
const selectionPieces = 32

// TextEngine is ui.TextEngine over CoreText: a line is shaped once, in the
// system font with the fallback CoreText picks per character, and the width layout
// uses, the glyphs a label draws, the caret, the selection and a click all come from
// that line. Use SharedTextEngine: the labels and the native editor draw from the same
// cache.
type TextEngine struct {
	mutex sync.Mutex
	cache *textcache.Cache[*C.viva_text_line]
	// family is the family the interface's text is shaped in, "" for the system
	// font. The name is also told to C, which keeps it for viva_system_font.
	family string
	// generation moves whenever a line shapes differently from before (ui.FontFamilyAware).
	generation atomic.Uint32
	// spans is where C writes a selection: an array passed to C would move to the heap
	// on every call.
	spans [2 * selectionPieces]C.double
}

var (
	sharedEngine     *TextEngine
	sharedEngineOnce sync.Once
)

func sharedTextEngine() *TextEngine {
	sharedEngineOnce.Do(func() {
		sharedEngine = &TextEngine{cache: textcache.New(lineCacheLimit, func(line *C.viva_text_line) { C.viva_text_line_release(line) })}
	})
	return sharedEngine
}

// SharedTextEngine is the engine the darwin backend draws with.
func SharedTextEngine() *TextEngine { return sharedTextEngine() }

var _ ui.FontFamilyAware = (*TextEngine)(nil)

// SetFontFamily implements ui.FontFamilyAware: the family the interface's text is
// shaped in, "" for the system font. The name is told to C, which holds it for
// every font it creates, and everything shaped with another family is forgotten.
func (engine *TextEngine) SetFontFamily(family string) {
	family = strings.TrimSpace(family)
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	if family == engine.family {
		return
	}
	engine.family = family
	name := C.CString(family)
	defer C.free(unsafe.Pointer(name))
	C.viva_text_set_family(name)
	engine.cache.Clear()
	engine.generation.Add(1)
}

// FontGeneration implements ui.FontFamilyAware.
func (engine *TextEngine) FontGeneration() uint32 { return engine.generation.Load() }

// Reset forgets every shaped line, for a change of the system font or of the display
// that would shape them differently.
func (engine *TextEngine) Reset() {
	engine.mutex.Lock()
	engine.cache.Clear()
	engine.generation.Add(1)
	engine.mutex.Unlock()
}

// line is the shaped line for value; the cache owns it and the lock must be held.
func (engine *TextEngine) line(value string, size, weight float32) *C.viva_text_line {
	key := textcache.Key{Value: value, Size: size, Weight: weight}
	if line, ok := engine.cache.Get(key); ok {
		return line
	}
	var data *C.uchar
	if len(value) > 0 {
		data = (*C.uchar)(unsafe.Pointer(unsafe.StringData(value)))
	}
	line := C.viva_text_line_create(data, C.size_t(len(value)), C.double(size), C.double(weight))
	if line == nil {
		return nil
	}
	engine.cache.Put(key, line)
	return line
}

// retainedLine is line for a caller outside the lock, with a reference of its own.
func (engine *TextEngine) retainedLine(value string, size, weight float32) *C.viva_text_line {
	engine.mutex.Lock()
	line := engine.line(value, size, weight)
	C.viva_text_line_retain(line)
	engine.mutex.Unlock()
	return line
}

// Width implements ui.TextEngine.
func (engine *TextEngine) Width(value string, size, weight float32) float32 {
	if value == "" || size <= 0 {
		return 0
	}
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	line := engine.line(value, size, weight)
	if line == nil {
		return 0
	}
	return float32(line.width)
}

// Caret implements ui.TextEngine.
func (engine *TextEngine) Caret(value string, size, weight float32, offset int) float32 {
	if value == "" || size <= 0 {
		return 0
	}
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	line := engine.line(value, size, weight)
	if line == nil {
		return 0
	}
	return float32(C.viva_text_line_caret(line, C.size_t(utf16Offset(value, offset))))
}

// Selection implements ui.TextEngine.
func (engine *TextEngine) Selection(dst []ui.TextSpan, value string, size, weight float32, start, end int) []ui.TextSpan {
	if value == "" || size <= 0 {
		return append(dst, ui.TextSpan{})
	}
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	line := engine.line(value, size, weight)
	if line == nil {
		return append(dst, ui.TextSpan{})
	}
	count := int(C.viva_text_line_selection(line, C.size_t(utf16Offset(value, start)), C.size_t(utf16Offset(value, end)), &engine.spans[0], selectionPieces))
	for piece := 0; piece < count; piece++ {
		dst = append(dst, ui.TextSpan{X: float32(engine.spans[2*piece]), Width: float32(engine.spans[2*piece+1])})
	}
	return dst
}

// OffsetAt implements ui.TextEngine.
func (engine *TextEngine) OffsetAt(value string, size, weight, x float32) int {
	if value == "" || size <= 0 {
		return 0
	}
	engine.mutex.Lock()
	defer engine.mutex.Unlock()
	line := engine.line(value, size, weight)
	if line == nil {
		return 0
	}
	return byteOffset(value, int(C.viva_text_line_index_at(line, C.double(x))))
}

// utf16Offset is the number of UTF-16 units CoreText counts before byte offset of value;
// the offset is clamped to the string and moved back to the start of the character it
// falls inside.
func utf16Offset(value string, offset int) int {
	offset = max(0, min(offset, len(value)))
	for offset > 0 && offset < len(value) && !utf8.RuneStart(value[offset]) {
		offset--
	}
	units := 0
	for _, r := range value[:offset] {
		units += utf16Units(r)
	}
	return units
}

// byteOffset is the byte offset of value at UTF-16 unit index, or len(value) past its end.
func byteOffset(value string, index int) int {
	units := 0
	for offset, r := range value {
		if units >= index {
			return offset
		}
		units += utf16Units(r)
	}
	return len(value)
}

func utf16Units(r rune) int {
	if r >= 0x10000 && r != utf8.RuneError {
		return 2
	}
	return 1
}
