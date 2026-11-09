package windows

import (
	"runtime"
	"syscall"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/vivaui/viva/gfx"
)

// unicodeRange is DWRITE_UNICODE_RANGE.
type unicodeRange struct{ first, last uint32 }

// faceFallback is the font fallback a face draws the characters its family lacks with.
type faceFallback struct {
	face     gfx.MonoFace
	fallback uintptr // an IDWriteFontFallback; 0 when it could not be made
}

// fontFallback is the IDWriteFontFallback of a face, built the first time the face is
// drawn and kept until the rasterizer closes: the families the application gave in
// order, then the system's own fallback for what none of them has. It is 0 for a face
// without fallback families, and where IDWriteFactory2 (Windows 8.1) is missing; the
// text then takes the layout's default fallback, as it always did.
func (rasterizer *directRasterizer) fontFallback(face gfx.MonoFace) uintptr {
	names := gfx.MonoFallbacks(face)
	if len(names) == 0 {
		return 0
	}
	for _, known := range rasterizer.fallbacks {
		if known.face == face {
			return known.fallback
		}
	}
	fallback := rasterizer.buildFontFallback(names)
	rasterizer.fallbacks = append(rasterizer.fallbacks, faceFallback{face: face, fallback: fallback})
	return fallback
}

func (rasterizer *directRasterizer) buildFontFallback(names []string) uintptr {
	var factory2 uintptr
	if comCall(rasterizer.services.write, 0, uintptr(unsafe.Pointer(&writeFactory2IID)), uintptr(unsafe.Pointer(&factory2))) != 0 || factory2 == 0 {
		return 0
	}
	defer release(&factory2)
	var builder uintptr
	if hresult(comCall(factory2, writeFactoryCreateFallbackBuilder, uintptr(unsafe.Pointer(&builder)))) != nil || builder == 0 {
		return 0
	}
	defer release(&builder)
	targets := make([]*uint16, 0, len(names))
	for _, name := range names {
		pointer, err := syscall.UTF16PtrFromString(name)
		if err != nil {
			continue
		}
		targets = append(targets, pointer)
	}
	if len(targets) == 0 {
		return 0
	}
	// One mapping for the whole of Unicode: DirectWrite asks it only for the characters
	// the base font has no glyph for, and takes the first listed family that has one.
	// A family that is not installed is skipped.
	everything := unicodeRange{first: 0, last: 0x10FFFF}
	// The scale is a float, so the call is typed: the platforms pass it in a register
	// of its own kind.
	var addMapping func(this uintptr, ranges *unicodeRange, rangesCount uint32, names **uint16, namesCount uint32, collection uintptr, locale, base *uint16, scale float32) uintptr
	purego.RegisterFunc(&addMapping, vtable(builder, writeBuilderAddMapping))
	status := addMapping(builder, &everything, 1, &targets[0], uint32(len(targets)), 0, nil, nil, 1)
	runtime.KeepAlive(targets)
	if hresult(status) != nil {
		return 0
	}
	if system := rasterizer.engine.fallback; system != 0 {
		// What none of the families has is left to the system, as before.
		comCall(builder, writeBuilderAddMappings, system)
	}
	var fallback uintptr
	if hresult(comCall(builder, writeBuilderCreateFontFallback, uintptr(unsafe.Pointer(&fallback)))) != nil {
		return 0
	}
	return fallback
}

func (rasterizer *directRasterizer) releaseFontFallbacks() {
	for index := range rasterizer.fallbacks {
		release(&rasterizer.fallbacks[index].fallback)
	}
	rasterizer.fallbacks = nil
}
