package linux

import (
	"sort"
	"strings"
	"unsafe"

	"github.com/ebitengine/purego"
)

// fcMono is fontconfig's FC_MONO spacing: every glyph the same width.
const fcMono = 100

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
// ones (fontconfig's FC_MONO spacing) when mono is set.
func fontFamilies(mono bool) []string {
	library, err := purego.Dlopen("libfontconfig.so.1", purego.RTLD_NOW|purego.RTLD_LOCAL)
	if err != nil {
		return nil
	}
	var (
		patternCreate     func() uintptr
		patternDestroy    func(uintptr)
		patternAddInteger func(pattern uintptr, object *byte, value int32) int32
		objectSetCreate   func() uintptr
		objectSetDestroy  func(uintptr)
		objectSetAdd      func(set uintptr, object *byte) int32
		fontList          func(config, pattern, set uintptr) *fcFontSet
		fontSetDestroy    func(*fcFontSet)
		patternGetString  func(pattern uintptr, object *byte, index int32, value **byte) int32
	)
	purego.RegisterLibFunc(&patternCreate, library, "FcPatternCreate")
	purego.RegisterLibFunc(&patternDestroy, library, "FcPatternDestroy")
	purego.RegisterLibFunc(&patternAddInteger, library, "FcPatternAddInteger")
	purego.RegisterLibFunc(&objectSetCreate, library, "FcObjectSetCreate")
	purego.RegisterLibFunc(&objectSetDestroy, library, "FcObjectSetDestroy")
	purego.RegisterLibFunc(&objectSetAdd, library, "FcObjectSetAdd")
	purego.RegisterLibFunc(&fontList, library, "FcFontList")
	purego.RegisterLibFunc(&fontSetDestroy, library, "FcFontSetDestroy")
	purego.RegisterLibFunc(&patternGetString, library, "FcPatternGetString")

	spacing, family := []byte("spacing\x00"), []byte("family\x00")
	pattern := patternCreate()
	set := objectSetCreate()
	if pattern == 0 || set == 0 {
		return nil
	}
	defer patternDestroy(pattern)
	defer objectSetDestroy(set)
	if mono && (patternAddInteger(pattern, &spacing[0], fcMono) == 0 || objectSetAdd(set, &family[0]) == 0) {
		return nil
	}
	if !mono && objectSetAdd(set, &family[0]) == 0 {
		return nil
	}
	fonts := fontList(0, pattern, set)
	if fonts == nil {
		return nil
	}
	defer fontSetDestroy(fonts)

	seen := make(map[string]struct{})
	for index := int32(0); index < fonts.count; index++ {
		font := *(*uintptr)(unsafe.Add(fonts.fonts, uintptr(index)*unsafe.Sizeof(uintptr(0))))
		var value *byte
		if font == 0 || patternGetString(font, &family[0], 0, &value) != 0 || value == nil {
			continue
		}
		if name := cStringAt(value); name != "" {
			seen[name] = struct{}{}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	return names
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
