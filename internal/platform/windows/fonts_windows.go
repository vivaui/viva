package windows

import (
	"github.com/vivaui/viva/internal/platform"
	"sort"
	"sync"
	"syscall"
	"unsafe"
)

// enumLogFont is the front of ENUMLOGFONTEXW: a LOGFONTW, whose face name is the
// family.
type enumLogFont struct {
	Height, Width, Escapement, Orientation, Weight int32
	Italic, Underline, StrikeOut, CharSet          byte
	OutPrecision, ClipPrecision, Quality, Pitch    byte
	FaceName                                       [32]uint16
}

// newTextMetric is the front of NEWTEXTMETRICEXW: a TEXTMETRICW up to its pitch
// byte.
type newTextMetric struct {
	Height, Ascent, Descent, InternalLeading, ExternalLeading int32
	AveCharWidth, MaxCharWidth, Weight, Overhang              int32
	DigitizedAspectX, DigitizedAspectY                        int32
	FirstChar, LastChar, DefaultChar, BreakChar               uint16
	Italic, Underlined, StruckOut, PitchAndFamily             byte
}

const (
	defaultCharset = 1
	// tmpfFixedPitch is set for fonts that are NOT fixed-pitch: the name is a
	// historical accident.
	tmpfFixedPitch = 1
	// vectorFonts excludes raster fonts, which cannot be scaled to a size.
	rasterFontType = 1
)

var (
	gdi                = systemDLL("gdi32.dll")
	enumFontFamiliesEx = gdi.NewProc("EnumFontFamiliesExW")
	getCharWidth32     = gdi.NewProc("GetCharWidth32W")
	fontEnumMu         sync.Mutex
	fontEnumFound      map[string]struct{}
	// fontEnumMaybe holds the families the fixed-pitch flag does not vouch
	// for, with one LOGFONT each to measure them by before leaving them out.
	fontEnumMaybe       map[string]enumLogFont
	fontEnumCallbackPtr uintptr
	fontEnumMono        bool
)

func fontEnumCallback(logFont *enumLogFont, metric *newTextMetric, fontType uint32, _ uintptr, mono bool) (result uintptr) {
	defer func() {
		if recovered := recover(); recovered != nil {
			platform.ReportFault("font enumeration", recovered)
			result = 1
		}
	}()
	if logFont == nil || metric == nil || fontType&rasterFontType != 0 {
		return 1
	}
	units := 0
	for units < len(logFont.FaceName) && logFont.FaceName[units] != 0 {
		units++
	}
	if units >= len(logFont.FaceName)-1 {
		// The face name field holds 31 characters, and the system cuts a longer
		// name to fit: a name that fills it may be a cut one, which names no family.
		return 1
	}
	name := syscall.UTF16ToString(logFont.FaceName[:])
	if name == "" || name[0] == '@' { // @-names are the vertical writing variants
		return 1
	}
	if mono && metric.PitchAndFamily&tmpfFixedPitch != 0 {
		// The flag really says "proportional" (a historical accident of a
		// name), and it is wrong for some families a terminal wants — Nerd
		// Fonts among them (t8y2/motty#296). Keep one LOGFONT to measure the
		// family by before leaving it out.
		if _, seen := fontEnumMaybe[name]; !seen {
			fontEnumMaybe[name] = *logFont
		}
		return 1
	}
	fontEnumFound[name] = struct{}{}
	return 1
}

// fontMeasuresUniform creates the family and measures a spread of printable
// ASCII: one advance for all of them is what drawing on a character grid needs,
// and what the system's fixed-pitch flag fails to report for some families. A
// family that will not say (the call needs a scalable font) stays out, as it
// was before.
func fontMeasuresUniform(logFont *enumLogFont) bool {
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return false
	}
	defer releaseDC.Call(0, dc)
	font, _, _ := gdiCreateFont.Call(uintptr(unsafe.Pointer(logFont)))
	if font == 0 {
		return false
	}
	defer deleteObject.Call(font)
	previous, _, _ := selectObject.Call(dc, font)
	defer selectObject.Call(dc, previous)
	// The fixed-pitch flag lies for some CJK and linked families, so measure
	// every printable ASCII advance in one range call and demand one shared
	// width — the same signal the darwin reader gets from its own measurement.
	var widths [0x7f - 0x20]int32
	if r1, _, _ := getCharWidth32.Call(dc, 0x20, 0x7e, uintptr(unsafe.Pointer(&widths[0]))); r1 == 0 {
		return false
	}
	for _, advance := range widths {
		if advance != widths[0] {
			return false
		}
	}
	return true
}

// MonospaceFamilies lists the installed fixed-pitch font families, sorted.
func MonospaceFamilies() []string { return fontFamilies(true) }

// FontFamilies lists every installed font family, sorted. A family whose name fills
// the 31 characters of a LOGFONT is left out (see fontEnumCallback).
func FontFamilies() []string { return fontFamilies(false) }

// fontFamilies enumerates the installed families, only the fixed-pitch ones when
// monospace is set. Raster fonts and the vertical-writing variants are left out
// either way.
func fontFamilies(mono bool) []string {
	fontEnumMu.Lock()
	defer fontEnumMu.Unlock()
	if fontEnumCallbackPtr == 0 {
		fontEnumCallbackPtr = syscall.NewCallback(func(logFont *enumLogFont, metric *newTextMetric, fontType uint32, extra uintptr) uintptr {
			return fontEnumCallback(logFont, metric, fontType, extra, fontEnumMono)
		})
	}
	fontEnumMono = mono
	fontEnumFound = make(map[string]struct{})
	fontEnumMaybe = make(map[string]enumLogFont)
	defer func() { fontEnumFound, fontEnumMaybe = nil, nil }()
	dc, _, _ := getDC.Call(0)
	if dc == 0 {
		return nil
	}
	defer releaseDC.Call(0, dc)
	var filter enumLogFont
	filter.CharSet = defaultCharset
	enumFontFamiliesEx.Call(dc, uintptr(unsafe.Pointer(&filter)), fontEnumCallbackPtr, 0, 0)
	names := make([]string, 0, len(fontEnumFound)+len(fontEnumMaybe))
	for name := range fontEnumFound {
		names = append(names, name)
	}
	for name, logFont := range fontEnumMaybe {
		if fontMeasuresUniform(&logFont) {
			names = append(names, name)
		}
	}
	sort.Slice(names, func(i, j int) bool { return lowerASCII(names[i]) < lowerASCII(names[j]) })
	return names
}

func lowerASCII(value string) string {
	bytes := []byte(value)
	for index, b := range bytes {
		if b >= 'A' && b <= 'Z' {
			bytes[index] = b + 'a' - 'A'
		}
	}
	return string(bytes)
}
