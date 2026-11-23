//go:build android

package android

/*
#include <stdlib.h>
#include "jni_android.h"
*/
import "C"

import (
	"errors"
	"math"
	"sync"
	"unicode/utf16"
	"unsafe"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/internal/bidi"
	"github.com/vivaui/viva/internal/lineengine"
	"github.com/vivaui/viva/internal/platform"
	"github.com/vivaui/viva/internal/platform/vulkan"
	"github.com/vivaui/viva/internal/textmetrics"
)

// The Android text engine is the shared line engine over the system's own text
// drawing (dev.viva.host.Text): the width of a line and the extent of each of its
// grapheme clusters come from the font Android draws it with, shaped by Minikin, so
// what is measured is what is on screen. A line that has been measured is answered
// from the engine's tables and crosses into Java no more.

// measurer is the line engine's view of the system's text drawing. The Java side
// keeps one Paint and one bitmap, so every call is made under mu.
type measurer struct {
	mu         sync.Mutex
	units      []uint16
	pixels     unsafe.Pointer
	capacity   int
	invalidate func()
}

var _ lineengine.Measurer = (*measurer)(nil)
var _ lineengine.StyledMeasurer = (*measurer)(nil)
var _ lineengine.Rasterizer = (*measurer)(nil)

// shared is the one measurer of the process: the engine and the Vulkan renderer's
// label drawing read the same fonts.
var shared = &measurer{}

// encode puts value in UTF-16 in the measurer's scratch, which is valid until the
// next call. The caller holds mu.
func (m *measurer) encode(value string) (*C.uint16_t, C.int) {
	m.units = m.units[:0]
	for _, r := range value {
		if r >= 0x10000 {
			high, low := utf16.EncodeRune(r)
			m.units = append(m.units, uint16(high), uint16(low))
		} else {
			m.units = append(m.units, uint16(r))
		}
	}
	if len(m.units) == 0 {
		return nil, 0
	}
	return (*C.uint16_t)(unsafe.Pointer(&m.units[0])), C.int(len(m.units))
}

// weightClass is the CSS weight of a gfx weight: the steps the platforms' own
// engines take.
func weightClass(weight float32) int {
	switch {
	case weight >= 0.34:
		return 700
	case weight >= 0.20:
		return 600
	case weight >= 0.10:
		return 500
	}
	return 400
}

func cBool(value bool) C.int {
	if value {
		return 1
	}
	return 0
}

func (m *measurer) SetInvalidate(invalidate func()) { m.invalidate = invalidate }

func (m *measurer) Width(value string, size, weight float32, rtl bool) float32 {
	return m.WidthStyle(value, size, weight, false, rtl)
}

func (m *measurer) WidthStyle(value string, size, weight float32, italic, rtl bool) float32 {
	if value == "" {
		return 0
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	units, length := m.encode(value)
	width := float32(C.viva_text_width(units, length, C.float(size), C.int(weightClass(weight)), cBool(italic), 0))
	if width < 0 {
		return 0
	}
	return width
}

func (m *measurer) Rects(value string, size, weight float32, rtl bool, bounds []int32, lefts, rights []float32) {
	m.RectsStyle(value, size, weight, false, rtl, bounds, lefts, rights)
}

func (m *measurer) RectsStyle(value string, size, weight float32, italic, rtl bool, bounds []int32, lefts, rights []float32) {
	count := len(bounds) - 1
	if count <= 0 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	units, length := m.encode(value)
	if C.viva_text_rects(units, length, C.float(size), C.int(weightClass(weight)), cBool(italic), 0, cBool(rtl),
		(*C.int32_t)(unsafe.Pointer(&bounds[0])), C.int(len(bounds)), (*C.float)(unsafe.Pointer(&lefts[0])), (*C.float)(unsafe.Pointer(&rights[0]))) == 0 {
		// The host could not answer: the line is drawn with no extent rather than not at all.
		clear(lefts[:count])
		clear(rights[:count])
	}
}

// argb is a color as Android's Canvas takes it.
func argb(red, green, blue, alpha float64) C.int {
	component := func(v float64) uint32 { return uint32(math.Round(min(max(v, 0), 1) * 255)) }
	return C.int(int32(component(alpha)<<24 | component(red)<<16 | component(green)<<8 | component(blue)))
}

// bitmap is the pixels of the Java bitmap, as BGRA premultiplied: Android's own
// order is RGBA, and the renderers' textures are BGRA.
func (m *measurer) bitmap(width, height int) ([]byte, error) {
	size := width * height * 4
	if size > m.capacity {
		if m.pixels != nil {
			C.free(m.pixels)
		}
		m.pixels, m.capacity = C.malloc(C.size_t(size)), size
		if m.pixels == nil {
			m.capacity = 0
			return nil, errors.New("android: no memory for a text bitmap")
		}
	}
	if C.viva_text_end(m.pixels, C.int64_t(size)) == 0 {
		return nil, errors.New("android: the host could not copy the text bitmap")
	}
	rgba := unsafe.Slice((*byte)(m.pixels), size)
	pixels := make([]byte, size)
	for i := 0; i < size; i += 4 {
		pixels[i], pixels[i+1], pixels[i+2], pixels[i+3] = rgba[i+2], rgba[i+1], rgba[i], rgba[i+3]
	}
	return pixels, nil
}

// RasterizeText draws one proportional line, as a gfx.TextRasterizer.
func (m *measurer) RasterizeText(value string, style gfx.TextStyle, scale float32, width, height int) (gfx.Bitmap, error) {
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 || width*height > 16*1024*1024 || scale <= 0 || !platform.ValidFontSize(style.Size) {
		return gfx.Bitmap{}, errors.New("android: invalid text bitmap size or scale")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if C.viva_text_begin(C.int(width), C.int(height)) == 0 {
		return gfx.Bitmap{}, errors.New("android: the host could not start a text bitmap")
	}
	if value != "" {
		units, length := m.encode(value)
		baseline := float32(textmetrics.LabelBaseline(height, float64(style.Size*scale)))
		color := style.Color
		C.viva_text_draw(units, length, 0, C.float(baseline), C.float(style.Size*scale), C.int(weightClass(style.Weight)), cBool(style.Italic),
			cBool(style.Mono), argb(float64(color.Red), float64(color.Green), float64(color.Blue), float64(color.Alpha)), 0,
			cBool(bidi.BaseDirection(value) == bidi.RTL))
	}
	pixels, err := m.bitmap(width, height)
	if err != nil {
		return gfx.Bitmap{}, err
	}
	return gfx.Bitmap{Pix: pixels, Width: width, Height: height, Stride: width * 4}, nil
}

// Rasterize draws the glyphs of a label into the coverage of its texture, as the
// Vulkan renderer's text rasterizer: the device columns from skip to skip+width of the
// line. It is the grayscale text Android draws; LCD text is not offered.
func (m *measurer) Rasterize(label platform.TextLabel, scale float32, skip, width, height int, allowLCD bool) ([]byte, bool, error) {
	if !platform.LabelTextureFits(width, height) || skip < 0 {
		return nil, false, errors.New("android: label texture exceeds bounded size")
	}
	if !platform.ValidFontSize(label.FontSize) || scale <= 0 || math.IsNaN(float64(scale)) || math.IsInf(float64(scale), 0) {
		return nil, false, errors.New("android: invalid label font size or scale")
	}
	pixels := make([]byte, width*height*4)
	if label.Value == "" || (label.Color.Alpha <= 0 && len(label.Runs) == 0) {
		return pixels, false, nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if C.viva_text_begin(C.int(width), C.int(height)) == 0 {
		return nil, false, errors.New("android: the host could not start a text bitmap")
	}
	size := C.float(label.FontSize * scale)
	baseline := C.float(platform.TextBaseline(label, height, scale))
	rtl := cBool(bidi.BaseDirection(label.Value) == bidi.RTL)
	if len(label.Runs) == 0 {
		units, length := m.encode(label.Value)
		C.viva_text_draw(units, length, C.float(-skip), baseline, size, C.int(weightClass(label.Weight)), cBool(label.Italic), cBool(label.Mono),
			argb(label.Color.Red, label.Color.Green, label.Color.Blue, label.Color.Alpha), 0, rtl)
	} else {
		pitch := float64(label.CellWidth * scale)
		if pitch <= 0 || math.IsNaN(pitch) || math.IsInf(pitch, 0) {
			return nil, false, errors.New("android: invalid fixed-pitch cell width")
		}
		offset := 0
		for _, run := range label.Runs {
			end := min(offset+int(run.Bytes), len(label.Value))
			text := label.Value[offset:end]
			offset = end
			if text == "" {
				continue
			}
			weight := run.Weight
			if weight == 0 {
				weight = label.Weight
			}
			x := float64(run.Column)*pitch - float64(skip)
			units, length := m.encode(text)
			C.viva_text_draw(units, length, C.float(x), baseline, size, C.int(weightClass(weight)), cBool(run.Italic || label.Italic), 1,
				argb(float64(run.Color.Red), float64(run.Color.Green), float64(run.Color.Blue), float64(run.Color.Alpha)), cBool(run.Underline), 0)
		}
	}
	drawn, err := m.bitmap(width, height)
	if err != nil {
		return nil, false, err
	}
	return drawn, false, nil
}

func (m *measurer) Spans() []vulkan.ColorSpan { return nil }
func (m *measurer) Close()                    {}

// newRasterizer makes what the Vulkan renderer draws label glyphs with.
func newRasterizer() (vulkan.TextRasterizer, error) { return shared, nil }

// engine is the text engine the application measures with: the shared line engine
// over the system's text drawing, with the fixed-pitch cell of the system's own
// monospace font.
type engine struct {
	*lineengine.Engine
}

var _ gfx.TextEngine = engine{}
var _ gfx.StyledTextEngine = engine{}
var _ gfx.TextRasterizer = engine{}
var _ gfx.FontFamilyAware = engine{}
var _ gfx.MonoCellEngine = engine{}

var sharedEngine = struct {
	once   sync.Once
	engine engine
}{}

// TextEngine is the gfx.TextEngine over the system's text, shared by every window.
func TextEngine() gfx.TextEngine {
	sharedEngine.once.Do(func() {
		sharedEngine.engine = engine{lineengine.New(shared)}
	})
	return sharedEngine.engine
}

// SetFontFamily names the family the interface's proportional text is shaped in.
func (e engine) SetFontFamily(family string) {
	e.Engine.SetFontFamily(family)
	shared.mu.Lock()
	defer shared.mu.Unlock()
	if family == "" {
		C.viva_text_set_family(nil, 0)
		return
	}
	units, length := shared.encode(family)
	C.viva_text_set_family(units, length)
}

// MonoCell is the cell of the system's monospace font at a logical size.
func (e engine) MonoCell(size float32, face gfx.MonoFace) gfx.MonoCellMetrics {
	if !platform.ValidFontSize(size) {
		return gfx.MonoCellMetrics{}
	}
	shared.mu.Lock()
	defer shared.mu.Unlock()
	var out [3]C.float
	C.viva_text_metrics(C.float(size), 400, 0, 1, &out[0])
	units, length := shared.encode("0")
	advance := float32(C.viva_text_width(units, length, C.float(size), 400, 0, 1))
	if advance <= 0 || out[0]+out[1] <= 0 {
		return gfx.MonoCellMetrics{}
	}
	return gfx.MonoCellMetrics{Advance: advance, Ascent: float32(out[0]), Descent: float32(out[1]), LineGap: float32(out[2])}
}

// utf16Units is value in UTF-16.
func utf16Units(value string) []uint16 { return utf16.Encode([]rune(value)) }
