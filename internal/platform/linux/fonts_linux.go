//go:build !android

package linux

import (
	"math"
	"sort"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

const (
	// Fontconfig spacing values (fontconfig.h): FC_DUAL (90) is used by CJK
	// dual-width terminal fonts, FC_MONO (100) by single-width monospace fonts,
	// and FC_CHARCELL (110) by character-cell fonts.
	fcDual     = 90
	fcMono     = 100
	fcCharCell = 110

	monoMeasureSize      = 14
	monoMeasureTolerance = 0.01
)

// fcFontSet is FcFontSet: how many patterns, and the array of them.
type fcFontSet struct {
	count int32
	size  int32
	fonts unsafe.Pointer
}

// MonospaceFamilies lists the installed fixed-pitch font families, sorted. It
// asks fontconfig, which every desktop with fonts to draw has; without it the
// list is empty.
func MonospaceFamilies() []string { return fontFamilies(true) }

// FontFamilies lists every installed font family, sorted, asked of fontconfig.
func FontFamilies() []string { return fontFamilies(false) }

// fontFamilies asks fontconfig for the installed families, only the fixed-pitch
// ones when mono is set.
//
// Trusting fontconfig's FC_MONO (100) alone leaves out dual-width CJK terminal
// fonts (FC_DUAL, 90, such as Sarasa Term) and monospace fonts whose spacing
// metadata is proportional or unset. Measuring each family through Pango with
// fallback disabled catches every family that draws printable ASCII on one grid;
// when Pango is unavailable, it falls back to fontconfig's fixed-pitch spacings.
func fontFamilies(mono bool) []string {
	library, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	var (
		patternCreate     func() uintptr
		patternDestroy    func(uintptr)
		objectSetCreate   func() uintptr
		objectSetDestroy  func(uintptr)
		objectSetAdd      func(set uintptr, object *byte) int32
		fontList          func(config, pattern, set uintptr) *fcFontSet
		fontSetDestroy    func(*fcFontSet)
		patternGetString  func(pattern uintptr, object *byte, index int32, value **byte) int32
		patternGetInteger func(pattern uintptr, object *byte, index int32, value *int32) int32
	)
	purego.RegisterLibFunc(&patternCreate, library, "FcPatternCreate")
	purego.RegisterLibFunc(&patternDestroy, library, "FcPatternDestroy")
	purego.RegisterLibFunc(&objectSetCreate, library, "FcObjectSetCreate")
	purego.RegisterLibFunc(&objectSetDestroy, library, "FcObjectSetDestroy")
	purego.RegisterLibFunc(&objectSetAdd, library, "FcObjectSetAdd")
	purego.RegisterLibFunc(&fontList, library, "FcFontList")
	purego.RegisterLibFunc(&fontSetDestroy, library, "FcFontSetDestroy")
	purego.RegisterLibFunc(&patternGetString, library, "FcPatternGetString")
	purego.RegisterLibFunc(&patternGetInteger, library, "FcPatternGetInteger")

	spacing, family := []byte("spacing\x00"), []byte("family\x00")
	pattern := patternCreate()
	set := objectSetCreate()
	if pattern == 0 || set == 0 {
		return nil
	}
	defer patternDestroy(pattern)
	defer objectSetDestroy(set)
	if objectSetAdd(set, &family[0]) == 0 {
		return nil
	}
	if mono && objectSetAdd(set, &spacing[0]) == 0 {
		return nil
	}
	fonts := fontList(0, pattern, set)
	if fonts == nil {
		return nil
	}
	defer fontSetDestroy(fonts)

	seen := make(map[string]bool)
	for index := int32(0); index < fonts.count; index++ {
		font := *(*uintptr)(unsafe.Add(fonts.fonts, uintptr(index)*unsafe.Sizeof(uintptr(0))))
		var value *byte
		if font == 0 || patternGetString(font, &family[0], 0, &value) != 0 || value == nil {
			continue
		}
		name := cStringAt(value)
		if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
			continue
		}
		if !mono {
			seen[name] = true
			continue
		}
		var sp int32
		hasSpacing := patternGetInteger(font, &spacing[0], 0, &sp) == 0
		isFixedSpacing := hasSpacing && (sp == fcDual || sp == fcMono || sp == fcCharCell)
		if isFixedSpacing {
			seen[name] = true
		} else if _, exists := seen[name]; !exists {
			seen[name] = false
		}
	}

	var names []string
	if !mono {
		names = make([]string, 0, len(seen))
		for name := range seen {
			names = append(names, name)
		}
	} else if services, err := newTextServices(); err == nil {
		defer services.close()
		names = make([]string, 0, len(seen))
		for name := range seen {
			if familyMeasuresMono(services, name) {
				names = append(names, name)
			}
		}
	} else {
		names = make([]string, 0, len(seen))
		for name, fixed := range seen {
			if fixed {
				names = append(names, name)
			}
		}
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
}

// familyMeasuresMono reports whether family draws printable ASCII on one advance,
// as a terminal's grid needs. Twenty i's and twenty W's come out the same width
// only when it does, with fallback disabled so another font never supplies the
// glyphs for a family that lacks them.
func familyMeasuresMono(services *textServices, family string) bool {
	description, err := services.description(family, monoMeasureSize, 0, false)
	if err != nil {
		return false
	}
	defer services.fontFree(description)
	if services.noFallback != 0 {
		services.layoutSetAttributes(services.layout, services.noFallback)
		defer services.layoutSetAttributes(services.layout, 0)
	}
	narrow, _, _ := services.layoutValue(description, "iiiiiiiiiiiiiiiiiiii")
	if narrow.Width <= 0 {
		return false
	}
	if services.noFallback != 0 && services.layoutGetUnknownGlyphs != nil && services.layoutGetUnknownGlyphs(services.layout) > 0 {
		return false
	}
	wide, _, _ := services.layoutValue(description, "WWWWWWWWWWWWWWWWWWWW")
	if wide.Width <= 0 {
		return false
	}
	if services.noFallback != 0 && services.layoutGetUnknownGlyphs != nil && services.layoutGetUnknownGlyphs(services.layout) > 0 {
		return false
	}
	narrowPx := float64(narrow.Width) / pangoScale
	widePx := float64(wide.Width) / pangoScale
	return math.Abs(narrowPx-widePx) < monoMeasureTolerance
}

// cStringAt reads the NUL-terminated string a C library returned.
func cStringAt(start *byte) string {
	const limit = 256
	var bytes []byte
	for offset := uintptr(0); offset < limit; offset++ {
		b := *(*byte)(unsafe.Add(unsafe.Pointer(start), offset))
		if b == 0 {
			break
		}
		bytes = append(bytes, b)
	}
	return string(bytes)
}
