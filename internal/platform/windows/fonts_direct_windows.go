package windows

import (
	"math"
	"sort"
	"syscall"
	"unsafe"
)

// Method slots of IDWriteFontCollection, IDWriteFontFamily and
// IDWriteLocalizedStrings (checked against dwrite.h).
const (
	writeCollectionGetFontFamilyCount = 3
	writeFontFamilyGetFamilyNames     = 6
	writeStringsGetStringLength       = 7
	writeStringsGetString             = 8
)

// The installed families, read from DirectWrite's system font collection. This
// is the list every name in this package is resolved through: CreateTextFormat
// looks a family up in that collection, so a name that is not one of its families
// draws in the fallback font without saying so. GDI, whose enumeration the list
// used to come from, reports the typeface names of a family's styles instead
// ("Maple Mono Light", "Cascadia Mono SemiBold"): a style is what a user may
// install, but it is not a family, and picking one changed nothing.

// directFamilies lists the family names of the collection every text format is
// made from, sorted, only the fixed-pitch ones when monospace is set. It reports
// false when DirectWrite cannot answer at all, which leaves the caller its GDI
// fallback.
func directFamilies(mono bool) ([]string, bool) {
	services := &textServices{}
	if err := services.initializeWrite(); err != nil {
		return nil, false
	}
	defer services.close()
	names, ok := collectionFamilyNames(services)
	if !ok {
		return nil, false
	}
	if mono {
		// The measurement takes the same collection lock the names were read
		// under, so it runs after that lock is back: a writer waiting for it
		// would otherwise meet the read lock this goroutine already holds.
		kept := names[:0]
		for _, name := range names {
			if familyMeasuresMono(services, name) {
				kept = append(kept, name)
			}
		}
		names = kept
	}
	if len(names) == 0 {
		return nil, false
	}
	sort.Slice(names, func(i, j int) bool { return lowerASCII(names[i]) < lowerASCII(names[j]) })
	return names, true
}

// collectionFamilyNames reads every family name of the collection text formats
// are made from: the application's fonts and the system's when it registered
// one, the system's alone otherwise.
func collectionFamilyNames(services *textServices) ([]string, bool) {
	collection, unlock, ok := openCollection(services)
	if !ok {
		return nil, false
	}
	defer unlock()
	// GetFontFamilyCount returns the count itself; it takes no out parameter.
	count := uint32(comCall(collection, writeCollectionGetFontFamilyCount))
	names := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		name, ok := collectionFamilyName(collection, index)
		if !ok || name == "" {
			continue
		}
		// An @-prefixed name is the vertical-writing variant of the family before
		// it, which nothing here draws.
		if name[0] == '@' {
			continue
		}
		names = append(names, name)
	}
	return names, len(names) > 0
}

// openCollection is the font collection every text format is made from: the
// application's fonts and the system's when it registered one, the system's alone
// otherwise. unlock is called once the collection is no longer read; it stays
// locked only while a name is looked up in it, never across a layout that takes
// the same lock.
func openCollection(services *textServices) (collection uintptr, unlock func(), ok bool) {
	customFonts.RLock()
	if customFonts.collection != 0 {
		return customFonts.collection, customFonts.RUnlock, true
	}
	customFonts.RUnlock()
	system := uintptr(0)
	status := comCall(services.write, writeFactoryGetSystemFontCollection, uintptr(unsafe.Pointer(&system)), 0)
	if err := hresult(status); err != nil || system == 0 {
		return 0, func() {}, false
	}
	return system, func() { release(&system) }, true
}

// collectionFamilyName is the family's name in the collection, in the locale the
// font lists first — the name CreateTextFormat resolves.
func collectionFamilyName(collection uintptr, index uint32) (string, bool) {
	var family uintptr
	if err := hresult(comCall(collection, writeCollectionGetFontFamily, uintptr(index), uintptr(unsafe.Pointer(&family)))); err != nil || family == 0 {
		return "", false
	}
	defer release(&family)
	var strings uintptr
	if err := hresult(comCall(family, writeFontFamilyGetFamilyNames, uintptr(unsafe.Pointer(&strings)))); err != nil || strings == 0 {
		return "", false
	}
	defer release(&strings)
	var length uint32
	if err := hresult(comCall(strings, writeStringsGetStringLength, 0, uintptr(unsafe.Pointer(&length)))); err != nil || length == 0 {
		return "", false
	}
	buffer := make([]uint16, length+1)
	if err := hresult(comCall(strings, writeStringsGetString, 0, uintptr(unsafe.Pointer(&buffer[0])), uintptr(len(buffer)))); err != nil {
		return "", false
	}
	return syscall.UTF16ToString(buffer), true
}

// familyMeasuresMono reports whether family draws the printable ASCII on one advance, as
// a terminal's grid needs. Twenty i's and twenty W's come out the same width only
// when it does, which measures the family through the same layout the text is
// drawn with: the system's fixed-pitch flag is wrong for some families a terminal
// wants (Nerd Fonts among them) and right-but-slow to trust for others.
func familyMeasuresMono(services *textServices, family string) bool {
	narrow, err := services.measureIn(family, "iiiiiiiiiiiiiiiiiiii", monoMeasureSize, 0)
	if err != nil || narrow.Width <= 0 {
		return false
	}
	wide, err := services.measureIn(family, "WWWWWWWWWWWWWWWWWWWW", monoMeasureSize, 0)
	if err != nil {
		return false
	}
	return math.Abs(float64(narrow.Width-wide.Width)) < monoMeasureTolerance
}

// monoMeasureSize is the size the fixed-pitch families are measured at, and
// monoMeasureTolerance the width they must agree within, in logical pixels. Two
// advances differ by far more than this in a proportional family at any size.
const (
	monoMeasureSize      = 14
	monoMeasureTolerance = 0.01
)
